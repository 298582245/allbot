package web

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/allbot/allbot/core/adapter/_contract"
	"github.com/allbot/allbot/core/config"
	"github.com/allbot/allbot/core/types"
)

const platformName = "web"

const DefaultSMTPSubject = "AllBot Web 聊天室验证码"

type Adapter struct {
	database       *config.Database
	messageHandler func(*types.Message)
	mu             sync.RWMutex
	subscribers    map[string]map[chan *config.WebChatMessage]struct{}
	started        bool
	config         Config
}

type Config struct {
	SMTPHost     string `json:"smtp_host"`
	SMTPPort     string `json:"smtp_port"`
	SMTPUsername string `json:"smtp_username"`
	SMTPPassword string `json:"smtp_password"`
	SMTPFrom     string `json:"smtp_from"`
	SMTPSubject  string `json:"smtp_subject"`
}

func NewAdapter() *Adapter {
	return NewAdapterWithConfig(nil)
}

func NewAdapterWithConfig(cfg *Config) *Adapter {
	adapter := &Adapter{subscribers: make(map[string]map[chan *config.WebChatMessage]struct{})}
	if cfg != nil {
		adapter.config = normalizeConfig(*cfg)
	}
	return adapter
}

func (a *Adapter) SMTPConfig() Config {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.config
}

func (a *Adapter) SetDatabase(database *config.Database) {
	a.mu.Lock()
	a.database = database
	a.mu.Unlock()
}

func (a *Adapter) GetPlatform() string { return platformName }

func (a *Adapter) SetMessageHandler(handler func(*types.Message)) {
	a.mu.Lock()
	a.messageHandler = handler
	a.mu.Unlock()
}

func (a *Adapter) Start() error {
	a.mu.Lock()
	a.started = true
	a.mu.Unlock()
	return nil
}

func (a *Adapter) Stop() error {
	a.mu.Lock()
	for _, channels := range a.subscribers {
		for ch := range channels {
			close(ch)
		}
	}
	a.subscribers = make(map[string]map[chan *config.WebChatMessage]struct{})
	a.started = false
	a.mu.Unlock()
	return nil
}

func (a *Adapter) SendMessage(target string, text string) error {
	return a.saveOutbound(target, &config.WebChatMessage{MessageType: "text", Content: text, Target: target, PluginID: webChatPluginIDFromTarget(target)})
}

func (a *Adapter) SendMarkdown(target string, markdown string) error {
	return a.saveOutbound(target, &config.WebChatMessage{MessageType: "markdown", Content: markdown, Target: target, PluginID: webChatPluginIDFromTarget(target)})
}

func (a *Adapter) SendImage(target string, imageURL string) error {
	return a.saveOutbound(target, &config.WebChatMessage{MessageType: "image", ImageURL: imageURL, Target: target, PluginID: webChatPluginIDFromTarget(target)})
}

func (a *Adapter) SendFile(target string, filePath string) error {
	return a.SendMessage(target, "文件："+filePath)
}

func (a *Adapter) SendRichMessage(target string, message types.RichMessage) error {
	data, err := json.Marshal(message)
	if err != nil {
		return err
	}
	content := strings.TrimSpace(message.FallbackText)
	if content == "" {
		content = richFallback(message)
	}
	return a.saveOutbound(target, &config.WebChatMessage{MessageType: "rich", Content: content, RichJSON: string(data), Target: target, PluginID: webChatPluginIDFromTarget(target)})
}

func (a *Adapter) SendButtons(target string, text string, buttons [][]types.ButtonOption) error {
	data, err := json.Marshal(buttons)
	if err != nil {
		return err
	}
	return a.saveOutbound(target, &config.WebChatMessage{MessageType: "buttons", Content: text, RichJSON: string(data), Target: target, PluginID: webChatPluginIDFromTarget(target)})
}

func (a *Adapter) SendWebChatMessage(target string, message *config.WebChatMessage) error {
	if message == nil {
		return fmt.Errorf("Web 聊天消息不能为空")
	}
	message.Target = target
	return a.saveOutbound(target, message)
}

func (a *Adapter) GetUserInfo(userID string) (*contract.UserInfo, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, fmt.Errorf("用户 ID 不能为空")
	}
	database := a.currentDatabase()
	if database != nil {
		if user, err := database.GetWebChatUser(userID); err == nil {
			return &contract.UserInfo{UserID: user.UserID, Nickname: user.DisplayName, Extra: map[string]string{"username": user.Username, "email": user.Email}}, nil
		}
	}
	return &contract.UserInfo{UserID: userID, Nickname: userID}, nil
}

func (a *Adapter) GetGroupInfo(groupID string) (*contract.GroupInfo, error) {
	return &contract.GroupInfo{GroupID: strings.TrimSpace(groupID), Name: "Web 聊天室"}, nil
}

func (a *Adapter) AtUser(groupID string, userID string) error { return nil }

