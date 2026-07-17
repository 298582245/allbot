package feishu

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/allbot/allbot/core/adapter/_contract"
	"github.com/allbot/allbot/core/types"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	larkws "github.com/larksuite/oapi-sdk-go/v3/ws"
)

type UserInfo = contract.UserInfo
type GroupInfo = contract.GroupInfo

const (
	platformName              = "feishu"
	feishuDefaultCallbackPath = "callback"
	feishuDefaultAPIBaseURL   = "https://open.feishu.cn/open-apis"
	feishuDefaultTokenURL     = "https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal"
	feishuTokenRefreshLead    = 5 * time.Minute
)

type FeishuAdapter struct {
	appID             string
	appSecret         string
	verificationToken string
	encryptKey        string
	callbackPath      string
	apiBaseURL        string
	tokenURL          string
	httpClient        *http.Client

	messageHandler func(*types.Message)

	tenantAccessToken string
	tokenExpiresAt    time.Time
	tokenMu           sync.Mutex

	wsMu                    sync.Mutex
	wsClient                *larkws.Client
	wsCancel                context.CancelFunc
	startLongConnectionFunc func() error

	stopOnce sync.Once
	stopped  chan struct{}
}

type feishuCallbackPayload struct {
	Schema    string             `json:"schema"`
	Header    feishuEventHeader  `json:"header"`
	Event     feishuMessageEvent `json:"event"`
	Type      string             `json:"type"`
	Token     string             `json:"token"`
	Challenge string             `json:"challenge"`
	Encrypt   string             `json:"encrypt"`
	RawEvent  json.RawMessage    `json:"-"`
}

type feishuEventHeader struct {
	EventID    string `json:"event_id"`
	EventType  string `json:"event_type"`
	TenantKey  string `json:"tenant_key"`
	Token      string `json:"token"`
	CreateTime string `json:"create_time"`
}

type feishuMessageEvent struct {
	Sender  feishuMessageSender `json:"sender"`
	Message feishuEventMessage  `json:"message"`
}

type feishuMessageSender struct {
	SenderID  feishuSenderID `json:"sender_id"`
	TenantKey string         `json:"tenant_key"`
}

type feishuSenderID struct {
	OpenID  string `json:"open_id"`
	UserID  string `json:"user_id"`
	UnionID string `json:"union_id"`
}

type feishuEventMessage struct {
	MessageID   string `json:"message_id"`
	RootID      string `json:"root_id"`
	ParentID    string `json:"parent_id"`
	CreateTime  string `json:"create_time"`
	ChatID      string `json:"chat_id"`
	ChatType    string `json:"chat_type"`
	MessageType string `json:"message_type"`
	Content     string `json:"content"`
}

type feishuTenantAccessTokenResponse struct {
	Code              int    `json:"code"`
	Msg               string `json:"msg"`
	TenantAccessToken string `json:"tenant_access_token"`
	Expire            int64  `json:"expire"`
}

type feishuAPIResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

type feishuImageUploadResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		ImageKey string `json:"image_key"`
	} `json:"data"`
}

type feishuMessageTarget struct {
	kind string
	id   string
}

// NewFeishuAdapter 创建飞书机器人适配器。
func NewFeishuAdapter(appID, appSecret, verificationToken, encryptKey, callbackPath, apiBaseURL, tokenURL string) *FeishuAdapter {
	callbackPath = normalizeCallbackPath(callbackPath)
	apiBaseURL = strings.TrimSpace(apiBaseURL)
	if apiBaseURL == "" {
		apiBaseURL = feishuDefaultAPIBaseURL
	}
	tokenURL = strings.TrimSpace(tokenURL)
	if tokenURL == "" {
		tokenURL = strings.TrimRight(apiBaseURL, "/") + "/auth/v3/tenant_access_token/internal"
	}
	adapter := &FeishuAdapter{
		appID:             strings.TrimSpace(appID),
		appSecret:         strings.TrimSpace(appSecret),
		verificationToken: strings.TrimSpace(verificationToken),
		encryptKey:        strings.TrimSpace(encryptKey),
		callbackPath:      callbackPath,
		apiBaseURL:        strings.TrimRight(apiBaseURL, "/"),
		tokenURL:          tokenURL,
		httpClient:        &http.Client{Timeout: 15 * time.Second},
		stopped:           make(chan struct{}),
	}
	adapter.startLongConnectionFunc = adapter.startLongConnection
	return adapter
}

