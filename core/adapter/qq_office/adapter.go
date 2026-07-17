package qq_office

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
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
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/allbot/allbot/core/adapter/_contract"
	"github.com/allbot/allbot/core/types"
	"github.com/allbot/allbot/core/utils"
)

type UserInfo = contract.UserInfo
type GroupInfo = contract.GroupInfo

const (
	qqOfficePlatform            = "qq_office"
	qqOfficeDefaultAPIBaseURL   = "https://api.sgroup.qq.com"
	qqOfficeDefaultTokenURL     = "https://bots.qq.com/app/getAppAccessToken"
	qqOfficeIntentDirectMessage = 1 << 12
	qqOfficeIntentGroupMember   = 1 << 24
	qqOfficeIntentGroupAndC2C   = 1 << 25
	qqOfficeIntentInteraction   = 1 << 26
	qqOfficeTokenRefreshBefore  = 60 * time.Second
	qqOfficeReplySeqTTL         = 10 * time.Minute
)

type QQOfficeAdapter struct {
	appID        string
	clientSecret string
	apiBaseURL   string
	tokenURL     string
	httpClient   *http.Client

	messageHandler func(*types.Message)

	accessToken    string
	tokenExpiresAt time.Time
	tokenMu        sync.Mutex

	conn    net.Conn
	reader  *bufio.Reader
	connMu  sync.Mutex
	writeMu sync.Mutex

	stopChan chan struct{}
	stopOnce sync.Once
	lastSeq  int64

	replySeqMu sync.Mutex
	replySeqs  map[string]qqOfficeReplySeq
}

type qqOfficeReplySeq struct {
	seq       int
	updatedAt time.Time
}

type qqOfficeMedia struct {
	FileUUID string `json:"file_uuid,omitempty"`
	FileInfo string `json:"file_info,omitempty"`
	TTL      int    `json:"ttl,omitempty"`
}

type qqOfficeGatewayPayload struct {
	Op int             `json:"op"`
	D  json.RawMessage `json:"d"`
	S  int64           `json:"s,omitempty"`
	T  string          `json:"t,omitempty"`
}

// NewQQOfficeAdapter 创建 QQ 官方机器人适配器。
func NewQQOfficeAdapter(appID, clientSecret, apiBaseURL, tokenURL string) *QQOfficeAdapter {
	apiBaseURL = strings.TrimSpace(apiBaseURL)
	tokenURL = strings.TrimSpace(tokenURL)
	if apiBaseURL == "" {
		apiBaseURL = qqOfficeDefaultAPIBaseURL
	}
	if tokenURL == "" {
		tokenURL = qqOfficeDefaultTokenURL
	}
	return &QQOfficeAdapter{
		appID:        strings.TrimSpace(appID),
		clientSecret: strings.TrimSpace(clientSecret),
		apiBaseURL:   strings.TrimRight(apiBaseURL, "/"),
		tokenURL:     tokenURL,
		httpClient:   &http.Client{Timeout: 15 * time.Second},
		stopChan:     make(chan struct{}),
		replySeqs:    make(map[string]qqOfficeReplySeq),
	}
}

func (a *QQOfficeAdapter) GetPlatform() string {
	return qqOfficePlatform
}

func (a *QQOfficeAdapter) GetBotIdentity(msg *types.Message) contract.BotIdentity {
	return contract.BotIdentity{Label: "机器人 App ID", Value: strings.TrimSpace(a.appID)}
}

func (a *QQOfficeAdapter) SetMessageHandler(handler func(*types.Message)) {
	a.messageHandler = handler
}

// ReplyTarget 按 QQ 官方 DMS/C2C/群聊 API 目标格式解析回复目标。
func (a *QQOfficeAdapter) ReplyTarget(msg *types.Message) string {
	if msg == nil {
		return ""
	}
	if msg.Metadata != nil {
		if replyTarget := strings.TrimSpace(msg.Metadata["reply_target"]); replyTarget != "" {
			if strings.HasPrefix(replyTarget, "group_") && !strings.Contains(replyTarget, "|at_") {
				if memberOpenID := strings.TrimSpace(msg.Metadata["qq_office_member_openid"]); memberOpenID != "" {
					return replyTarget + "|at_" + memberOpenID
				}
			}
			return replyTarget
		}
		if groupOpenID := strings.TrimSpace(msg.Metadata["qq_office_group_openid"]); groupOpenID != "" {
			if memberOpenID := strings.TrimSpace(msg.Metadata["qq_office_member_openid"]); memberOpenID != "" {
				return "group_" + groupOpenID + "|at_" + memberOpenID
			}
			return "group_" + groupOpenID
		}
		if userOpenID := strings.TrimSpace(msg.Metadata["qq_office_user_openid"]); userOpenID != "" {
			return "user_" + userOpenID
		}
		if guildID := strings.TrimSpace(msg.Metadata["qq_office_guild_id"]); guildID != "" {
			return "dms_" + guildID
		}
	}
	if msg.GroupID != "" {
		if hasQQOfficeTargetPrefix(msg.GroupID) {
			return msg.GroupID
		}
		return "group_" + msg.GroupID
	}
	if hasQQOfficeTargetPrefix(msg.UserID) {
		return msg.UserID
	}
	return "user_" + msg.UserID
}

// FormatReplyText 保持 QQ 官方原文回复，不拼接 CQ 码或文本 @。
func (a *QQOfficeAdapter) FormatReplyText(msg *types.Message, text string) string {
	return text
}

