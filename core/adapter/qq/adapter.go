package qq

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/allbot/allbot/core/adapter/_contract"
	"github.com/allbot/allbot/core/types"
	"github.com/gorilla/websocket"
)

type UserInfo = contract.UserInfo
type GroupInfo = contract.GroupInfo

const (
	defaultFramework      = "napcat"
	defaultActionTimeout  = 10 * time.Second
	maxOneBotMessageBytes = 4 << 20
	messageEventQueueSize = 128
	maxConcurrentActions  = 64
)

// QQAdapterConfig 描述 QQ OneBot 适配器连接参数。
type QQAdapterConfig struct {
	Framework          string
	ServerURL          string
	HTTPAPIURL         string
	AccessToken        string
	HTTPAPIAccessToken string
}

// QQAdapter QQ 平台适配器，连接 OneBot 11 正向通用 WebSocket 服务。
type QQAdapter struct {
	framework          string
	serverURL          string
	httpAPIURL         string
	accessToken        string
	httpAPIAccessToken string
	messageHandler     func(*types.Message)
	conn               *websocket.Conn
	pending            map[string]chan oneBotAPIResponse
	selfID             string
	recentSent         map[string]time.Time
	eventQueue         chan map[string]interface{}
	actionTimeout      time.Duration
	httpClient         *http.Client
	requestContext     context.Context
	cancelRequests     context.CancelFunc
	actionSlots        chan struct{}
	writeSlots         chan struct{}
	echoSequence       atomic.Uint64
	mu                 sync.Mutex
	recentMu           sync.Mutex
	closed             chan struct{}
	closeOnce          sync.Once
}

type oneBotAPIResponse struct {
	Status  string          `json:"status"`
	RetCode int             `json:"retcode"`
	Data    json.RawMessage `json:"data"`
	Message string          `json:"message"`
	Wording string          `json:"wording"`
	Echo    string          `json:"echo"`
}

// NewQQAdapter 创建 QQ 适配器。
func NewQQAdapter(config QQAdapterConfig) *QQAdapter {
	framework := strings.ToLower(strings.TrimSpace(config.Framework))
	if framework == "" {
		framework = defaultFramework
	}
	requestContext, cancelRequests := context.WithCancel(context.Background())
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxConnsPerHost = maxConcurrentActions
	transport.MaxIdleConnsPerHost = maxConcurrentActions
	return &QQAdapter{
		framework:          framework,
		serverURL:          strings.TrimSpace(config.ServerURL),
		httpAPIURL:         strings.TrimRight(strings.TrimSpace(config.HTTPAPIURL), "/"),
		accessToken:        strings.TrimSpace(config.AccessToken),
		httpAPIAccessToken: strings.TrimSpace(config.HTTPAPIAccessToken),
		pending:            make(map[string]chan oneBotAPIResponse),
		recentSent:         make(map[string]time.Time),
		eventQueue:         make(chan map[string]interface{}, messageEventQueueSize),
		actionTimeout:      defaultActionTimeout,
		httpClient:         &http.Client{Transport: transport, Timeout: defaultActionTimeout},
		requestContext:     requestContext,
		cancelRequests:     cancelRequests,
		actionSlots:        make(chan struct{}, maxConcurrentActions),
		writeSlots:         make(chan struct{}, 1),
		closed:             make(chan struct{}),
	}
}

func (a *QQAdapter) GetPlatform() string {
	return "qq"
}

func (a *QQAdapter) GetBotIdentity(msg *types.Message) contract.BotIdentity {
	a.mu.Lock()
	defer a.mu.Unlock()
	return contract.BotIdentity{Label: "机器人 QQ", Value: strings.TrimSpace(a.selfID)}
}

func (a *QQAdapter) SetMessageHandler(handler func(*types.Message)) {
	a.messageHandler = handler
}

func (a *QQAdapter) ReplyTarget(msg *types.Message) string {
	if msg == nil {
		return ""
	}
	if msg.GroupID != "" {
		return "group_" + msg.GroupID
	}
	return msg.UserID
}

func (a *QQAdapter) FormatReplyText(msg *types.Message, text string) string {
	if msg == nil || msg.GroupID == "" {
		return text
	}
	return fmt.Sprintf("[CQ:at,qq=%s] %s", msg.UserID, text)
}

func (a *QQAdapter) SendTarget(userID string, groupID string) string {
	if groupID != "" {
		return "group_" + groupID
	}
	return userID
}