func (a *FeishuAdapter) GetPlatform() string {
	return platformName
}

func (a *FeishuAdapter) GetBotIdentity(msg *types.Message) contract.BotIdentity {
	return contract.BotIdentity{Label: "机器人 App ID", Value: strings.TrimSpace(a.appID)}
}

func (a *FeishuAdapter) SetMessageHandler(handler func(*types.Message)) {
	a.messageHandler = handler
}

func (a *FeishuAdapter) Start() error {
	if a.appID == "" {
		return fmt.Errorf("飞书 App ID 不能为空")
	}
	if a.appSecret == "" {
		return fmt.Errorf("飞书 App Secret 不能为空")
	}
	if a.startLongConnectionFunc == nil {
		a.startLongConnectionFunc = a.startLongConnection
	}
	if err := a.startLongConnectionFunc(); err != nil {
		return err
	}
	log.Printf("飞书机器人 Adapter 已启动，长连接事件订阅已开启，HTTP 回调路径: %s", a.callbackPath)
	return nil
}

func (a *FeishuAdapter) Stop() error {
	a.stopOnce.Do(func() {
		close(a.stopped)
		a.stopLongConnection()
	})
	log.Printf("飞书机器人 Adapter 已停止")
	return nil
}

func (a *FeishuAdapter) startLongConnection() error {
	eventHandler := dispatcher.NewEventDispatcher("", "").
		OnP2MessageReceiveV1(func(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
			a.handleP2MessageReceiveV1(event)
			return nil
		})
	ctx, cancel := context.WithCancel(context.Background())
	client := larkws.NewClient(
		a.appID,
		a.appSecret,
		larkws.WithEventHandler(eventHandler),
		larkws.WithOnReady(func() { log.Printf("飞书长连接已就绪") }),
		larkws.WithOnError(func(err error) { log.Printf("[WARN][飞书] 长连接异常: %v", err) }),
		larkws.WithOnReconnecting(func() { log.Printf("[INFO][飞书] 长连接正在重连") }),
		larkws.WithOnReconnected(func() { log.Printf("[INFO][飞书] 长连接已重连") }),
		larkws.WithOnDisconnected(func() { log.Printf("[INFO][飞书] 长连接已断开") }),
	)

	a.wsMu.Lock()
	if a.wsCancel != nil {
		a.wsCancel()
	}
	if a.wsClient != nil {
		a.wsClient.Close()
	}
	a.wsClient = client
	a.wsCancel = cancel
	a.wsMu.Unlock()

	go func() {
		if err := client.Start(ctx); err != nil && ctx.Err() == nil {
			log.Printf("[WARN][飞书] 长连接启动失败: %v", err)
		}
	}()
	return nil
}

func (a *FeishuAdapter) stopLongConnection() {
	a.wsMu.Lock()
	defer a.wsMu.Unlock()
	if a.wsCancel != nil {
		a.wsCancel()
		a.wsCancel = nil
	}
	if a.wsClient != nil {
		a.wsClient.Close()
		a.wsClient = nil
	}
}