// SendTarget 按 QQ 官方 DMS/C2C/群聊 API 目标格式解析插件主动发送目标。
func (a *QQOfficeAdapter) SendTarget(userID string, groupID string) string {
	if groupID != "" {
		if hasQQOfficeTargetPrefix(groupID) {
			return groupID
		}
		return "dms_" + groupID
	}
	if hasQQOfficeTargetPrefix(userID) {
		return userID
	}
	return "user_" + userID
}

func hasQQOfficeTargetPrefix(value string) bool {
	return strings.HasPrefix(value, "user_") || strings.HasPrefix(value, "group_") || strings.HasPrefix(value, "dms_")
}

func (a *QQOfficeAdapter) Start() error {
	if a.appID == "" {
		return fmt.Errorf("App ID 不能为空")
	}
	if a.clientSecret == "" {
		return fmt.Errorf("Client Secret 不能为空")
	}
	if _, err := a.getAccessToken(); err != nil {
		return fmt.Errorf("获取 QQ 官方 access token 失败: %w", err)
	}
	go a.gatewayLoop()
	log.Printf("QQ 官方机器人 Adapter 已启动")
	return nil
}

func (a *QQOfficeAdapter) Stop() error {
	a.stopOnce.Do(func() {
		close(a.stopChan)
	})
	a.closeCurrentConn()
	log.Printf("QQ 官方机器人 Adapter 已停止")
	return nil
}

func (a *QQOfficeAdapter) SendMessage(target string, text string) error {
	targetInfo, err := parseQQOfficeMessageTarget(target)
	if err != nil {
		return err
	}
	body := map[string]interface{}{"content": text}
	if targetInfo.msgID != "" {
		body["msg_id"] = targetInfo.msgID
	}
	path := ""
	switch targetInfo.kind {
	case "dms":
		path = "/dms/" + url.PathEscape(targetInfo.id) + "/messages"
	case "user":
		body["msg_type"] = 0
		if targetInfo.msgID != "" {
			body["msg_seq"] = a.nextReplySeq(targetInfo)
		}
		path = "/v2/users/" + url.PathEscape(targetInfo.id) + "/messages"
	case "group":
		body["msg_type"] = 0
		if targetInfo.msgID != "" {
			body["msg_seq"] = a.nextReplySeq(targetInfo)
		}
		path = "/v2/groups/" + url.PathEscape(targetInfo.id) + "/messages"
	default:
		return fmt.Errorf("QQ 官方消息目标类型无效: %s", targetInfo.kind)
	}
	log.Printf("[发送][QQ官方][%s]：%s", target, text)
	return a.callAPIWithPassiveFallback(path, body, targetInfo)
}

func (a *QQOfficeAdapter) SendButtons(target string, text string, buttons [][]types.ButtonOption) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return fmt.Errorf("QQ 官方按钮消息内容不能为空")
	}
	targetInfo, err := parseQQOfficeMessageTarget(target)
	if err != nil {
		return err
	}
	if targetInfo.kind == "dms" {
		return a.SendMessage(target, formatButtonsFallbackText(text, buttons))
	}
	keyboard := qqOfficeButtonsToKeyboard(buttons)
	if len(keyboard) == 0 {
		return a.SendMessage(target, text)
	}
	body := map[string]interface{}{
		"msg_type": 2,
		"markdown": map[string]interface{}{"content": qqOfficeGroupAtContent(targetInfo, text)},
		"keyboard": map[string]interface{}{"content": map[string]interface{}{"rows": keyboard}},
	}
	if targetInfo.msgID != "" {
		body["msg_id"] = targetInfo.msgID
		body["msg_seq"] = a.nextReplySeq(targetInfo)
	}
	path := ""
	switch targetInfo.kind {
	case "user":
		path = "/v2/users/" + url.PathEscape(targetInfo.id) + "/messages"
	case "group":
		path = "/v2/groups/" + url.PathEscape(targetInfo.id) + "/messages"
	default:
		return fmt.Errorf("QQ 官方按钮目标类型无效: %s", targetInfo.kind)
	}
	log.Printf("[发送][QQ官方][%s]：[Buttons] %s", target, text)
	return a.callAPIWithPassiveFallback(path, body, targetInfo)
}

func qqOfficeButtonsToKeyboard(buttons [][]types.ButtonOption) []map[string]interface{} {
	rows := make([]map[string]interface{}, 0, len(buttons))
	buttonID := 1
	for _, row := range buttons {
		items := make([]map[string]interface{}, 0, len(row))
		for _, item := range row {
			text := strings.TrimSpace(item.Text)
			value := strings.TrimSpace(item.Value)
			if text == "" || value == "" {
				continue
			}
			permission := map[string]interface{}{"type": 2}
			if userID := strings.TrimSpace(item.UserID); userID != "" {
				permission = map[string]interface{}{"type": 0, "specify_user_ids": []string{userID}}
			}
			items = append(items, map[string]interface{}{
				"id": strconv.Itoa(buttonID),
				"render_data": map[string]interface{}{
					"label":         text,
					"visited_label": text,
				},
				"action": map[string]interface{}{
					"type":                     1,
					"permission":               permission,
					"click_limit":              1,
					"unsupport_tips":           value,
					"data":                     value,
					"at_bot_show_channel_list": true,
				},
			})
			buttonID++
		}
		if len(items) > 0 {
			rows = append(rows, map[string]interface{}{"buttons": items})
		}
	}
	return rows
}