// Start 建立事件 WebSocket，并同步探测 OneBot action 通道。
func (a *QQAdapter) Start() error {
	if a.serverURL == "" {
		return fmt.Errorf("OneBot 正向 WebSocket 地址不能为空")
	}
	header := http.Header{}
	if a.accessToken != "" {
		header.Set("Authorization", "Bearer "+a.accessToken)
	}
	dialer := *websocket.DefaultDialer
	dialer.HandshakeTimeout = defaultActionTimeout
	conn, response, err := dialer.DialContext(a.requestContext, a.serverURL, header)
	if err != nil {
		if response != nil {
			return fmt.Errorf("连接 %s OneBot WebSocket 失败: HTTP %d: %w", a.frameworkName(), response.StatusCode, err)
		}
		return fmt.Errorf("连接 %s OneBot WebSocket 失败: %w", a.frameworkName(), err)
	}

	conn.SetReadLimit(maxOneBotMessageBytes)
	a.mu.Lock()
	a.conn = conn
	a.mu.Unlock()
	go a.readLoop(conn)
	go a.eventLoop()

	var loginInfo map[string]interface{}
	if err := a.callAPIWithResult("get_login_info", map[string]interface{}{}, &loginInfo); err != nil {
		_ = a.Stop()
		return fmt.Errorf("%s OneBot 启动探测失败: %w", a.frameworkName(), err)
	}
	selfID := stringValue(loginInfo["user_id"])
	if selfID == "" {
		_ = a.Stop()
		return fmt.Errorf("%s OneBot 启动探测失败: get_login_info 未返回 user_id", a.frameworkName())
	}
	a.mu.Lock()
	if a.conn != conn {
		a.mu.Unlock()
		return fmt.Errorf("%s OneBot 启动探测期间 WebSocket 已关闭", a.frameworkName())
	}
	a.selfID = selfID
	a.mu.Unlock()
	select {
	case <-a.closed:
		return fmt.Errorf("%s OneBot 启动探测期间 WebSocket 已关闭", a.frameworkName())
	default:
	}
	log.Printf("QQ Adapter 已连接 %s OneBot WebSocket: %s，自身账号: %s", a.frameworkName(), a.serverURL, selfID)
	return nil
}

func (a *QQAdapter) frameworkName() string {
	if a.framework == "napcat" {
		return "NapCat"
	}
	return a.framework
}

// Stop 停止适配器。
func (a *QQAdapter) Stop() error {
	var closeErr error
	a.closeOnce.Do(func() {
		close(a.closed)
		a.cancelRequests()
		a.mu.Lock()
		conn := a.conn
		a.conn = nil
		pending := a.pending
		a.pending = make(map[string]chan oneBotAPIResponse)
		a.mu.Unlock()
		if conn != nil {
			closeErr = conn.Close()
		}
		for _, ch := range pending {
			close(ch)
		}
	})
	return closeErr
}

func (a *QQAdapter) readLoop(conn *websocket.Conn) {
	for {
		_, payload, err := conn.ReadMessage()
		if err != nil {
			select {
			case <-a.closed:
				return
			default:
				log.Printf("QQ Adapter 读取 %s OneBot 消息失败: %v", a.frameworkName(), err)
				_ = a.Stop()
				return
			}
		}
		a.handleOneBotPayload(payload)
	}
}

func (a *QQAdapter) handleOneBotPayload(payload []byte) {
	var event map[string]interface{}
	if err := json.Unmarshal(payload, &event); err != nil {
		log.Printf("QQ Adapter 解析 %s OneBot 消息失败: %v", a.frameworkName(), err)
		return
	}
	if echo := stringValue(event["echo"]); echo != "" {
		a.resolveAPIResponse(echo, payload)
		return
	}
	if stringValue(event["post_type"]) != "message" {
		return
	}
	select {
	case a.eventQueue <- event:
	case <-a.closed:
	default:
		log.Printf("QQ Adapter %s OneBot 消息队列已满，丢弃事件", a.frameworkName())
	}
}

func (a *QQAdapter) eventLoop() {
	for {
		select {
		case <-a.closed:
			return
		default:
		}
		select {
		case <-a.closed:
			return
		case event := <-a.eventQueue:
			select {
			case <-a.closed:
				return
			default:
				a.handleMessageEvent(event)
			}
		}
	}
}

func (a *QQAdapter) resolveAPIResponse(echo string, payload []byte) {
	var response oneBotAPIResponse
	if err := json.Unmarshal(payload, &response); err != nil {
		log.Printf("QQ Adapter 解析 OneBot API 响应失败: %v", err)
		return
	}
	a.mu.Lock()
	ch := a.pending[echo]
	delete(a.pending, echo)
	a.mu.Unlock()
	if ch != nil {
		ch <- response
		close(ch)
	}
}