func (a *FeishuAdapter) ReplyTarget(msg *types.Message) string {
	if msg == nil {
		return ""
	}
	if msg.Metadata != nil {
		if target := strings.TrimSpace(msg.Metadata["reply_target"]); target != "" {
			return target
		}
		if messageID := strings.TrimSpace(msg.Metadata["feishu_message_id"]); messageID != "" {
			return "reply_" + messageID
		}
	}
	if groupID := strings.TrimSpace(msg.GroupID); groupID != "" {
		if hasFeishuTargetPrefix(groupID) {
			return groupID
		}
		return "chat_" + groupID
	}
	userID := strings.TrimSpace(msg.UserID)
	if hasFeishuTargetPrefix(userID) {
		return userID
	}
	if userID == "" {
		return ""
	}
	return "user_" + userID
}

func (a *FeishuAdapter) SendTarget(userID string, groupID string) string {
	groupID = strings.TrimSpace(groupID)
	if groupID != "" {
		if hasFeishuTargetPrefix(groupID) {
			return groupID
		}
		return "chat_" + groupID
	}
	userID = strings.TrimSpace(userID)
	if hasFeishuTargetPrefix(userID) {
		return userID
	}
	if userID == "" {
		return ""
	}
	return "user_" + userID
}

func (a *FeishuAdapter) FormatReplyText(msg *types.Message, text string) string {
	return text
}

func (a *FeishuAdapter) SendMessage(target string, text string) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return fmt.Errorf("飞书消息内容不能为空")
	}
	content, err := json.Marshal(map[string]string{"text": text})
	if err != nil {
		return err
	}
	log.Printf("[发送][飞书][%s]：%s", target, text)
	return a.sendFeishuMessage(target, "text", string(content))
}

func (a *FeishuAdapter) SendMarkdown(target string, markdown string) error {
	markdown = strings.TrimSpace(markdown)
	if markdown == "" {
		return fmt.Errorf("飞书 Markdown 内容不能为空")
	}
	content, err := json.Marshal(map[string]interface{}{
		"config": map[string]interface{}{
			"wide_screen_mode": true,
		},
		"elements": []map[string]interface{}{
			{
				"tag":     "markdown",
				"content": markdown,
			},
		},
	})
	if err != nil {
		return err
	}
	log.Printf("[发送][飞书][%s]：[Markdown] %s", target, markdown)
	return a.sendFeishuMessage(target, "interactive", string(content))
}

func (a *FeishuAdapter) SendImage(target string, imageURL string) error {
	if _, err := parseFeishuMessageTarget(target); err != nil {
		return err
	}
	imageURL = strings.TrimSpace(imageURL)
	if imageURL == "" {
		return fmt.Errorf("飞书图片地址不能为空")
	}
	log.Printf("[发送][飞书][%s]：[图片] %s", target, imageURL)
	imageKey, err := a.uploadImage(imageURL)
	if err != nil {
		return err
	}
	content, err := json.Marshal(map[string]string{"image_key": imageKey})
	if err != nil {
		return err
	}
	return a.sendFeishuMessage(target, "image", string(content))
}

func (a *FeishuAdapter) SendFile(target string, filePath string) error {
	return fmt.Errorf("飞书文件发送暂未实现")
}

func (a *FeishuAdapter) GetUserInfo(userID string) (*UserInfo, error) {
	return &UserInfo{UserID: userID, Nickname: userID, Extra: map[string]string{"platform": platformName}}, nil
}

func (a *FeishuAdapter) GetGroupInfo(groupID string) (*GroupInfo, error) {
	return &GroupInfo{GroupID: groupID, Name: groupID, Extra: map[string]string{"platform": platformName}}, nil
}

func (a *FeishuAdapter) AtUser(groupID string, userID string) error {
	return fmt.Errorf("飞书 @ 用户暂未实现")
}