func formatButtonsFallbackText(text string, buttons [][]types.ButtonOption) string {
	lines := []string{strings.TrimSpace(text)}
	for _, row := range buttons {
		parts := make([]string, 0, len(row))
		for _, item := range row {
			label := strings.TrimSpace(item.Text)
			value := strings.TrimSpace(item.Value)
			if label == "" || value == "" {
				continue
			}
			parts = append(parts, label)
		}
		if len(parts) > 0 {
			lines = append(lines, strings.Join(parts, "  "))
		}
	}
	return strings.Join(lines, "\n")
}

func (a *QQOfficeAdapter) SendMarkdown(target string, markdown string) error {
	markdown = strings.TrimSpace(markdown)
	if markdown == "" {
		return fmt.Errorf("QQ 官方 Markdown 内容不能为空")
	}
	targetInfo, err := parseQQOfficeMessageTarget(target)
	if err != nil {
		return err
	}
	if targetInfo.kind == "dms" {
		return a.SendMessage(target, utils.MarkdownToPlainText(markdown))
	}
	body := map[string]interface{}{
		"msg_type": 2,
		"markdown": map[string]interface{}{"content": qqOfficeGroupAtContent(targetInfo, markdown)},
	}
	if targetInfo.msgID != "" {
		body["msg_id"] = targetInfo.msgID
		body["msg_seq"] = a.nextReplySeq(targetInfo)
	}
	path := ""
	switch targetInfo.kind {
	case "user":
		path = "/v2/users/" + url.PathEscape(targetInfo.id) + "/messages"
	case "group":
		path = "/v2/groups/" + url.PathEscape(targetInfo.id) + "/messages"
	default:
		return fmt.Errorf("QQ 官方 Markdown 目标类型无效: %s", targetInfo.kind)
	}
	log.Printf("[发送][QQ官方][%s]：[Markdown] %s", target, markdown)
	return a.callAPIWithPassiveFallback(path, body, targetInfo)
}

func (a *QQOfficeAdapter) nextReplySeq(target qqOfficeMessageTarget) int {
	key := strings.Join([]string{target.kind, target.id, target.msgID}, "\x1f")
	now := time.Now()
	a.replySeqMu.Lock()
	defer a.replySeqMu.Unlock()
	if a.replySeqs == nil {
		a.replySeqs = make(map[string]qqOfficeReplySeq)
	}
	for itemKey, item := range a.replySeqs {
		if now.Sub(item.updatedAt) > qqOfficeReplySeqTTL {
			delete(a.replySeqs, itemKey)
		}
	}
	item := a.replySeqs[key]
	seq := item.seq + 1
	a.replySeqs[key] = qqOfficeReplySeq{seq: seq, updatedAt: now}
	return seq
}

func (a *QQOfficeAdapter) SendImage(target string, imageURL string) error {
	targetInfo, err := parseQQOfficeMessageTarget(target)
	if err != nil {
		return err
	}
	imageURL = strings.TrimSpace(imageURL)
	if imageURL == "" {
		return fmt.Errorf("QQ 官方图片地址不能为空")
	}
	if targetInfo.kind == "dms" {
		return fmt.Errorf("QQ 官方 DMS 图片发送暂未实现")
	}
	media, err := a.uploadImageMedia(targetInfo, imageURL)
	if err != nil {
		return err
	}
	body := map[string]interface{}{
		"msg_type": 7,
		"media":    media,
	}
	if targetInfo.msgID != "" {
		body["msg_id"] = targetInfo.msgID
		body["msg_seq"] = a.nextReplySeq(targetInfo)
	}
	path := ""
	switch targetInfo.kind {
	case "user":
		path = "/v2/users/" + url.PathEscape(targetInfo.id) + "/messages"
	case "group":
		path = "/v2/groups/" + url.PathEscape(targetInfo.id) + "/messages"
	default:
		return fmt.Errorf("QQ 官方图片目标类型无效: %s", targetInfo.kind)
	}
	log.Printf("[发送][QQ官方][%s]：[图片] %s", target, imageURL)
	return a.callAPIWithPassiveFallback(path, body, targetInfo)
}

func (a *QQOfficeAdapter) uploadImageMedia(target qqOfficeMessageTarget, imageURL string) (qqOfficeMedia, error) {
	body := map[string]interface{}{
		"file_type": 1,
		"url":       imageURL,
	}
	if target.msgID != "" {
		body["srv_send_msg"] = false
	}
	path := ""
	switch target.kind {
	case "user":
		path = "/v2/users/" + url.PathEscape(target.id) + "/files"
	case "group":
		path = "/v2/groups/" + url.PathEscape(target.id) + "/files"
	default:
		return qqOfficeMedia{}, fmt.Errorf("QQ 官方图片上传目标类型无效: %s", target.kind)
	}
	var media qqOfficeMedia
	if err := a.callAPI(http.MethodPost, path, body, &media); err != nil {
		return qqOfficeMedia{}, err
	}
	if strings.TrimSpace(media.FileInfo) == "" {
		return qqOfficeMedia{}, fmt.Errorf("QQ 官方图片上传响应缺少 file_info")
	}
	return media, nil
}

func (a *QQOfficeAdapter) SendFile(target string, filePath string) error {
	return fmt.Errorf("QQ 官方机器人文件发送暂未实现")
}