func (a *Adapter) ReplyTarget(msg *types.Message) string {
	if msg == nil {
		return ""
	}
	target := a.SendTarget(msg.UserID, msg.GroupID)
	if msg.Metadata != nil {
		if pluginID := strings.TrimSpace(msg.Metadata["web_chat_plugin_id"]); pluginID != "" {
			target += "#plugin_" + pluginID
		}
	}
	return target
}

func (a *Adapter) SendTarget(userID string, groupID string) string {
	return "user_" + strings.TrimSpace(userID)
}

func (a *Adapter) ReceiveMessage(userID, content, messageType, imageURL, richJSON string) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return
	}
	a.mu.RLock()
	handler := a.messageHandler
	a.mu.RUnlock()
	if handler == nil {
		return
	}
	metadata := map[string]string{"message_type": strings.TrimSpace(messageType)}
	if imageURL != "" {
		metadata["image_url"] = imageURL
	}
	if richJSON != "" {
		metadata["rich_json"] = richJSON
	}
	log.Printf("[接收][web][%s(私聊)]：%s", userID, content)
	handler(&types.Message{ID: fmt.Sprintf("web_%d", time.Now().UnixNano()), Platform: platformName, UserID: userID, Content: content, Metadata: metadata})
}

func (a *Adapter) Subscribe(userID string, buffer int) (<-chan *config.WebChatMessage, func()) {
	userID = strings.TrimSpace(userID)
	if buffer <= 0 {
		buffer = 16
	}
	ch := make(chan *config.WebChatMessage, buffer)
	a.mu.Lock()
	if a.subscribers[userID] == nil {
		a.subscribers[userID] = make(map[chan *config.WebChatMessage]struct{})
	}
	a.subscribers[userID][ch] = struct{}{}
	a.mu.Unlock()
	cancel := func() {
		a.mu.Lock()
		if channels := a.subscribers[userID]; channels != nil {
			if _, ok := channels[ch]; ok {
				delete(channels, ch)
				close(ch)
			}
			if len(channels) == 0 {
				delete(a.subscribers, userID)
			}
		}
		a.mu.Unlock()
	}
	return ch, cancel
}

func (a *Adapter) saveOutbound(target string, message *config.WebChatMessage) error {
	userID := userIDFromTarget(target)
	if userID == "" {
		return fmt.Errorf("web 发送目标无效: %s", target)
	}
	message.UserID = userID
	message.Direction = "out"
	log.Printf("[发送][web][%s]：%s", target, webChatLogContent(message))
	database := a.currentDatabase()
	if database != nil {
		saved, err := database.SaveWebChatMessage(message)
		if err != nil {
			return err
		}
		message = saved
	}
	a.broadcast(userID, message)
	return nil
}

func (a *Adapter) broadcast(userID string, message *config.WebChatMessage) {
	a.mu.RLock()
	channels := a.subscribers[userID]
	for ch := range channels {
		select {
		case ch <- message:
		default:
		}
	}
	a.mu.RUnlock()
}

func (a *Adapter) currentDatabase() *config.Database {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.database
}

func userIDFromTarget(target string) string {
	target = strings.TrimSpace(target)
	if index := strings.Index(target, "#plugin_"); index >= 0 {
		target = target[:index]
	}
	if strings.HasPrefix(target, "user_") {
		return strings.TrimSpace(strings.TrimPrefix(target, "user_"))
	}
	return target
}

func webChatPluginIDFromTarget(target string) string {
	target = strings.TrimSpace(target)
	index := strings.Index(target, "#plugin_")
	if index < 0 {
		return ""
	}
	return strings.TrimSpace(target[index+len("#plugin_"):])
}

func webChatLogContent(message *config.WebChatMessage) string {
	if message == nil {
		return ""
	}
	content := strings.TrimSpace(message.Content)
	switch strings.TrimSpace(message.MessageType) {
	case "markdown":
		return "[Markdown] " + content
	case "image":
		return "[图片] " + strings.TrimSpace(message.ImageURL)
	case "buttons":
		return "[Buttons] " + content
	case "rich":
		return "[Rich] " + content
	default:
		return content
	}
}

func normalizeConfig(cfg Config) Config {
	cfg.SMTPHost = strings.TrimSpace(cfg.SMTPHost)
	cfg.SMTPPort = strings.TrimSpace(cfg.SMTPPort)
	cfg.SMTPUsername = strings.TrimSpace(cfg.SMTPUsername)
	cfg.SMTPPassword = strings.TrimSpace(cfg.SMTPPassword)
	cfg.SMTPFrom = strings.TrimSpace(cfg.SMTPFrom)
	cfg.SMTPSubject = strings.TrimSpace(cfg.SMTPSubject)
	if cfg.SMTPSubject == "" {
		cfg.SMTPSubject = DefaultSMTPSubject
	}
	return cfg
}

func richFallback(message types.RichMessage) string {
	parts := make([]string, 0, len(message.Parts))
	for _, part := range message.Parts {
		switch part.Type {
		case "markdown":
			parts = append(parts, strings.TrimSpace(part.Markdown))
		case "image":
			parts = append(parts, strings.TrimSpace(part.URL))
		default:
			parts = append(parts, strings.TrimSpace(part.Text))
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}