// HandleHTTPCallback 处理飞书事件订阅 URL verification 和消息事件。
func (a *FeishuAdapter) HandleHTTPCallback(relativePath string, w http.ResponseWriter, r *http.Request) {
	if normalizeCallbackPath(relativePath) != a.callbackPath {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	callback, err := a.parseCallbackPayload(payload)
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(callback.Encrypt) != "" {
		http.Error(w, "飞书加密回调暂未支持，请先关闭事件订阅加密", http.StatusBadRequest)
		return
	}
	if !a.verifyCallbackToken(callback) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	if callback.Type == "url_verification" {
		writeFeishuJSON(w, map[string]string{"challenge": callback.Challenge})
		return
	}
	if callback.Header.EventType == "im.message.receive_v1" {
		if msg := a.buildMessage(callback); msg != nil {
			go a.dispatchMessage(msg)
		}
	}
	writeFeishuJSON(w, map[string]string{"message": "ok"})
}

func (a *FeishuAdapter) handleP2MessageReceiveV1(event *larkim.P2MessageReceiveV1) {
	callback := callbackPayloadFromP2MessageReceiveV1(event)
	if msg := a.buildMessage(callback); msg != nil {
		if msg.GroupID == "" {
			log.Printf("[接收][飞书][%s(私聊)]：%s", msg.UserID, msg.Content)
		} else {
			log.Printf("[接收][飞书][%s(群 %s)]：%s", msg.UserID, msg.GroupID, msg.Content)
		}
		a.dispatchMessage(msg)
	}
}

func callbackPayloadFromP2MessageReceiveV1(event *larkim.P2MessageReceiveV1) feishuCallbackPayload {
	var callback feishuCallbackPayload
	callback.Schema = "2.0"
	callback.Header.EventType = "im.message.receive_v1"
	if event == nil {
		return callback
	}
	if event.EventV2Base != nil && event.EventV2Base.Header != nil {
		callback.Header.EventID = event.EventV2Base.Header.EventID
		callback.Header.EventType = event.EventV2Base.Header.EventType
		callback.Header.TenantKey = event.EventV2Base.Header.TenantKey
		callback.Header.Token = event.EventV2Base.Header.Token
		callback.Header.CreateTime = event.EventV2Base.Header.CreateTime
	}
	if event.Event == nil {
		return callback
	}
	if event.Event.Sender != nil {
		callback.Event.Sender.TenantKey = stringPtrValue(event.Event.Sender.TenantKey)
		if event.Event.Sender.SenderId != nil {
			callback.Event.Sender.SenderID.OpenID = stringPtrValue(event.Event.Sender.SenderId.OpenId)
			callback.Event.Sender.SenderID.UserID = stringPtrValue(event.Event.Sender.SenderId.UserId)
			callback.Event.Sender.SenderID.UnionID = stringPtrValue(event.Event.Sender.SenderId.UnionId)
		}
	}
	if event.Event.Message != nil {
		callback.Event.Message.MessageID = stringPtrValue(event.Event.Message.MessageId)
		callback.Event.Message.RootID = stringPtrValue(event.Event.Message.RootId)
		callback.Event.Message.ParentID = stringPtrValue(event.Event.Message.ParentId)
		callback.Event.Message.CreateTime = stringPtrValue(event.Event.Message.CreateTime)
		callback.Event.Message.ChatID = stringPtrValue(event.Event.Message.ChatId)
		callback.Event.Message.ChatType = stringPtrValue(event.Event.Message.ChatType)
		callback.Event.Message.MessageType = stringPtrValue(event.Event.Message.MessageType)
		callback.Event.Message.Content = stringPtrValue(event.Event.Message.Content)
	}
	return callback
}

func (a *FeishuAdapter) parseCallbackPayload(payload []byte) (feishuCallbackPayload, error) {
	var callback feishuCallbackPayload
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(payload, &raw); err != nil {
		return callback, err
	}
	if rawEvent, ok := raw["event"]; ok {
		callback.RawEvent = rawEvent
	}
	if err := json.Unmarshal(payload, &callback); err != nil {
		return callback, err
	}
	return callback, nil
}

func (a *FeishuAdapter) verifyCallbackToken(callback feishuCallbackPayload) bool {
	expected := []byte(a.verificationToken)
	for _, token := range []string{callback.Token, callback.Header.Token} {
		token = strings.TrimSpace(token)
		if token != "" && subtle.ConstantTimeCompare([]byte(token), expected) == 1 {
			return true
		}
	}
	return false
}

func (a *FeishuAdapter) buildMessage(callback feishuCallbackPayload) *types.Message {
	message := callback.Event.Message
	if strings.TrimSpace(message.MessageType) != "text" {
		return nil
	}
	content := extractFeishuTextContent(message.Content)
	if content == "" {
		return nil
	}
	messageID := strings.TrimSpace(message.MessageID)
	eventID := strings.TrimSpace(callback.Header.EventID)
	if messageID == "" {
		messageID = eventID
	}
	chatID := strings.TrimSpace(message.ChatID)
	chatType := strings.TrimSpace(message.ChatType)
	userID := firstNonEmpty(callback.Event.Sender.SenderID.OpenID, callback.Event.Sender.SenderID.UserID, callback.Event.Sender.SenderID.UnionID)
	messageType := "private"
	groupID := ""
	if chatType != "p2p" && chatID != "" {
		messageType = "group"
		groupID = chatID
	}
	feishuMessageID := strings.TrimSpace(callback.Event.Message.MessageID)
	metadata := map[string]string{
		"message_type":           messageType,
		"feishu_event_id":        eventID,
		"feishu_event_type":      strings.TrimSpace(callback.Header.EventType),
		"feishu_message_id":      feishuMessageID,
		"feishu_chat_id":         chatID,
		"feishu_chat_type":       chatType,
		"feishu_message_type":    strings.TrimSpace(message.MessageType),
		"feishu_sender_open_id":  strings.TrimSpace(callback.Event.Sender.SenderID.OpenID),
		"feishu_sender_user_id":  strings.TrimSpace(callback.Event.Sender.SenderID.UserID),
		"feishu_sender_union_id": strings.TrimSpace(callback.Event.Sender.SenderID.UnionID),
		"feishu_tenant_key":      firstNonEmpty(callback.Header.TenantKey, callback.Event.Sender.TenantKey),
	}
	if feishuMessageID != "" {
		metadata["reply_target"] = "reply_" + feishuMessageID
	}
	return &types.Message{
		ID:       messageID,
		Platform: platformName,
		UserID:   userID,
		GroupID:  groupID,
		Content:  content,
		Metadata: metadata,
	}
}

func extractFeishuTextContent(raw string) string {
	var content struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal([]byte(raw), &content); err != nil {
		return ""
	}
	return strings.TrimSpace(content.Text)
}

func (a *FeishuAdapter) dispatchMessage(msg *types.Message) {
	if a.messageHandler != nil {
		a.messageHandler(msg)
	}
}

func (a *FeishuAdapter) sendFeishuMessage(target string, msgType string, content string) error {
	parsed, err := parseFeishuMessageTarget(target)
	if err != nil {
		return err
	}
	body := map[string]interface{}{
		"msg_type": msgType,
		"content":  content,
	}
	if parsed.kind == "reply" {
		return a.callAPI(http.MethodPost, "/im/v1/messages/"+url.PathEscape(parsed.id)+"/reply", body, nil)
	}
	body["receive_id"] = parsed.id
	path := "/im/v1/messages?receive_id_type=" + url.QueryEscape(parsed.kind)
	return a.callAPI(http.MethodPost, path, body, nil)
}

func (a *FeishuAdapter) uploadImage(imageURL string) (string, error) {
	reader, filename, err := a.openImageReader(imageURL)
	if err != nil {
		return "", err
	}
	defer reader.Close()

	imageData, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}
	var result feishuImageUploadResponse
	if err := a.callMultipartAPI("/im/v1/images", imageData, filename, &result, true); err != nil {
		return "", err
	}
	imageKey := strings.TrimSpace(result.Data.ImageKey)
	if imageKey == "" {
		return "", fmt.Errorf("飞书图片上传响应缺少 image_key")
	}
	return imageKey, nil
}