func (a *QQAdapter) handleMessageEvent(event map[string]interface{}) {
	messageType := stringValue(event["message_type"])
	userID := stringValue(event["user_id"])
	selfID := stringValue(event["self_id"])
	if selfID == "" {
		a.mu.Lock()
		selfID = a.selfID
		a.mu.Unlock()
	}
	if selfID != "" && selfID == userID {
		return
	}
	content := messageText(event["message"])
	if content == "" {
		return
	}
	msg := &types.Message{
		ID:       stringValue(event["message_id"]),
		Platform: "qq",
		UserID:   userID,
		Content:  content,
		Metadata: map[string]string{"message_type": messageType},
	}
	chatInfo := "私聊"
	targetID := userID
	if messageType == "group" {
		msg.GroupID = stringValue(event["group_id"])
		targetID = msg.GroupID
		chatInfo = "群组" + msg.GroupID
	}
	if a.isRecentSent(messageType, targetID, content) {
		return
	}
	log.Printf("[接收][QQ][%s(%s)]：%s", userID, chatInfo, content)
	if a.messageHandler != nil {
		a.messageHandler(msg)
	}
}

// SendMessage 发送消息。
func (a *QQAdapter) SendMessage(target string, text string) error {
	messageType := "private"
	targetID := target
	if strings.HasPrefix(target, "group_") {
		messageType = "group"
		targetID = strings.TrimPrefix(target, "group_")
	}
	params := map[string]interface{}{"message_type": messageType, "message": text}
	if messageType == "group" {
		params["group_id"] = parseQQID(targetID)
	} else {
		params["user_id"] = parseQQID(targetID)
	}
	log.Printf("[发送][QQ][%s]：%s", target, text)
	a.markRecentSent(messageType, targetID, text)
	return a.callAPI("send_msg", params)
}

func (a *QQAdapter) markRecentSent(messageType string, targetID string, content string) {
	a.recentMu.Lock()
	defer a.recentMu.Unlock()
	now := time.Now()
	for key, expiresAt := range a.recentSent {
		if now.After(expiresAt) {
			delete(a.recentSent, key)
		}
	}
	a.recentSent[recentSentKey(messageType, targetID, content)] = now.Add(30 * time.Second)
}

func (a *QQAdapter) isRecentSent(messageType string, targetID string, content string) bool {
	a.recentMu.Lock()
	defer a.recentMu.Unlock()
	key := recentSentKey(messageType, targetID, content)
	expiresAt, ok := a.recentSent[key]
	if !ok {
		return false
	}
	if time.Now().After(expiresAt) {
		delete(a.recentSent, key)
		return false
	}
	return true
}

func recentSentKey(messageType string, targetID string, content string) string {
	return messageType + "|" + targetID + "|" + content
}

func (a *QQAdapter) SendImage(target string, imageURL string) error {
	return a.SendMessage(target, fmt.Sprintf("[CQ:image,file=%s]", imageURL))
}

func (a *QQAdapter) SendFile(target string, filePath string) error {
	return fmt.Errorf("QQ 文件发送暂未实现")
}

func (a *QQAdapter) GetUserInfo(userID string) (*UserInfo, error) {
	var data map[string]interface{}
	if err := a.callAPIWithResult("get_stranger_info", map[string]interface{}{"user_id": parseQQID(userID)}, &data); err != nil {
		return nil, err
	}
	return &UserInfo{UserID: userID, Nickname: stringValue(data["nickname"]), Avatar: fmt.Sprintf("https://q1.qlogo.cn/g?b=qq&nk=%s&s=640", userID), Extra: make(map[string]string)}, nil
}

func (a *QQAdapter) GetGroupInfo(groupID string) (*GroupInfo, error) {
	var data map[string]interface{}
	if err := a.callAPIWithResult("get_group_info", map[string]interface{}{"group_id": parseQQID(groupID)}, &data); err != nil {
		return nil, err
	}
	return &GroupInfo{GroupID: groupID, Name: stringValue(data["group_name"]), MemberCount: int(numberValue(data["member_count"])), Extra: make(map[string]string)}, nil
}

func (a *QQAdapter) AtUser(groupID string, userID string) error {
	return a.SendMessage("group_"+groupID, fmt.Sprintf("[CQ:at,qq=%s]", userID))
}

func (a *QQAdapter) callAPI(action string, params map[string]interface{}) error {
	var response oneBotAPIResponse
	if err := a.callAPIWithResponse(action, params, &response); err != nil {
		return err
	}
	return validateOneBotResponse(action, response)
}

func (a *QQAdapter) callAPIWithResult(action string, params map[string]interface{}, result interface{}) error {
	var response oneBotAPIResponse
	if err := a.callAPIWithResponse(action, params, &response); err != nil {
		return err
	}
	if err := validateOneBotResponse(action, response); err != nil {
		return err
	}
	if len(response.Data) == 0 || string(response.Data) == "null" {
		return nil
	}
	return json.Unmarshal(response.Data, result)
}

