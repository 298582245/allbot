package wechat_official

import (
	"bytes"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/allbot/allbot/core/adapter/_contract"
	"github.com/allbot/allbot/core/types"
)

type UserInfo = contract.UserInfo
type GroupInfo = contract.GroupInfo

const (
	platformName                   = "wechat_official"
	wechatOfficialDefaultPath      = "callback"
	wechatOfficialDefaultAPIBase   = "https://api.weixin.qq.com"
	wechatOfficialDefaultTokenURL  = "https://api.weixin.qq.com/cgi-bin/token"
	wechatOfficialTokenRefreshLead = 5 * time.Minute
)

var wechatOfficialPassiveReplyWait = 2 * time.Second

type WeChatOfficialAdapter struct {
	appID        string
	appSecret    string
	token        string
	callbackPath string
	apiBaseURL   string
	tokenURL     string
	httpClient   *http.Client

	messageHandler func(*types.Message)

	accessToken    string
	tokenExpiresAt time.Time
	tokenMu        sync.Mutex

	passiveReplyMu sync.Mutex
	passiveReplies map[string]chan wechatOfficialPassiveReply

	stopOnce sync.Once
	stopped  chan struct{}
}

type wechatOfficialMessageXML struct {
	XMLName      xml.Name `xml:"xml"`
	ToUserName   string   `xml:"ToUserName"`
	FromUserName string   `xml:"FromUserName"`
	CreateTime   string   `xml:"CreateTime"`
	MsgType      string   `xml:"MsgType"`
	Content      string   `xml:"Content"`
	MsgID        string   `xml:"MsgId"`
	Event        string   `xml:"Event"`
	EventKey     string   `xml:"EventKey"`
}

type wechatOfficialTokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int64  `json:"expires_in"`
	ErrCode     int    `json:"errcode"`
	ErrMsg      string `json:"errmsg"`
}

type wechatOfficialAPIResponse struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

type wechatOfficialPassiveReplyXML struct {
	XMLName      xml.Name `xml:"xml"`
	ToUserName   string   `xml:"ToUserName"`
	FromUserName string   `xml:"FromUserName"`
	CreateTime   int64    `xml:"CreateTime"`
	MsgType      string   `xml:"MsgType"`
	Content      string   `xml:"Content"`
}

type wechatOfficialPassiveReply struct {
	target string
	text   string
}

// NewWeChatOfficialAdapter 创建微信公众号适配器。
func NewWeChatOfficialAdapter(appID, appSecret, token, callbackPath, apiBaseURL, tokenURL string) *WeChatOfficialAdapter {
	callbackPath = normalizeCallbackPath(callbackPath)
	apiBaseURL = strings.TrimSpace(apiBaseURL)
	if apiBaseURL == "" {
		apiBaseURL = wechatOfficialDefaultAPIBase
	}
	tokenURL = strings.TrimSpace(tokenURL)
	if tokenURL == "" {
		tokenURL = wechatOfficialDefaultTokenURL
	}
	return &WeChatOfficialAdapter{
		appID:          strings.TrimSpace(appID),
		appSecret:      strings.TrimSpace(appSecret),
		token:          strings.TrimSpace(token),
		callbackPath:   callbackPath,
		apiBaseURL:     strings.TrimRight(apiBaseURL, "/"),
		tokenURL:       tokenURL,
		httpClient:     &http.Client{Timeout: 15 * time.Second},
		passiveReplies: make(map[string]chan wechatOfficialPassiveReply),
		stopped:        make(chan struct{}),
	}
}

func (a *WeChatOfficialAdapter) GetPlatform() string {
	return platformName
}

func (a *WeChatOfficialAdapter) SetMessageHandler(handler func(*types.Message)) {
	a.messageHandler = handler
}

func (a *WeChatOfficialAdapter) Start() error {
	if a.appID == "" {
		return fmt.Errorf("微信公众号 AppID 不能为空")
	}
	if a.appSecret == "" {
		return fmt.Errorf("微信公众号 AppSecret 不能为空")
	}
	if a.token == "" {
		return fmt.Errorf("微信公众号 Token 不能为空")
	}
	log.Printf("微信公众号 Adapter 已启动，回调路径: %s", a.callbackPath)
	return nil
}

func (a *WeChatOfficialAdapter) Stop() error {
	a.stopOnce.Do(func() {
		close(a.stopped)
	})
	log.Printf("微信公众号 Adapter 已停止")
	return nil
}

func (a *WeChatOfficialAdapter) ReplyTarget(msg *types.Message) string {
	if msg == nil {
		return ""
	}
	if msg.Metadata != nil {
		if target := strings.TrimSpace(msg.Metadata["reply_target"]); target != "" {
			return target
		}
		if openID := strings.TrimSpace(msg.Metadata["wechat_openid"]); openID != "" {
			return openID
		}
	}
	return strings.TrimSpace(msg.UserID)
}