func (a *FeishuAdapter) openImageReader(imageURL string) (io.ReadCloser, string, error) {
	parsed, err := url.Parse(imageURL)
	if err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") {
		request, err := http.NewRequest(http.MethodGet, imageURL, nil)
		if err != nil {
			return nil, "", err
		}
		request.Header.Set("User-Agent", "Mozilla/5.0 AllBot/1.0")
		resp, err := a.httpClient.Do(request)
		if err != nil {
			return nil, "", err
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)
			return nil, "", fmt.Errorf("下载飞书图片资源状态码 %d: %s", resp.StatusCode, string(body))
		}
		filename := filepath.Base(parsed.Path)
		if filename == "." || filename == "/" || filename == "" {
			filename = "image"
		}
		return resp.Body, filename, nil
	}

	path := imageURL
	if err == nil && parsed.Scheme == "file" {
		path = parsed.Path
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, "", fmt.Errorf("打开飞书图片文件失败: %w", err)
	}
	filename := filepath.Base(path)
	if filename == "." || filename == string(filepath.Separator) || filename == "" {
		filename = "image"
	}
	return file, filename, nil
}

func (a *FeishuAdapter) callMultipartAPI(path string, image []byte, filename string, result interface{}, retryToken bool) error {
	body, contentType, err := buildFeishuImageUploadBody(image, filename)
	if err != nil {
		return err
	}

	token, err := a.getTenantAccessToken()
	if err != nil {
		return err
	}
	requestURL := strings.TrimRight(a.apiBaseURL, "/") + "/" + strings.TrimLeft(path, "/")
	request, err := http.NewRequest(http.MethodPost, requestURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", contentType)
	resp, err := a.httpClient.Do(request)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode == http.StatusUnauthorized && retryToken {
		a.clearTenantAccessToken()
		return a.callMultipartAPI(path, image, filename, result, false)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("飞书 API POST %s 状态码 %d: %s", path, resp.StatusCode, string(payload))
	}
	var apiResult feishuAPIResponse
	if len(payload) > 0 {
		if err := json.Unmarshal(payload, &apiResult); err != nil {
			return err
		}
	}
	if apiResult.Code != 0 {
		if retryToken && feishuTokenExpired(apiResult.Code) {
			a.clearTenantAccessToken()
			return a.callMultipartAPI(path, image, filename, result, false)
		}
		return fmt.Errorf("飞书 API POST %s 失败 code=%d msg=%s", path, apiResult.Code, apiResult.Msg)
	}
	if result == nil || len(payload) == 0 {
		return nil
	}
	return json.Unmarshal(payload, result)
}

func buildFeishuImageUploadBody(image []byte, filename string) ([]byte, string, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("image_type", "message"); err != nil {
		return nil, "", err
	}
	part, err := writer.CreateFormFile("image", filename)
	if err != nil {
		return nil, "", err
	}
	if _, err := part.Write(image); err != nil {
		return nil, "", err
	}
	if err := writer.Close(); err != nil {
		return nil, "", err
	}
	return body.Bytes(), writer.FormDataContentType(), nil
}

func (a *FeishuAdapter) getTenantAccessToken() (string, error) {
	a.tokenMu.Lock()
	defer a.tokenMu.Unlock()
	if a.tenantAccessToken != "" && time.Now().Before(a.tokenExpiresAt.Add(-feishuTokenRefreshLead)) {
		return a.tenantAccessToken, nil
	}
	payload, err := json.Marshal(map[string]string{"app_id": a.appID, "app_secret": a.appSecret})
	if err != nil {
		return "", err
	}
	request, err := http.NewRequest(http.MethodPost, a.tokenURL, bytes.NewBuffer(payload))
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", "application/json")
	resp, err := a.httpClient.Do(request)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("飞书 token 接口状态码 %d: %s", resp.StatusCode, string(body))
	}
	var result feishuTenantAccessTokenResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}
	if result.Code != 0 {
		return "", fmt.Errorf("飞书 token 获取失败 code=%d msg=%s", result.Code, result.Msg)
	}
	if strings.TrimSpace(result.TenantAccessToken) == "" {
		return "", fmt.Errorf("飞书 token 响应缺少 tenant_access_token")
	}
	if result.Expire <= 0 {
		return "", fmt.Errorf("飞书 token 响应缺少有效 expire")
	}
	a.tenantAccessToken = strings.TrimSpace(result.TenantAccessToken)
	a.tokenExpiresAt = time.Now().Add(time.Duration(result.Expire) * time.Second)
	return a.tenantAccessToken, nil
}