func validateOneBotResponse(action string, response oneBotAPIResponse) error {
	if response.Status == "ok" && response.RetCode == 0 {
		return nil
	}
	detail := strings.TrimSpace(response.Message)
	wording := strings.TrimSpace(response.Wording)
	if wording != "" && wording != detail {
		if detail != "" {
			detail += ": " + wording
		} else {
			detail = wording
		}
	}
	if detail == "" {
		detail = "服务端未提供错误信息"
	}
	return fmt.Errorf("OneBot API %s 失败: status=%s retcode=%d %s", action, response.Status, response.RetCode, detail)
}

func (a *QQAdapter) callAPIWithResponse(action string, params map[string]interface{}, result *oneBotAPIResponse) error {
	ctx, cancel := context.WithTimeout(a.requestContext, a.actionTimeout)
	defer cancel()
	select {
	case a.actionSlots <- struct{}{}:
		defer func() { <-a.actionSlots }()
	case <-ctx.Done():
		return fmt.Errorf("OneBot API %s 调用超时: %w", action, ctx.Err())
	}
	if a.httpAPIURL != "" {
		return a.callHTTPAPI(ctx, action, params, result)
	}
	return a.callWebSocketAPI(ctx, action, params, result)
}

func (a *QQAdapter) callWebSocketAPI(ctx context.Context, action string, params map[string]interface{}, result *oneBotAPIResponse) error {
	echo := fmt.Sprintf("allbot-%d", a.echoSequence.Add(1))
	ch := make(chan oneBotAPIResponse, 1)
	a.mu.Lock()
	conn := a.conn
	if conn == nil {
		a.mu.Unlock()
		return fmt.Errorf("OneBot WebSocket 未连接")
	}
	a.pending[echo] = ch
	a.mu.Unlock()

	payload := map[string]interface{}{"action": action, "params": params, "echo": echo}
	select {
	case a.writeSlots <- struct{}{}:
	case <-ctx.Done():
		a.removePending(echo)
		return fmt.Errorf("OneBot API %s 调用超时: %w", action, ctx.Err())
	}
	deadline, ok := ctx.Deadline()
	if ok {
		_ = conn.SetWriteDeadline(deadline)
	}
	err := conn.WriteJSON(payload)
	_ = conn.SetWriteDeadline(time.Time{})
	<-a.writeSlots
	if err != nil {
		a.removePending(echo)
		return err
	}

	select {
	case response, ok := <-ch:
		if !ok {
			return fmt.Errorf("OneBot WebSocket 已关闭")
		}
		*result = response
		return nil
	case <-ctx.Done():
		a.removePending(echo)
		return fmt.Errorf("OneBot API %s 响应超时: %w", action, ctx.Err())
	}
}

func (a *QQAdapter) removePending(echo string) {
	a.mu.Lock()
	delete(a.pending, echo)
	a.mu.Unlock()
}

func (a *QQAdapter) callHTTPAPI(ctx context.Context, action string, params map[string]interface{}, result *oneBotAPIResponse) error {
	jsonData, err := json.Marshal(params)
	if err != nil {
		return err
	}
	actionURL, err := url.JoinPath(a.httpAPIURL, action)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, actionURL, bytes.NewReader(jsonData))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	if a.httpAPIAccessToken != "" {
		request.Header.Set("Authorization", "Bearer "+a.httpAPIAccessToken)
	}
	response, err := a.httpClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxOneBotMessageBytes+1))
	if err != nil {
		return err
	}
	if len(body) > maxOneBotMessageBytes {
		return fmt.Errorf("OneBot HTTP API 响应超过 %d 字节限制", maxOneBotMessageBytes)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("OneBot HTTP API 状态码 %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	}
	if err := json.Unmarshal(body, result); err != nil {
		return fmt.Errorf("解析 OneBot HTTP API 响应失败: %w", err)
	}
	return nil
}

func messageText(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case []interface{}:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			segment, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			segmentType := stringValue(segment["type"])
			data, _ := segment["data"].(map[string]interface{})
			switch segmentType {
			case "text":
				parts = append(parts, stringValue(data["text"]))
			case "at":
				parts = append(parts, "[CQ:at,qq="+stringValue(data["qq"])+"]")
			case "image":
				parts = append(parts, "[图片]")
			}
		}
		return strings.TrimSpace(strings.Join(parts, ""))
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", value))
	}
}

func stringValue(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return typed
	case float64:
		return strconv.FormatInt(int64(typed), 10)
	case json.Number:
		return typed.String()
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", typed)
	}
}

func numberValue(value interface{}) int64 {
	switch typed := value.(type) {
	case float64:
		return int64(typed)
	case string:
		parsed, _ := strconv.ParseInt(typed, 10, 64)
		return parsed
	case json.Number:
		parsed, _ := typed.Int64()
		return parsed
	default:
		return 0
	}
}

func parseQQID(value string) interface{} {
	if parsed, err := strconv.ParseInt(value, 10, 64); err == nil {
		return parsed
	}
	return value
}