func (a *WeChatOfficialAdapter) FormatReplyText(msg *types.Message, text string) string {
	return text
}

func (a *WeChatOfficialAdapter) SendTarget(userID string, groupID string) string {
	return strings.TrimSpace(userID)
}

func (a *WeChatOfficialAdapter) SendMessage(target string, text string) error {
	target = strings.TrimSpace(target)
	if target == "" {
		return fmt.Errorf("微信公众号发送目标不能为空")
	}
	if a.sendPassiveReply(target, text) {
		log.Printf("[发送][微信公众号][%s][被动回复]：%s", target, text)
		return nil
	}
	body := map[string]interface{}{
		"touser":  target,
		"msgtype": "text",
		"text": map[string]string{
			"content": text,
		},
	}
	if err := a.callAPI(http.MethodPost, "/cgi-bin/message/custom/send", body, nil); err != nil {
		log.Printf("[发送失败][微信公众号][%s][客服消息]：%v", target, err)
		return err
	}
	log.Printf("[发送][微信公众号][%s][客服消息]：%s", target, text)
	return nil
}

func (a *WeChatOfficialAdapter) SendImage(target string, imageURL string) error {
	imageURL = strings.TrimSpace(imageURL)
	if imageURL == "" {
		return fmt.Errorf("微信公众号图片链接不能为空")
	}
	return a.SendMessage(target, "暂不支持图片，点击链接->"+imageURL)
}

func (a *WeChatOfficialAdapter) SendRichMessage(target string, message types.RichMessage) error {
	parts := make([]string, 0, len(message.Parts))
	for _, part := range message.Parts {
		switch strings.TrimSpace(part.Type) {
		case "text":
			if text := strings.TrimSpace(part.Text); text != "" {
				parts = append(parts, text)
			}
		case "markdown":
			if text := strings.TrimSpace(part.Markdown); text != "" {
				parts = append(parts, text)
			}
		case "image":
			if imageURL := strings.TrimSpace(part.URL); imageURL != "" {
				parts = append(parts, "暂不支持图片，点击链接->"+imageURL)
			}
		}
	}
	if len(parts) == 0 {
		if text := strings.TrimSpace(message.FallbackText); text != "" {
			parts = append(parts, text)
		}
	}
	if len(parts) == 0 {
		return fmt.Errorf("微信公众号富文本消息内容不能为空")
	}
	return a.SendMessage(target, strings.Join(parts, "\n\n"))
}

func (a *WeChatOfficialAdapter) SendFile(target string, filePath string) error {
	return fmt.Errorf("微信公众号文件发送暂未实现")
}

func (a *WeChatOfficialAdapter) GetUserInfo(userID string) (*UserInfo, error) {
	return &UserInfo{UserID: userID, Nickname: userID, Extra: map[string]string{"platform": platformName}}, nil
}

func (a *WeChatOfficialAdapter) GetGroupInfo(groupID string) (*GroupInfo, error) {
	return nil, fmt.Errorf("微信公众号无群组信息")
}

func (a *WeChatOfficialAdapter) AtUser(groupID string, userID string) error {
	return fmt.Errorf("微信公众号不支持 @ 用户")
}