func (a *QQOfficeAdapter) ackInteraction(interactionID string) error {
	interactionID = strings.TrimSpace(interactionID)
	if interactionID == "" {
		return nil
	}
	return a.callAPI(http.MethodPut, "/interactions/"+url.PathEscape(interactionID), map[string]interface{}{"code": 0}, nil)
}

func (a *QQOfficeAdapter) GetUserInfo(userID string) (*UserInfo, error) {
	return &UserInfo{UserID: userID, Nickname: userID, Extra: map[string]string{"platform": qqOfficePlatform}}, nil
}

func (a *QQOfficeAdapter) GetGroupInfo(groupID string) (*GroupInfo, error) {
	return nil, fmt.Errorf("QQ 官方机器人群组信息暂未实现")
}

func (a *QQOfficeAdapter) AtUser(groupID string, userID string) error {
	return fmt.Errorf("QQ 官方机器人 @ 用户暂未实现")
}

func (a *QQOfficeAdapter) getAccessToken() (string, error) {
	a.tokenMu.Lock()
	defer a.tokenMu.Unlock()

	if a.accessToken != "" && time.Now().Before(a.tokenExpiresAt.Add(-qqOfficeTokenRefreshBefore)) {
		return a.accessToken, nil
	}

	body, err := json.Marshal(map[string]string{
		"appId":        a.appID,
		"clientSecret": a.clientSecret,
	})
	if err != nil {
		return "", err
	}
	request, err := http.NewRequest(http.MethodPost, a.tokenURL, bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", "application/json")

	resp, err := a.httpClient.Do(request)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("状态码 %d", resp.StatusCode)
	}

	var result map[string]interface{}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	if err := decoder.Decode(&result); err != nil {
		return "", err
	}
	accessToken := stringValue(result["access_token"])
	if accessToken == "" {
		return "", fmt.Errorf("响应缺少 access_token: %s", qqOfficeTokenErrorSummary(result))
	}
	expiresIn := numberValue(result["expires_in"])
	if expiresIn <= 0 {
		return "", fmt.Errorf("响应缺少有效的 expires_in: %s", qqOfficeTokenErrorSummary(result))
	}

	a.accessToken = accessToken
	a.tokenExpiresAt = time.Now().Add(time.Duration(expiresIn) * time.Second)
	return a.accessToken, nil
}

func (a *QQOfficeAdapter) callAPIWithPassiveFallback(path string, body map[string]interface{}, target qqOfficeMessageTarget) error {
	err := a.callAPI(http.MethodPost, path, body, nil)
	if err == nil || target.msgID == "" || target.kind == "dms" || !isQQOfficePassiveReplyLimitError(err) {
		return err
	}
	delete(body, "msg_id")
	delete(body, "msg_seq")
	log.Printf("[WARN][QQ官方] 被动回复超过限制，改用主动消息重发: target=%s_%s err=%v", target.kind, target.id, err)
	return a.callAPI(http.MethodPost, path, body, nil)
}

func isQQOfficePassiveReplyLimitError(err error) bool {
	if err == nil {
		return false
	}
	text := err.Error()
	return strings.Contains(text, "40034128") || strings.Contains(text, "被动回复时间或者次数超过限制")
}

func (a *QQOfficeAdapter) callAPI(method, path string, body interface{}, result interface{}) error {
	token, err := a.getAccessToken()
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
	request.Header.Set("Authorization", "QQBot "+token)
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
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("QQ 官方 API %s %s 状态码 %d: %s", method, path, resp.StatusCode, string(payload))
	}
	if result == nil || len(payload) == 0 {
		return nil
	}
	return json.Unmarshal(payload, result)
}

func (a *QQOfficeAdapter) gatewayLoop() {
	failureCount := 0
	for {
		select {
		case <-a.stopChan:
			return
		default:
		}

		if err := a.connectAndReadGateway(); err != nil {
			select {
			case <-a.stopChan:
				return
			default:
			}
			failureCount++
			if failureCount == 1 {
				log.Printf("[WARN][QQ官方] Gateway 异常: %v", err)
			} else if failureCount%10 == 0 {
				log.Printf("[WARN][QQ官方] Gateway 持续异常 %d 次，最近一次: %v", failureCount, err)
			}
			if !a.waitBeforeRetry(qqOfficeRetryDelay(failureCount)) {
				return
			}
			continue
		}

		if failureCount > 0 {
			log.Printf("[INFO][QQ官方] Gateway 连接已恢复，之前连续异常 %d 次", failureCount)
			failureCount = 0
		}
	}
}

func (a *QQOfficeAdapter) connectAndReadGateway() error {
	var gateway struct {
		URL string `json:"url"`
	}
	if err := a.callAPI(http.MethodGet, "/gateway", nil, &gateway); err != nil {
		return fmt.Errorf("获取 Gateway 地址失败: %w", err)
	}
	if strings.TrimSpace(gateway.URL) == "" {
		return fmt.Errorf("Gateway 地址为空")
	}

	conn, reader, err := dialQQOfficeGatewayWebSocket(gateway.URL)
	if err != nil {
		return fmt.Errorf("连接 Gateway 失败: %w", err)
	}
	a.setCurrentConn(conn, reader)
	defer a.closeCurrentConn()

	hello, err := a.readGatewayPayload(reader)
	if err != nil {
		return fmt.Errorf("读取 Hello 失败: %w", err)
	}
	if hello.Op != 10 {
		return fmt.Errorf("首个 Gateway 包不是 Hello: op=%d", hello.Op)
	}
	heartbeatInterval := qqOfficeHeartbeatInterval(hello.D)
	if heartbeatInterval <= 0 {
		heartbeatInterval = 45 * time.Second
	}
	if err := a.sendIdentify(); err != nil {
		return fmt.Errorf("发送 Identify 失败: %w", err)
	}
	log.Printf("[INFO][QQ官方] Gateway 已连接，已订阅 DMS/C2C/群聊/群成员/群事件")

	heartbeatDone := make(chan struct{})
	defer close(heartbeatDone)
	go a.heartbeatLoop(heartbeatInterval, heartbeatDone)
	return a.readGatewayLoop(reader)
}

