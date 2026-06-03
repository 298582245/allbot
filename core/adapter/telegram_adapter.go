package adapter

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/allbot/allbot/core/types"
	"github.com/allbot/allbot/core/utils"
)

// TelegramAdapter Telegram 平台适配器
type TelegramAdapter struct {
	botToken       string
	apiURL         string
	messageHandler func(*types.Message)
	stopChan       chan struct{}
	lastUpdateID   int64
	httpClient     *http.Client
}

// NewTelegramAdapter 创建 Telegram 适配器
func NewTelegramAdapter(botToken string, proxyURL string) *TelegramAdapter {
	// 创建 HTTP 客户端
	client := &http.Client{
		Timeout: 40 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: false},
		},
	}

	// 如果配置了代理，使用代理
	if proxyURL != "" {
		if proxy, err := url.Parse(proxyURL); err == nil {
			client.Transport = &http.Transport{
				Proxy:           http.ProxyURL(proxy),
				TLSClientConfig: &tls.Config{InsecureSkipVerify: false},
			}
			log.Printf("Telegram 使用代理: %s", proxyURL)
		} else {
			log.Printf("警告：代理地址解析失败: %v", err)
		}
	}

	return &TelegramAdapter{
		botToken:   botToken,
		apiURL:     "https://api.telegram.org/bot" + botToken,
		stopChan:   make(chan struct{}),
		httpClient: client,
	}
}

// GetPlatform 获取平台名称
func (a *TelegramAdapter) GetPlatform() string {
	return "telegram"
}

// SetMessageHandler 设置消息处理器
func (a *TelegramAdapter) SetMessageHandler(handler func(*types.Message)) {
	a.messageHandler = handler
}

// Start 启动适配器
func (a *TelegramAdapter) Start() error {
	// 验证 Bot Token
	if err := a.verifyToken(); err != nil {
		return fmt.Errorf("验证 Bot Token 失败: %w", err)
	}

	// 删除 webhook（如果存在），以便使用 long polling
	if err := a.deleteWebhook(); err != nil {
		log.Printf("警告：删除 webhook 失败: %v", err)
	}

	// 启动长轮询
	go a.pollUpdates()

	log.Printf("Telegram Adapter 已启动")
	return nil
}

// Stop 停止适配器
func (a *TelegramAdapter) Stop() error {
	close(a.stopChan)
	log.Printf("Telegram Adapter 已停止")
	return nil
}

// verifyToken 验证 Bot Token
func (a *TelegramAdapter) verifyToken() error {
	resp, err := a.httpClient.Get(a.apiURL + "/getMe")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("无效的 Bot Token")
	}

	return nil
}

// deleteWebhook 删除 webhook 配置
func (a *TelegramAdapter) deleteWebhook() error {
	resp, err := a.httpClient.Get(a.apiURL + "/deleteWebhook")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("删除 webhook 失败 (状态码 %d): %s", resp.StatusCode, string(body))
	}

	log.Printf("Telegram webhook 已删除，切换到 long polling 模式")
	return nil
}

// pollUpdates 长轮询获取更新
func (a *TelegramAdapter) pollUpdates() {
	failureCount := 0
	for {
		select {
		case <-a.stopChan:
			return
		default:
			updates, err := a.getUpdates()
			if err != nil {
				failureCount++
				maskedError := utils.MaskSensitiveError(err)
				if failureCount == 1 {
					log.Printf("[WARN][Telegram] 获取更新异常: %s", maskedError)
				} else if failureCount%10 == 0 {
					log.Printf("[WARN][Telegram] 获取更新持续异常 %d 次，最近一次: %s", failureCount, maskedError)
				}
				if !a.waitBeforeRetry(telegramPollRetryDelay(failureCount)) {
					return
				}
				continue
			}

			if failureCount > 0 {
				log.Printf("[INFO][Telegram] 网络轮询已恢复，之前连续异常 %d 次", failureCount)
				failureCount = 0
			}

			for _, update := range updates {
				a.handleUpdate(update)
			}
		}
	}
}