// HandleHTTPCallback 处理微信公众号服务器配置校验和消息推送。
func (a *WeChatOfficialAdapter) HandleHTTPCallback(relativePath string, w http.ResponseWriter, r *http.Request) {
	if normalizeCallbackPath(relativePath) != a.callbackPath {
		http.NotFound(w, r)
		return
	}
	switch r.Method {
	case http.MethodGet:
		a.handleVerifyCallback(w, r)
	case http.MethodPost:
		a.handleMessageCallback(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (a *WeChatOfficialAdapter) handleVerifyCallback(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	if !a.verifySignature(query.Get("signature"), query.Get("timestamp"), query.Get("nonce")) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(query.Get("echostr")))
}

func (a *WeChatOfficialAdapter) handleMessageCallback(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	if !a.verifySignature(query.Get("signature"), query.Get("timestamp"), query.Get("nonce")) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	msg, err := a.parseMessageXML(payload)
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	if msg == nil {
		writeWeChatOfficialSuccess(w)
		return
	}
	replyCh := a.registerPassiveReply(msg.UserID)
	defer a.unregisterPassiveReply(msg.UserID, replyCh)
	go a.dispatchMessage(msg)
	replies := collectWeChatOfficialPassiveReplies(replyCh, wechatOfficialPassiveReplyWait)
	if len(replies) == 0 {
		writeWeChatOfficialSuccess(w)
		return
	}
	a.writePassiveTextReply(w, msg, strings.Join(replies, "\n\n"))
}

func (a *WeChatOfficialAdapter) verifySignature(signature, timestamp, nonce string) bool {
	signature = strings.TrimSpace(signature)
	timestamp = strings.TrimSpace(timestamp)
	nonce = strings.TrimSpace(nonce)
	if signature == "" || timestamp == "" || nonce == "" || a.token == "" {
		return false
	}
	values := []string{a.token, timestamp, nonce}
	sort.Strings(values)
	sum := sha1.Sum([]byte(strings.Join(values, "")))
	expected := hex.EncodeToString(sum[:])
	return subtle.ConstantTimeCompare([]byte(expected), []byte(strings.ToLower(signature))) == 1
}

func (a *WeChatOfficialAdapter) parseMessageXML(payload []byte) (*types.Message, error) {
	var incoming wechatOfficialMessageXML
	if err := xml.Unmarshal(payload, &incoming); err != nil {
		return nil, err
	}
	msgType := strings.TrimSpace(incoming.MsgType)
	switch msgType {
	case "text":
		content := strings.TrimSpace(incoming.Content)
		if content == "" {
			return nil, nil
		}
		return a.buildMessage(incoming, content), nil
	case "event":
		return a.buildMessage(incoming, wechatEventContent(incoming.Event, incoming.EventKey)), nil
	default:
		return nil, nil
	}
}

func (a *WeChatOfficialAdapter) buildMessage(incoming wechatOfficialMessageXML, content string) *types.Message {
	openID := strings.TrimSpace(incoming.FromUserName)
	messageID := strings.TrimSpace(incoming.MsgID)
	if messageID == "" {
		messageID = strings.Join([]string{"event", strings.TrimSpace(incoming.Event), openID, strings.TrimSpace(incoming.CreateTime)}, ":")
	}
	metadata := map[string]string{
		"message_type":          "private",
		"reply_target":          openID,
		"wechat_openid":         openID,
		"wechat_to_user_name":   strings.TrimSpace(incoming.ToUserName),
		"wechat_from_user_name": openID,
		"wechat_msg_type":       strings.TrimSpace(incoming.MsgType),
		"wechat_create_time":    strings.TrimSpace(incoming.CreateTime),
		"wechat_msg_id":         strings.TrimSpace(incoming.MsgID),
	}
	if strings.TrimSpace(incoming.MsgType) == "event" {
		metadata["wechat_event"] = strings.TrimSpace(incoming.Event)
		metadata["wechat_event_key"] = strings.TrimSpace(incoming.EventKey)
	}
	return &types.Message{
		ID:       messageID,
		Platform: platformName,
		UserID:   openID,
		Content:  content,
		Metadata: metadata,
	}
}

func wechatEventContent(event, eventKey string) string {
	event = strings.TrimSpace(event)
	eventKey = strings.TrimSpace(eventKey)
	if event == "" {
		return "event:unknown"
	}
	if event == "CLICK" && eventKey != "" {
		return "event:CLICK:" + eventKey
	}
	return "event:" + event
}

func (a *WeChatOfficialAdapter) dispatchMessage(msg *types.Message) {
	if a.messageHandler != nil {
		a.messageHandler(msg)
	}
}

func (a *WeChatOfficialAdapter) registerPassiveReply(openID string) chan wechatOfficialPassiveReply {
	ch := make(chan wechatOfficialPassiveReply, 1)
	a.passiveReplyMu.Lock()
	a.passiveReplies[openID] = ch
	a.passiveReplyMu.Unlock()
	return ch
}

func (a *WeChatOfficialAdapter) unregisterPassiveReply(openID string, ch chan wechatOfficialPassiveReply) {
	a.passiveReplyMu.Lock()
	if a.passiveReplies[openID] == ch {
		delete(a.passiveReplies, openID)
	}
	a.passiveReplyMu.Unlock()
}

func (a *WeChatOfficialAdapter) sendPassiveReply(target string, text string) bool {
	a.passiveReplyMu.Lock()
	ch := a.passiveReplies[target]
	a.passiveReplyMu.Unlock()
	if ch == nil {
		return false
	}
	select {
	case ch <- wechatOfficialPassiveReply{target: target, text: text}:
		return true
	case <-time.After(wechatOfficialPassiveReplyWait):
		return false
	}
}

func collectWeChatOfficialPassiveReplies(ch <-chan wechatOfficialPassiveReply, wait time.Duration) []string {
	deadline := time.NewTimer(wait)
	defer deadline.Stop()
	replies := make([]string, 0, 4)
	for {
		select {
		case reply := <-ch:
			text := strings.TrimSpace(reply.text)
			if text != "" {
				replies = append(replies, text)
			}
		case <-deadline.C:
			return replies
		}
	}
}

func (a *WeChatOfficialAdapter) writePassiveTextReply(w http.ResponseWriter, msg *types.Message, text string) {
	reply := wechatOfficialPassiveReplyXML{
		ToUserName:   strings.TrimSpace(msg.Metadata["wechat_from_user_name"]),
		FromUserName: strings.TrimSpace(msg.Metadata["wechat_to_user_name"]),
		CreateTime:   time.Now().Unix(),
		MsgType:      "text",
		Content:      text,
	}
	payload, err := xml.Marshal(reply)
	if err != nil {
		log.Printf("[发送失败][微信公众号][%s][被动回复]：%v", msg.UserID, err)
		writeWeChatOfficialSuccess(w)
		return
	}
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	_, _ = w.Write(payload)
}

func writeWeChatOfficialSuccess(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("success"))
}

func (a *WeChatOfficialAdapter) getAccessToken() (string, error) {
	a.tokenMu.Lock()
	defer a.tokenMu.Unlock()

	if a.accessToken != "" && time.Now().Before(a.tokenExpiresAt.Add(-wechatOfficialTokenRefreshLead)) {
		return a.accessToken, nil
	}

	requestURL, err := url.Parse(a.tokenURL)
	if err != nil {
		return "", err
	}
	query := requestURL.Query()
	query.Set("grant_type", "client_credential")
	query.Set("appid", a.appID)
	query.Set("secret", a.appSecret)
	requestURL.RawQuery = query.Encode()

	request, err := http.NewRequest(http.MethodGet, requestURL.String(), nil)
	if err != nil {
		return "", err
	}
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
		return "", fmt.Errorf("微信公众号 token 接口状态码 %d: %s", resp.StatusCode, string(payload))
	}
	var result wechatOfficialTokenResponse
	if err := json.Unmarshal(payload, &result); err != nil {
		return "", err
	}
	if result.ErrCode != 0 {
		return "", fmt.Errorf("微信公众号 token 获取失败 errcode=%d errmsg=%s", result.ErrCode, result.ErrMsg)
	}
	if strings.TrimSpace(result.AccessToken) == "" {
		return "", fmt.Errorf("微信公众号 token 响应缺少 access_token")
	}
	if result.ExpiresIn <= 0 {
		return "", fmt.Errorf("微信公众号 token 响应缺少有效 expires_in")
	}
	a.accessToken = result.AccessToken
	a.tokenExpiresAt = time.Now().Add(time.Duration(result.ExpiresIn) * time.Second)
	return a.accessToken, nil
}