func (a *QQOfficeAdapter) sendIdentify() error {
	token, err := a.getAccessToken()
	if err != nil {
		return err
	}
	return a.sendGatewayPayload(2, map[string]interface{}{
		"token":   "QQBot " + token,
		"intents": qqOfficeIntentDirectMessage | qqOfficeIntentGroupMember | qqOfficeIntentGroupAndC2C | qqOfficeIntentInteraction,
		"shard":   []int{0, 1},
		"properties": map[string]string{
			"$os":      runtime.GOOS,
			"$browser": "allbot",
			"$device":  "allbot",
		},
	})
}

func (a *QQOfficeAdapter) heartbeatLoop(interval time.Duration, done <-chan struct{}) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-done:
			return
		case <-a.stopChan:
			return
		case <-ticker.C:
			if err := a.sendGatewayPayload(1, atomic.LoadInt64(&a.lastSeq)); err != nil {
				log.Printf("[WARN][QQ官方] Heartbeat 发送失败: %v", err)
				return
			}
		}
	}
}

func (a *QQOfficeAdapter) readGatewayLoop(reader *bufio.Reader) error {
	for {
		payload, err := a.readGatewayPayload(reader)
		if err != nil {
			return err
		}
		if payload.S > 0 {
			atomic.StoreInt64(&a.lastSeq, payload.S)
		}
		switch payload.Op {
		case 0:
			var data map[string]interface{}
			if len(payload.D) > 0 {
				if err := json.Unmarshal(payload.D, &data); err != nil {
					log.Printf("[WARN][QQ官方] Dispatch 解析失败: %v", err)
					continue
				}
			}
			a.handleDispatch(payload.T, data)
		case 7:
			log.Printf("[WARN][QQ官方] Gateway 要求重连")
			return fmt.Errorf("Gateway 要求重连")
		case 9:
			log.Printf("[WARN][QQ官方] Gateway 会话无效: %s", qqOfficePayloadSummary(payload.D))
			return fmt.Errorf("Gateway 会话无效")
		case 10, 11:
		default:
			log.Printf("[INFO][QQ官方] Gateway 忽略 op=%d", payload.Op)
		}
	}
}

func (a *QQOfficeAdapter) readGatewayPayload(reader *bufio.Reader) (qqOfficeGatewayPayload, error) {
	for {
		messageType, payload, err := readWebSocketFrame(reader)
		if err != nil {
			return qqOfficeGatewayPayload{}, err
		}
		switch messageType {
		case 1, 2:
			var gatewayPayload qqOfficeGatewayPayload
			if err := json.Unmarshal(payload, &gatewayPayload); err != nil {
				return qqOfficeGatewayPayload{}, err
			}
			return gatewayPayload, nil
		case 8:
			return qqOfficeGatewayPayload{}, fmt.Errorf("WebSocket 已关闭")
		case 9:
			_ = a.writeWebSocketFrame(10, payload)
		}
	}
}

func (a *QQOfficeAdapter) handleDispatch(eventType string, data map[string]interface{}) {
	switch eventType {
	case "READY":
		log.Printf("[INFO][QQ官方] Gateway READY: session_id=%s", stringValue(data["session_id"]))
	case "DIRECT_MESSAGE_CREATE":
		a.handleDirectMessage(data)
	case "C2C_MESSAGE_CREATE":
		a.handleC2CMessage(data)
	case "GROUP_AT_MESSAGE_CREATE", "GROUP_MESSAGE_CREATE":
		a.handleGroupMessage(eventType, data)
	case "GROUP_MEMBER_ADD", "GROUP_MEMBER_REMOVE", "GROUP_ADD_ROBOT", "GROUP_DEL_ROBOT", "GROUP_MSG_RECEIVE", "GROUP_MSG_REJECT":
		a.handleQQOfficeEvent(eventType, data)
	case "INTERACTION_CREATE":
		a.handleInteractionCreate(data)
	default:
		if eventType != "" {
			log.Printf("[INFO][QQ官方] Gateway 收到未处理事件: %s", eventType)
		}
	}
}

func (a *QQOfficeAdapter) handleDirectMessage(data map[string]interface{}) {
	content := strings.TrimSpace(stringValue(data["content"]))
	if content == "" {
		log.Printf("[INFO][QQ官方] 忽略空 DMS 消息: id=%s guild=%s", stringValue(data["id"]), stringValue(data["guild_id"]))
		return
	}
	messageID := stringValue(data["id"])
	guildID := stringValue(data["guild_id"])
	channelID := stringValue(data["channel_id"])
	author, _ := data["author"].(map[string]interface{})
	userID := stringValue(author["id"])
	authorName := stringValue(author["username"])
	if authorName == "" {
		authorName = stringValue(author["nick"])
	}

	msg := &types.Message{
		ID:       messageID,
		Platform: qqOfficePlatform,
		UserID:   userID,
		Content:  content,
		Metadata: map[string]string{
			"message_type":          "dms",
			"qq_office_guild_id":    guildID,
			"qq_office_channel_id":  channelID,
			"qq_office_msg_id":      messageID,
			"qq_office_author_name": authorName,
			"reply_target":          "dms_" + guildID + "|msg_" + messageID,
		},
	}
	log.Printf("[接收][QQ官方][%s(DMS %s)]：%s", userID, guildID, content)
	a.dispatchMessage(msg)
}