func (a *FeishuAdapter) callAPI(method, path string, body interface{}, result interface{}) error {
	return a.callAPIWithRetry(method, path, body, result, true)
}

func (a *FeishuAdapter) callAPIWithRetry(method, path string, body interface{}, result interface{}, retryToken bool) error {
	token, err := a.getTenantAccessToken()
	if err != nil {
		return err
	}
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewBuffer(payload)
	}
	requestURL := strings.TrimRight(a.apiBaseURL, "/") + "/" + strings.TrimLeft(path, "/")
	request, err := http.NewRequest(method, requestURL, reader)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	resp, err := a.httpClient.Do(request)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode == http.StatusUnauthorized && retryToken {
		a.clearTenantAccessToken()
		return a.callAPIWithRetry(method, path, body, result, false)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("飞书 API %s %s 状态码 %d: %s", method, path, resp.StatusCode, string(payload))
	}
	var apiResult feishuAPIResponse
	if len(payload) > 0 {
		if err := json.Unmarshal(payload, &apiResult); err != nil {
			return err
		}
	}
	if apiResult.Code != 0 {
		if retryToken && feishuTokenExpired(apiResult.Code) {
			a.clearTenantAccessToken()
			return a.callAPIWithRetry(method, path, body, result, false)
		}
		return fmt.Errorf("飞书 API %s %s 失败 code=%d msg=%s", method, path, apiResult.Code, apiResult.Msg)
	}
	if result == nil || len(payload) == 0 {
		return nil
	}
	return json.Unmarshal(payload, result)
}