func (a *TelegramAdapter) waitBeforeRetry(delay time.Duration) bool {
	select {
	case <-a.stopChan:
		return false
	case <-time.After(delay):
		return true
	}
}

func telegramPollRetryDelay(failureCount int) time.Duration {
	if failureCount <= 1 {
		return 3 * time.Second
	}
	if failureCount <= 3 {
		return 5 * time.Second
	}
	if failureCount <= 6 {
		return 10 * time.Second
	}
	if failureCount <= 10 {
		return 30 * time.Second
	}
	return 60 * time.Second
}

// getUpdates 获取更新
func (a *TelegramAdapter) getUpdates() ([]map[string]interface{}, error) {
	url := fmt.Sprintf("%s/getUpdates?offset=%d&timeout=30", a.apiURL, a.lastUpdateID+1)

	resp, err := a.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("HTTP请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	// 检查HTTP状态码
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP状态码 %d, 响应: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Ok          bool                     `json:"ok"`
		Result      []map[string]interface{} `json:"result"`
		ErrorCode   int                      `json:"error_code"`
		Description string                   `json:"description"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("JSON解析失败: %w, 响应: %s", err, string(body))
	}

	if !result.Ok {
		return nil, fmt.Errorf("Telegram API错误 [%d]: %s", result.ErrorCode, result.Description)
	}

	return result.Result, nil
}

// handleUpdate 处理更新
func (a *TelegramAdapter) handleUpdate(update map[string]interface{}) {
	// 更新 lastUpdateID
	if updateID, ok := update["update_id"].(float64); ok {
		if int64(updateID) > a.lastUpdateID {
			a.lastUpdateID = int64(updateID)
		}
	}

	// 处理消息
	message, ok := update["message"].(map[string]interface{})
	if !ok {
		return
	}

	text, ok := message["text"].(string)
	if !ok {
		return
	}
	text = normalizeTelegramCommandText(text, message)

	from, ok := message["from"].(map[string]interface{})
	if !ok {
		return
	}

	chat, ok := message["chat"].(map[string]interface{})
	if !ok {
		return
	}

	// 正确提取chat_id，避免科学计数法
	userIDNum, _ := from["id"].(float64)
	chatIDNum, _ := chat["id"].(float64)
	messageIDNum, _ := message["message_id"].(float64)

	userID := fmt.Sprintf("%.0f", userIDNum)
	chatID := fmt.Sprintf("%.0f", chatIDNum)
	messageID := fmt.Sprintf("%.0f", messageIDNum)
	fromName := telegramDisplayName(from)

	// 判断是群组还是私聊
	chatType, _ := chat["type"].(string)
	chatInfo := "私聊"
	if chatType != "private" {
		chatInfo = fmt.Sprintf("群组%s", chatID)
	}

	log.Printf("[接收][Telegram][%s(%s)]：%s", userID, chatInfo, text)

	msg := &types.Message{
		ID:       messageID,
		Platform: "telegram",
		UserID:   userID,
		Content:  text,
		Metadata: map[string]string{
			"chat_id":   chatID, // 保存chat_id用于回复
			"from_name": fromName,
		},
	}

	// 判断是群组还是私聊
	if chatType == "group" || chatType == "supergroup" {
		msg.GroupID = chatID
	}

	if a.messageHandler != nil {
		a.messageHandler(msg)
	}
}

// SendMessage 发送消息
func normalizeTelegramCommandText(text string, message map[string]interface{}) string {
	entities, ok := message["entities"].([]interface{})
	if !ok {
		return text
	}

	for _, item := range entities {
		entity, ok := item.(map[string]interface{})
		if !ok || entity["type"] != "bot_command" {
			continue
		}

		offset, ok := entity["offset"].(float64)
		if !ok || int(offset) != 0 {
			continue
		}

		length, ok := entity["length"].(float64)
		if !ok {
			continue
		}

		commandEnd := int(length)
		if commandEnd > len(text) {
			continue
		}

		command := strings.TrimPrefix(text[:commandEnd], "/")
		if atIndex := strings.Index(command, "@"); atIndex >= 0 {
			command = command[:atIndex]
		}

		return strings.TrimSpace(command + text[commandEnd:])
	}

	return text
}

func telegramDisplayName(from map[string]interface{}) string {
	if username, ok := from["username"].(string); ok && strings.TrimSpace(username) != "" {
		return "@" + strings.TrimSpace(username)
	}
	parts := make([]string, 0, 2)
	if firstName, ok := from["first_name"].(string); ok && strings.TrimSpace(firstName) != "" {
		parts = append(parts, strings.TrimSpace(firstName))
	}
	if lastName, ok := from["last_name"].(string); ok && strings.TrimSpace(lastName) != "" {
		parts = append(parts, strings.TrimSpace(lastName))
	}
	return strings.Join(parts, " ")
}

func (a *TelegramAdapter) SendMessage(target string, text string) error {
	// Telegram API要求chat_id是数字类型，需要转换
	var chatID interface{}

	// 尝试将字符串转换为int64
	if id, err := strconv.ParseInt(target, 10, 64); err == nil {
		chatID = id
	} else {
		// 如果转换失败，保持字符串（用于username）
		chatID = target
	}

	data := map[string]interface{}{
		"chat_id": chatID,
		"text":    text,
	}
	if strings.Contains(text, "tg://user?id=") {
		data["parse_mode"] = "HTML"
	}

	// 发送前记录日志
	log.Printf("[发送][Telegram][%s]：%s", target, text)

	return a.callAPI("/sendMessage", data)
}

// SendImage 发送图片
func (a *TelegramAdapter) SendImage(target string, imageURL string) error {
	data := map[string]interface{}{
		"chat_id": target,
		"photo":   imageURL,
	}

	return a.callAPI("/sendPhoto", data)
}

// SendFile 发送文件
func (a *TelegramAdapter) SendFile(target string, filePath string) error {
	data := map[string]interface{}{
		"chat_id":  target,
		"document": filePath,
	}

	return a.callAPI("/sendDocument", data)
}

// GetUserInfo 获取用户信息
func (a *TelegramAdapter) GetUserInfo(userID string) (*UserInfo, error) {
	// Telegram 不提供直接获取用户信息的 API
	// 只能从消息中获取
	return &UserInfo{
		UserID: userID,
		Extra:  make(map[string]string),
	}, nil
}

// GetGroupInfo 获取群组信息
func (a *TelegramAdapter) GetGroupInfo(groupID string) (*GroupInfo, error) {
	data := map[string]interface{}{
		"chat_id": groupID,
	}

	var result map[string]interface{}
	if err := a.callAPIWithResult("/getChat", data, &result); err != nil {
		return nil, err
	}

	chatData, ok := result["result"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("无效的响应格式")
	}

	memberCount := 0
	if count, ok := chatData["member_count"].(float64); ok {
		memberCount = int(count)
	}

	return &GroupInfo{
		GroupID:     groupID,
		Name:        fmt.Sprintf("%v", chatData["title"]),
		MemberCount: memberCount,
		Extra:       make(map[string]string),
	}, nil
}

// AtUser @某人
func (a *TelegramAdapter) AtUser(groupID string, userID string) error {
	// Telegram 使用 mention 格式
	text := fmt.Sprintf("[User](tg://user?id=%s)", userID)
	return a.SendMessage(groupID, text)
}

// callAPI 调用 Telegram API
func (a *TelegramAdapter) callAPI(endpoint string, data map[string]interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	resp, err := a.httpClient.Post(a.apiURL+endpoint, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API 调用失败 (状态码 %d): %s", resp.StatusCode, string(body))
	}

	return nil
}

// callAPIWithResult 调用 API 并返回结果
func (a *TelegramAdapter) callAPIWithResult(endpoint string, data map[string]interface{}, result interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	resp, err := a.httpClient.Post(a.apiURL+endpoint, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	return json.Unmarshal(body, result)
}