func (a *QQOfficeAdapter) handleC2CMessage(data map[string]interface{}) {
	content := strings.TrimSpace(stringValue(data["content"]))
	if content == "" {
		log.Printf("[INFO][QQ官方] 忽略空 C2C 消息: id=%s", stringValue(data["id"]))
		return
	}
	messageID := stringValue(data["id"])
	author, _ := data["author"].(map[string]interface{})
	userOpenID := stringValue(author["user_openid"])
	if userOpenID == "" {
		userOpenID = stringValue(author["id"])
	}
	if userOpenID == "" {
		log.Printf("[WARN][QQ官方] 忽略缺少 user_openid 的 C2C 消息: id=%s", messageID)
		return
	}

	msg := &types.Message{
		ID:       messageID,
		Platform: qqOfficePlatform,
		UserID:   userOpenID,
		Content:  content,
		Metadata: map[string]string{
			"message_type":          "c2c",
			"qq_office_msg_id":      messageID,
			"qq_office_user_openid": userOpenID,
			"reply_target":          "user_" + userOpenID + "|msg_" + messageID,
		},
	}
	log.Printf("[接收][QQ官方][%s(C2C)]：%s", userOpenID, content)
	a.dispatchMessage(msg)
}

func (a *QQOfficeAdapter) handleGroupAtMessage(data map[string]interface{}) {
	a.handleGroupMessage("GROUP_AT_MESSAGE_CREATE", data)
}

func (a *QQOfficeAdapter) handleGroupMessage(eventType string, data map[string]interface{}) {
	content := strings.TrimSpace(stringValue(data["content"]))
	if content == "" {
		log.Printf("[INFO][QQ官方] 忽略空群聊消息: id=%s group=%s", stringValue(data["id"]), stringValue(data["group_openid"]))
		return
	}
	messageID := stringValue(data["id"])
	groupOpenID := stringValue(data["group_openid"])
	if groupOpenID == "" {
		log.Printf("[WARN][QQ官方] 忽略缺少 group_openid 的群聊消息: id=%s", messageID)
		return
	}
	author, _ := data["author"].(map[string]interface{})
	memberOpenID := stringValue(author["member_openid"])
	if memberOpenID == "" {
		memberOpenID = stringValue(author["user_openid"])
	}
	if memberOpenID == "" {
		memberOpenID = stringValue(author["id"])
	}
	if memberOpenID == "" {
		log.Printf("[WARN][QQ官方] 忽略缺少 member_openid 的群聊消息: id=%s group=%s", messageID, groupOpenID)
		return
	}

	msg := &types.Message{
		ID:       messageID,
		Platform: qqOfficePlatform,
		UserID:   memberOpenID,
		GroupID:  groupOpenID,
		Content:  content,
		Metadata: map[string]string{
			"message_type":            "group",
			"qq_office_event_type":    eventType,
			"qq_office_message_event": eventType,
			"qq_office_msg_id":        messageID,
			"qq_office_group_openid":  groupOpenID,
			"qq_office_member_openid": memberOpenID,
			"reply_target":            "group_" + groupOpenID + "|msg_" + messageID + "|at_" + memberOpenID,
		},
	}
	log.Printf("[接收][QQ官方][%s(群 %s)]：%s", memberOpenID, groupOpenID, content)
	a.dispatchMessage(msg)
}

func (a *QQOfficeAdapter) handleInteractionCreate(data map[string]interface{}) {
	eventID := stringValue(data["id"])
	if err := a.ackInteraction(eventID); err != nil {
		log.Printf("[WARN][QQ官方] 按钮交互 ACK 失败: id=%s err=%v", eventID, err)
	}
	content := qqOfficeInteractionData(data)
	if strings.TrimSpace(content) == "" {
		log.Printf("[INFO][QQ官方] 忽略空按钮交互: id=%s summary=%s", eventID, qqOfficeMapSummary(data))
		return
	}
	groupOpenID := stringValue(data["group_openid"])
	if groupOpenID == "" {
		groupOpenID = stringValue(data["group_id"])
	}
	userID := qqOfficeInteractionUserID(data)
	if userID == "" {
		log.Printf("[WARN][QQ官方] 忽略缺少用户标识的按钮交互: id=%s summary=%s", eventID, qqOfficeMapSummary(data))
		return
	}
	metadata := map[string]string{
		"message_type":               "button",
		"qq_office_event_type":       "INTERACTION_CREATE",
		"qq_office_interaction_id":   eventID,
		"qq_office_interaction_data": content,
		"reply_target":               "user_" + userID + "|msg_" + eventID,
	}
	if groupOpenID != "" {
		metadata["qq_office_group_openid"] = groupOpenID
		metadata["qq_office_member_openid"] = userID
		metadata["reply_target"] = "group_" + groupOpenID + "|msg_" + eventID + "|at_" + userID
	}
	msg := &types.Message{
		ID:       eventID,
		Platform: qqOfficePlatform,
		UserID:   userID,
		GroupID:  groupOpenID,
		Content:  strings.TrimSpace(content),
		Metadata: metadata,
	}
	log.Printf("[交互][QQ官方][%s(按钮 %s)]：%s", userID, groupOpenID, msg.Content)
	a.dispatchMessage(msg)
}

