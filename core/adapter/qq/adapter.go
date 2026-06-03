package qq

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/allbot/allbot/core/adapter/_contract"
	"github.com/allbot/allbot/core/types"
)

type UserInfo = contract.UserInfo
type GroupInfo = contract.GroupInfo

// QQAdapter QQ 平台适配器，主动连接 NapCat 提供的 OneBot 正向 WebSocket 服务。
type QQAdapter struct {
	serverURL      string
	accessToken    string
	apiURL         string
	messageHandler func(*types.Message)
	conn           net.Conn
	reader         *bufio.Reader
	pending        map[string]chan oneBotAPIResponse
	selfID         string
	recentSent     map[string]time.Time
	mu             sync.Mutex
	recentMu       sync.Mutex
	writeMu        sync.Mutex
	closed         chan struct{}
	closeOnce      sync.Once
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
func NewQQAdapter(serverURL string, accessToken string) *QQAdapter {
	return &QQAdapter{
		serverURL:   strings.TrimSpace(serverURL),
		accessToken: strings.TrimSpace(accessToken),
		apiURL:      oneBotAPIURL(serverURL),
		pending:     make(map[string]chan oneBotAPIResponse),
		recentSent:  make(map[string]time.Time),
		closed:      make(chan struct{}),
	}
}

// GetPlatform 获取平台名称。
func (a *QQAdapter) GetPlatform() string {
	return "qq"
}

// SetMessageHandler 设置消息处理器。
func (a *QQAdapter) SetMessageHandler(handler func(*types.Message)) {
	a.messageHandler = handler
}

// ReplyTarget 按 OneBot 发送接口约定解析回复目标。
func (a *QQAdapter) ReplyTarget(msg *types.Message) string {
	if msg == nil {
		return ""
	}
	if msg.GroupID != "" {
		return "group_" + msg.GroupID
	}
	return msg.UserID
}

// FormatReplyText 在群聊回复前拼接 OneBot CQ at 码。
func (a *QQAdapter) FormatReplyText(msg *types.Message, text string) string {
	if msg == nil || msg.GroupID == "" {
		return text
	}
	return fmt.Sprintf("[CQ:at,qq=%s] %s", msg.UserID, text)
}

// SendTarget 按 OneBot 发送接口约定解析插件主动发送目标。
func (a *QQAdapter) SendTarget(userID string, groupID string) string {
	if groupID != "" {
		return "group_" + groupID
	}
	return userID
}

// Start 连接 NapCat OneBot WebSocket 服务。
func (a *QQAdapter) Start() error {
	if a.serverURL == "" {
		return fmt.Errorf("NapCat 服务地址不能为空")
	}
	conn, reader, err := dialOneBotWebSocket(a.serverURL, a.accessToken)
	if err != nil {
		return fmt.Errorf("连接 NapCat OneBot WebSocket 失败: %w", err)
	}

	a.mu.Lock()
	a.conn = conn
	a.reader = reader
	a.mu.Unlock()

	log.Printf("QQ Adapter 已连接 NapCat: %s", a.serverURL)
	go a.readLoop()
	go a.refreshSelfID()
	return nil
}

// Stop 停止适配器。
func (a *QQAdapter) Stop() error {
	a.closeOnce.Do(func() {
		close(a.closed)
	})
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.conn != nil {
		err := a.conn.Close()
		a.conn = nil
		returnPending := a.pending
		a.pending = make(map[string]chan oneBotAPIResponse)
		for _, ch := range returnPending {
			close(ch)
		}
		return err
	}
	return nil
}

func (a *QQAdapter) readLoop() {
	for {
		select {
		case <-a.closed:
			return
		default:
		}

		messageType, payload, err := readWebSocketFrame(a.reader)
		if err != nil {
			select {
			case <-a.closed:
				return
			default:
				log.Printf("QQ Adapter 读取 NapCat 消息失败: %v", err)
				_ = a.Stop()
				return
			}
		}

		switch messageType {
		case 1, 2:
			a.handleOneBotPayload(payload)
		case 8:
			_ = a.Stop()
			return
		case 9:
			_ = a.writeWebSocketFrame(10, payload)
		}
	}
}

func (a *QQAdapter) handleOneBotPayload(payload []byte) {
	var event map[string]interface{}
	if err := json.Unmarshal(payload, &event); err != nil {
		log.Printf("QQ Adapter 解析 NapCat 消息失败: %v", err)
		return
	}

	if echo := stringValue(event["echo"]); echo != "" {
		a.resolveAPIResponse(echo, payload)
		return
	}

	if stringValue(event["post_type"]) == "message" {
		a.handleMessageEvent(event)
	}
}

func (a *QQAdapter) resolveAPIResponse(echo string, payload []byte) {
	var response oneBotAPIResponse
	if err := json.Unmarshal(payload, &response); err != nil {
		log.Printf("QQ Adapter 解析 API 响应失败: %v", err)
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
	messageID := stringValue(event["message_id"])

	if content == "" {
		return
	}

	msg := &types.Message{
		ID:       messageID,
		Platform: "qq",
		UserID:   userID,
		Content:  content,
		Metadata: map[string]string{
			"message_type": messageType,
		},
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

	params := map[string]interface{}{
		"message_type": messageType,
		"message":      text,
	}
	if messageType == "group" {
		params["group_id"] = parseQQID(targetID)
	} else {
		params["user_id"] = parseQQID(targetID)
	}

	log.Printf("[发送][QQ][%s]：%s", target, text)
	a.markRecentSent(messageType, targetID, text)
	return a.callAPI("send_msg", params)
}

func (a *QQAdapter) refreshSelfID() {
	var data map[string]interface{}
	if err := a.callAPIWithResult("get_login_info", map[string]interface{}{}, &data); err != nil {
		log.Printf("QQ Adapter 获取自身账号失败: %v", err)
		return
	}
	selfID := stringValue(data["user_id"])
	if selfID == "" {
		return
	}
	a.mu.Lock()
	a.selfID = selfID
	a.mu.Unlock()
	log.Printf("QQ Adapter 自身账号: %s", selfID)
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

// SendImage 发送图片。
func (a *QQAdapter) SendImage(target string, imageURL string) error {
	message := fmt.Sprintf("[CQ:image,file=%s]", imageURL)
	return a.SendMessage(target, message)
}

// SendFile 发送文件。
func (a *QQAdapter) SendFile(target string, filePath string) error {
	return fmt.Errorf("QQ 文件发送暂未实现")
}

// GetUserInfo 获取用户信息。
func (a *QQAdapter) GetUserInfo(userID string) (*UserInfo, error) {
	var data map[string]interface{}
	if err := a.callAPIWithResult("get_stranger_info", map[string]interface{}{"user_id": parseQQID(userID)}, &data); err != nil {
		return nil, err
	}
	return &UserInfo{
		UserID:   userID,
		Nickname: stringValue(data["nickname"]),
		Avatar:   fmt.Sprintf("https://q1.qlogo.cn/g?b=qq&nk=%s&s=640", userID),
		Extra:    make(map[string]string),
	}, nil
}

// GetGroupInfo 获取群组信息。
func (a *QQAdapter) GetGroupInfo(groupID string) (*GroupInfo, error) {
	var data map[string]interface{}
	if err := a.callAPIWithResult("get_group_info", map[string]interface{}{"group_id": parseQQID(groupID)}, &data); err != nil {
		return nil, err
	}
	return &GroupInfo{
		GroupID:     groupID,
		Name:        stringValue(data["group_name"]),
		MemberCount: int(numberValue(data["member_count"])),
		Extra:       make(map[string]string),
	}, nil
}

// AtUser @某人。
func (a *QQAdapter) AtUser(groupID string, userID string) error {
	message := fmt.Sprintf("[CQ:at,qq=%s]", userID)
	return a.SendMessage("group_"+groupID, message)
}

func (a *QQAdapter) callAPI(action string, params map[string]interface{}) error {
	var response oneBotAPIResponse
	if err := a.callAPIWithResponse(action, params, &response); err != nil {
		return err
	}
	if response.Status != "ok" && response.RetCode != 0 {
		message := response.Message
		if message == "" {
			message = response.Wording
		}
		return fmt.Errorf("OneBot API %s 失败: retcode=%d %s", action, response.RetCode, message)
	}
	return nil
}

func (a *QQAdapter) callAPIWithResult(action string, params map[string]interface{}, result interface{}) error {
	var response oneBotAPIResponse
	if err := a.callAPIWithResponse(action, params, &response); err != nil {
		return err
	}
	if response.Status != "ok" && response.RetCode != 0 {
		return fmt.Errorf("OneBot API %s 失败: retcode=%d %s", action, response.RetCode, response.Message)
	}
	if len(response.Data) == 0 || string(response.Data) == "null" {
		return nil
	}
	return json.Unmarshal(response.Data, result)
}

func (a *QQAdapter) callAPIWithResponse(action string, params map[string]interface{}, result *oneBotAPIResponse) error {
	if err := a.callWebSocketAPI(action, params, result); err == nil {
		return nil
	}
	if a.apiURL == "" {
		return fmt.Errorf("OneBot WebSocket 调用失败，且 HTTP API 地址不可用")
	}
	return a.callHTTPAPI(action, params, result)
}

func (a *QQAdapter) callWebSocketAPI(action string, params map[string]interface{}, result *oneBotAPIResponse) error {
	echo := fmt.Sprintf("allbot-%d", time.Now().UnixNano())
	ch := make(chan oneBotAPIResponse, 1)

	a.mu.Lock()
	if a.conn == nil {
		a.mu.Unlock()
		return fmt.Errorf("WebSocket 未连接")
	}
	a.pending[echo] = ch
	a.mu.Unlock()

	payload, _ := json.Marshal(map[string]interface{}{
		"action": action,
		"params": params,
		"echo":   echo,
	})
	if err := a.writeWebSocketFrame(1, payload); err != nil {
		a.mu.Lock()
		delete(a.pending, echo)
		a.mu.Unlock()
		return err
	}

	select {
	case response, ok := <-ch:
		if !ok {
			return fmt.Errorf("WebSocket 已关闭")
		}
		*result = response
		return nil
	case <-time.After(10 * time.Second):
		a.mu.Lock()
		delete(a.pending, echo)
		a.mu.Unlock()
		return fmt.Errorf("OneBot API %s 响应超时", action)
	}
}

func (a *QQAdapter) callHTTPAPI(action string, params map[string]interface{}, result *oneBotAPIResponse) error {
	jsonData, err := json.Marshal(params)
	if err != nil {
		return err
	}

	request, err := http.NewRequest(http.MethodPost, strings.TrimRight(a.apiURL, "/")+"/"+action, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	if a.accessToken != "" {
		request.Header.Set("Authorization", "Bearer "+a.accessToken)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(request)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP API 状态码 %d: %s", resp.StatusCode, string(body))
	}
	return json.Unmarshal(body, result)
}

func dialOneBotWebSocket(rawURL string, accessToken string) (net.Conn, *bufio.Reader, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, nil, err
	}
	if parsed.Scheme != "ws" && parsed.Scheme != "wss" {
		return nil, nil, fmt.Errorf("服务地址必须以 ws:// 或 wss:// 开头")
	}

	if parsed.Path == "" {
		parsed.Path = "/"
	}

	address := parsed.Host
	if !strings.Contains(address, ":") {
		if parsed.Scheme == "wss" {
			address += ":443"
		} else {
			address += ":80"
		}
	}

	dialer := net.Dialer{Timeout: 10 * time.Second}
	conn, err := dialer.DialContext(context.Background(), "tcp", address)
	if err != nil {
		return nil, nil, err
	}

	if parsed.Scheme == "wss" {
		_ = conn.Close()
		return nil, nil, fmt.Errorf("暂不支持 wss，请在本地 NapCat 使用 ws://")
	}

	keyBytes := make([]byte, 16)
	if _, err := rand.Read(keyBytes); err != nil {
		_ = conn.Close()
		return nil, nil, err
	}
	secKey := base64.StdEncoding.EncodeToString(keyBytes)

	path := parsed.RequestURI()
	if path == "" {
		path = "/"
	}
	headers := []string{
		fmt.Sprintf("GET %s HTTP/1.1", path),
		"Host: " + parsed.Host,
		"Upgrade: websocket",
		"Connection: Upgrade",
		"Sec-WebSocket-Key: " + secKey,
		"Sec-WebSocket-Version: 13",
	}
	if accessToken != "" {
		headers = append(headers, "Authorization: Bearer "+accessToken)
	}
	headers = append(headers, "\r\n")
	if _, err := conn.Write([]byte(strings.Join(headers, "\r\n"))); err != nil {
		_ = conn.Close()
		return nil, nil, err
	}

	reader := bufio.NewReader(conn)
	status, err := reader.ReadString('\n')
	if err != nil {
		_ = conn.Close()
		return nil, nil, err
	}
	if !strings.Contains(status, " 101 ") {
		_ = conn.Close()
		return nil, nil, fmt.Errorf("WebSocket 握手失败: %s", strings.TrimSpace(status))
	}
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			_ = conn.Close()
			return nil, nil, err
		}
		if strings.TrimSpace(line) == "" {
			break
		}
	}

	return conn, reader, nil
}

func (a *QQAdapter) writeWebSocketFrame(opcode byte, payload []byte) error {
	a.writeMu.Lock()
	defer a.writeMu.Unlock()

	a.mu.Lock()
	conn := a.conn
	a.mu.Unlock()
	if conn == nil {
		return fmt.Errorf("WebSocket 未连接")
	}

	frame := bytes.Buffer{}
	frame.WriteByte(0x80 | opcode)
	length := len(payload)
	switch {
	case length < 126:
		frame.WriteByte(0x80 | byte(length))
	case length <= math.MaxUint16:
		frame.WriteByte(0x80 | 126)
		_ = binary.Write(&frame, binary.BigEndian, uint16(length))
	default:
		frame.WriteByte(0x80 | 127)
		_ = binary.Write(&frame, binary.BigEndian, uint64(length))
	}

	maskKey := make([]byte, 4)
	if _, err := rand.Read(maskKey); err != nil {
		return err
	}
	frame.Write(maskKey)
	masked := make([]byte, len(payload))
	for index, value := range payload {
		masked[index] = value ^ maskKey[index%4]
	}
	frame.Write(masked)

	_, err := conn.Write(frame.Bytes())
	return err
}

func readWebSocketFrame(reader *bufio.Reader) (byte, []byte, error) {
	first, err := reader.ReadByte()
	if err != nil {
		return 0, nil, err
	}
	second, err := reader.ReadByte()
	if err != nil {
		return 0, nil, err
	}
	opcode := first & 0x0f
	masked := second&0x80 != 0
	length := uint64(second & 0x7f)
	switch length {
	case 126:
		var value uint16
		if err := binary.Read(reader, binary.BigEndian, &value); err != nil {
			return 0, nil, err
		}
		length = uint64(value)
	case 127:
		if err := binary.Read(reader, binary.BigEndian, &length); err != nil {
			return 0, nil, err
		}
	}

	var maskKey []byte
	if masked {
		maskKey = make([]byte, 4)
		if _, err := io.ReadFull(reader, maskKey); err != nil {
			return 0, nil, err
		}
	}

	payload := make([]byte, length)
	if _, err := io.ReadFull(reader, payload); err != nil {
		return 0, nil, err
	}
	if masked {
		for index := range payload {
			payload[index] ^= maskKey[index%4]
		}
	}
	return opcode, payload, nil
}

func oneBotAPIURL(serverURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(serverURL))
	if err != nil {
		return ""
	}
	if parsed.Scheme == "ws" {
		parsed.Scheme = "http"
	} else if parsed.Scheme == "wss" {
		parsed.Scheme = "https"
	} else {
		return ""
	}
	parsed.Path = "/"
	parsed.RawQuery = ""
	return strings.TrimRight(parsed.String(), "/")
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