func (a *FeishuAdapter) clearTenantAccessToken() {
	a.tokenMu.Lock()
	defer a.tokenMu.Unlock()
	a.tenantAccessToken = ""
	a.tokenExpiresAt = time.Time{}
}

func feishuTokenExpired(code int) bool {
	return code == 99991663 || code == 99991664 || code == 99991665 || code == 99991668 || code == 99991672
}

func parseFeishuMessageTarget(target string) (feishuMessageTarget, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return feishuMessageTarget{}, fmt.Errorf("飞书消息目标不能为空")
	}
	for _, item := range []struct{ prefix, kind, name string }{
		{prefix: "user_", kind: "open_id", name: "open_id"},
		{prefix: "chat_", kind: "chat_id", name: "chat_id"},
		{prefix: "reply_", kind: "reply", name: "message_id"},
	} {
		if strings.HasPrefix(target, item.prefix) {
			id := strings.TrimSpace(strings.TrimPrefix(target, item.prefix))
			if id == "" {
				return feishuMessageTarget{}, fmt.Errorf("飞书 %s 不能为空", item.name)
			}
			return feishuMessageTarget{kind: item.kind, id: id}, nil
		}
	}
	return feishuMessageTarget{}, fmt.Errorf("飞书消息目标格式无效: %s", target)
}

func hasFeishuTargetPrefix(value string) bool {
	return strings.HasPrefix(value, "user_") || strings.HasPrefix(value, "chat_") || strings.HasPrefix(value, "reply_")
}

func normalizeCallbackPath(path string) string {
	path = strings.Trim(strings.TrimSpace(path), "/")
	if path == "" {
		return feishuDefaultCallbackPath
	}
	return path
}

func writeFeishuJSON(w http.ResponseWriter, body interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(body)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func stringPtrValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}