func qqOfficeInteractionUserID(data map[string]interface{}) string {
	for _, key := range []string{"user_openid", "member_openid", "group_member_openid", "op_member_openid", "user_id", "member_id", "openid"} {
		if value := strings.TrimSpace(stringValue(data[key])); value != "" {
			return value
		}
	}
	for _, source := range qqOfficeInteractionNestedMaps(data) {
		for _, key := range []string{"user_openid", "member_openid", "group_member_openid", "op_member_openid", "user_id", "member_id", "openid", "id"} {
			if value := strings.TrimSpace(stringValue(source[key])); value != "" {
				return value
			}
		}
	}
	return ""
}

func qqOfficeInteractionNestedMaps(data map[string]interface{}) []map[string]interface{} {
	maps := make([]map[string]interface{}, 0, 6)
	for _, key := range []string{"user", "member", "author", "operator"} {
		if value, ok := data[key].(map[string]interface{}); ok {
			maps = append(maps, value)
		}
	}
	if dataMap, ok := data["data"].(map[string]interface{}); ok {
		for _, key := range []string{"resolved", "user", "member"} {
			if value, ok := dataMap[key].(map[string]interface{}); ok {
				maps = append(maps, value)
			}
		}
	}
	if resolved, ok := data["resolved"].(map[string]interface{}); ok {
		maps = append(maps, resolved)
	}
	return maps
}

func qqOfficeInteractionData(data map[string]interface{}) string {
	if value, ok := data["data"].(string); ok && strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	if dataMap, ok := data["data"].(map[string]interface{}); ok {
		for _, key := range []string{"data", "value", "button_data"} {
			if value := strings.TrimSpace(stringValue(dataMap[key])); value != "" {
				return value
			}
		}
		if resolved, ok := dataMap["resolved"].(map[string]interface{}); ok {
			for _, key := range []string{"button_data", "data", "value"} {
				if value := strings.TrimSpace(stringValue(resolved[key])); value != "" {
					return value
				}
			}
		}
	}
	if resolved, ok := data["resolved"].(map[string]interface{}); ok {
		for _, key := range []string{"button_data", "data", "value"} {
			if value := strings.TrimSpace(stringValue(resolved[key])); value != "" {
				return value
			}
		}
	}
	return ""
}

func (a *QQOfficeAdapter) handleQQOfficeEvent(eventType string, data map[string]interface{}) {
	groupOpenID := stringValue(data["group_openid"])
	memberOpenID := stringValue(data["member_openid"])
	opMemberOpenID := stringValue(data["op_member_openid"])
	userID := memberOpenID
	if userID == "" {
		userID = opMemberOpenID
	}
	timestamp := stringValue(data["timestamp"])
	if timestamp == "" {
		timestamp = stringValue(data["event_ts"])
	}
	messageID := strings.Join([]string{eventType, groupOpenID, userID, timestamp}, ":")
	metadata := map[string]string{
		"message_type":           "event",
		"event_name":             eventType,
		"qq_office_event_type":   eventType,
		"qq_office_group_openid": groupOpenID,
		"qq_office_timestamp":    timestamp,
	}
	if memberOpenID != "" {
		metadata["qq_office_member_openid"] = memberOpenID
	}
	if opMemberOpenID != "" {
		metadata["qq_office_op_member_openid"] = opMemberOpenID
	}
	msg := &types.Message{
		ID:       messageID,
		Platform: qqOfficePlatform,
		UserID:   userID,
		GroupID:  groupOpenID,
		Content:  eventType,
		Metadata: metadata,
	}
	log.Printf("[事件][QQ官方][%s][群 %s][用户 %s]", eventType, groupOpenID, userID)
	a.dispatchMessage(msg)
}

func (a *QQOfficeAdapter) dispatchMessage(msg *types.Message) {
	if a.messageHandler != nil {
		a.messageHandler(msg)
	}
}

func (a *QQOfficeAdapter) sendGatewayPayload(op int, data interface{}) error {
	payload, err := json.Marshal(map[string]interface{}{"op": op, "d": data})
	if err != nil {
		return err
	}
	return a.writeWebSocketFrame(1, payload)
}