func (a *WeChatOfficialAdapter) callAPI(method, path string, body interface{}, result interface{}) error {
	return a.callAPIWithRetry(method, path, body, result, true)
}

func (a *WeChatOfficialAdapter) callAPIWithRetry(method, path string, body interface{}, result interface{}, retryToken bool) error {
	accessToken, err := a.getAccessToken()
	if err != nil {
		return err
	}
	requestURL := strings.TrimRight(a.apiBaseURL, "/") + "/" + strings.TrimLeft(path, "/")
	parsedURL, err := url.Parse(requestURL)
	if err != nil {
		return err
	}
	query := parsedURL.Query()
	query.Set("access_token", accessToken)
	parsedURL.RawQuery = query.Encode()

	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewBuffer(payload)
	}
	request, err := http.NewRequest(method, parsedURL.String(), reader)
	if err != nil {
		return err
	}
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
		return fmt.Errorf("微信公众号 API %s %s 状态码 %d: %s", method, path, resp.StatusCode, string(payload))
	}
	var apiResult wechatOfficialAPIResponse
	if len(payload) > 0 {
		if err := json.Unmarshal(payload, &apiResult); err != nil {
			return err
		}
	}
	if apiResult.ErrCode != 0 {
		if retryToken && wechatOfficialAccessTokenExpired(apiResult.ErrCode) {
			a.clearAccessToken()
			return a.callAPIWithRetry(method, path, body, result, false)
		}
		return fmt.Errorf("微信公众号 API %s %s 失败 errcode=%d errmsg=%s", method, path, apiResult.ErrCode, apiResult.ErrMsg)
	}
	if result == nil || len(payload) == 0 {
		return nil
	}
	return json.Unmarshal(payload, result)
}

func (a *WeChatOfficialAdapter) clearAccessToken() {
	a.tokenMu.Lock()
	defer a.tokenMu.Unlock()
	a.accessToken = ""
	a.tokenExpiresAt = time.Time{}
}

func wechatOfficialAccessTokenExpired(errCode int) bool {
	return errCode == 40001 || errCode == 40014 || errCode == 42001
}

func normalizeCallbackPath(path string) string {
	path = strings.Trim(strings.TrimSpace(path), "/")
	if path == "" {
		return wechatOfficialDefaultPath
	}
	return path
}