func (a *QQOfficeAdapter) writeWebSocketFrame(opcode byte, payload []byte) error {
	a.writeMu.Lock()
	defer a.writeMu.Unlock()

	a.connMu.Lock()
	conn := a.conn
	a.connMu.Unlock()
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

func (a *QQOfficeAdapter) setCurrentConn(conn net.Conn, reader *bufio.Reader) {
	a.connMu.Lock()
	defer a.connMu.Unlock()
	if a.conn != nil {
		_ = a.conn.Close()
	}
	a.conn = conn
	a.reader = reader
}

func (a *QQOfficeAdapter) closeCurrentConn() {
	a.connMu.Lock()
	defer a.connMu.Unlock()
	if a.conn != nil {
		_ = a.conn.Close()
		a.conn = nil
		a.reader = nil
	}
}

func (a *QQOfficeAdapter) waitBeforeRetry(delay time.Duration) bool {
	select {
	case <-a.stopChan:
		return false
	case <-time.After(delay):
		return true
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

func qqOfficePayloadSummary(payload json.RawMessage) string {
	if len(payload) == 0 || string(payload) == "null" {
		return "empty"
	}
	var result map[string]interface{}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	if err := decoder.Decode(&result); err != nil {
		text := strings.TrimSpace(string(payload))
		if len(text) > 120 {
			text = text[:120]
		}
		return "raw=" + text
	}
	return qqOfficeMapSummary(result)
}

func qqOfficeTokenErrorSummary(result map[string]interface{}) string {
	return qqOfficeMapSummary(result)
}

func qqOfficeMapSummary(result map[string]interface{}) string {
	parts := make([]string, 0, 4)
	for _, key := range []string{"code", "errcode", "message", "msg"} {
		if value := strings.TrimSpace(stringValue(result[key])); value != "" {
			parts = append(parts, key+"="+value)
		}
	}
	keys := make([]string, 0, len(result))
	for key := range result {
		if key == "access_token" || key == "clientSecret" || key == "client_secret" {
			continue
		}
		keys = append(keys, key)
	}
	if len(keys) > 0 {
		parts = append(parts, "fields="+strings.Join(keys, ","))
	}
	if len(parts) == 0 {
		return "响应为空或字段不符合官方格式"
	}
	return strings.Join(parts, "; ")
}

type qqOfficeMessageTarget struct {
	kind   string
	id     string
	msgID  string
	atUser string
}

func parseQQOfficeMessageTarget(target string) (qqOfficeMessageTarget, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return qqOfficeMessageTarget{}, fmt.Errorf("QQ 官方消息目标不能为空")
	}
	kind := "dms"
	if strings.HasPrefix(target, "user_") {
		kind = "user"
		target = strings.TrimPrefix(target, "user_")
	} else if strings.HasPrefix(target, "group_") {
		kind = "group"
		target = strings.TrimPrefix(target, "group_")
	} else if strings.HasPrefix(target, "dms_") {
		target = strings.TrimPrefix(target, "dms_")
	}

	parts := strings.Split(target, "|")
	id := strings.TrimSpace(parts[0])
	if id == "" {
		return qqOfficeMessageTarget{}, fmt.Errorf("QQ 官方%s不能为空", qqOfficeTargetIDName(kind))
	}
	messageID := ""
	atUser := ""
	for _, rawPart := range parts[1:] {
		part := strings.TrimSpace(rawPart)
		if part == "" {
			continue
		}
		switch {
		case strings.HasPrefix(part, "msg_"):
			messageID = strings.TrimSpace(strings.TrimPrefix(part, "msg_"))
			if messageID == "" {
				return qqOfficeMessageTarget{}, fmt.Errorf("QQ 官方 msg_id 不能为空")
			}
		case strings.HasPrefix(part, "at_"):
			atUser = strings.TrimSpace(strings.TrimPrefix(part, "at_"))
			if atUser == "" {
				return qqOfficeMessageTarget{}, fmt.Errorf("QQ 官方 at 用户不能为空")
			}
		default:
			return qqOfficeMessageTarget{}, fmt.Errorf("QQ 官方回复目标格式无效")
		}
	}
	return qqOfficeMessageTarget{kind: kind, id: id, msgID: messageID, atUser: atUser}, nil
}

func qqOfficeGroupAtContent(target qqOfficeMessageTarget, content string) string {
	if strings.TrimSpace(target.atUser) == "" {
		return content
	}
	return fmt.Sprintf("<@%s>\n%s", strings.TrimSpace(target.atUser), content)
}

func parseQQOfficeDMSTarget(target string) (string, string, error) {
	parsed, err := parseQQOfficeMessageTarget(target)
	if err != nil {
		return "", "", err
	}
	if parsed.kind == "user" {
		return "", "", fmt.Errorf("QQ 官方机器人 C2C 单聊暂未实现")
	}
	if parsed.kind == "group" {
		return "", "", fmt.Errorf("QQ 官方机器人群聊暂未实现")
	}
	return parsed.id, parsed.msgID, nil
}

func qqOfficeTargetIDName(kind string) string {
	switch kind {
	case "user":
		return " C2C user_openid "
	case "group":
		return "群聊 group_openid "
	default:
		return " DMS guild_id "
	}
}

func qqOfficeHeartbeatInterval(payload json.RawMessage) time.Duration {
	var data map[string]interface{}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	if err := decoder.Decode(&data); err != nil {
		return 0
	}
	interval := numberValue(data["heartbeat_interval"])
	if interval <= 0 {
		return 0
	}
	return time.Duration(interval) * time.Millisecond
}

func qqOfficeRetryDelay(failureCount int) time.Duration {
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

func dialQQOfficeGatewayWebSocket(rawURL string) (net.Conn, *bufio.Reader, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, nil, err
	}
	if parsed.Scheme != "ws" && parsed.Scheme != "wss" {
		return nil, nil, fmt.Errorf("Gateway 地址必须以 ws:// 或 wss:// 开头")
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
		tlsConn := tls.Client(conn, &tls.Config{ServerName: parsed.Hostname()})
		if err := tlsConn.Handshake(); err != nil {
			_ = conn.Close()
			return nil, nil, err
		}
		conn = tlsConn
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
		"\r\n",
	}
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
