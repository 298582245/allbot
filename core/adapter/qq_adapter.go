package adapter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/allbot/allbot/core/types"
)

// QQAdapter QQ 平台适配器（基于 go-cqhttp）
type QQAdapter struct {
	apiURL         string
	listenAddr     string
	messageHandler func(*types.Message)
	httpServer     *http.Server
}

// NewQQAdapter 创建 QQ 适配器
func NewQQAdapter(apiURL string, listenAddr string) *QQAdapter {
	return &QQAdapter{
		apiURL:     apiURL,
		listenAddr: listenAddr,
	}
}

// GetPlatform 获取平台名称
func (a *QQAdapter) GetPlatform() string {
	return "qq"
}

// SetMessageHandler 设置消息处理器
func (a *QQAdapter) SetMessageHandler(handler func(*types.Message)) {
	a.messageHandler = handler
}

// Start 启动适配器
func (a *QQAdapter) Start() error {
	// 启动 HTTP 服务器接收 go-cqhttp 的上报
	mux := http.NewServeMux()
	mux.HandleFunc("/", a.handleWebhook)

	a.httpServer = &http.Server{
		Addr:    a.listenAddr,
		Handler: mux,
	}

	go func() {
		log.Printf("QQ Adapter listening on %s", a.listenAddr)
		if err := a.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("QQ Adapter HTTP server error: %v", err)
		}
	}()

	return nil
}

// Stop 停止适配器
func (a *QQAdapter) Stop() error {
	if a.httpServer != nil {
		return a.httpServer.Close()
	}
	return nil
}

// handleWebhook 处理 go-cqhttp 的上报
func (a *QQAdapter) handleWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Failed to read webhook body: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var event map[string]interface{}
	if err := json.Unmarshal(body, &event); err != nil {
		log.Printf("Failed to parse webhook body: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// 处理消息事件
	postType, _ := event["post_type"].(string)
	if postType == "message" {
		a.handleMessageEvent(event)
	}

	w.WriteHeader(http.StatusOK)
}

// handleMessageEvent 处理消息事件
func (a *QQAdapter) handleMessageEvent(event map[string]interface{}) {
	messageType, _ := event["message_type"].(string)
	userID := fmt.Sprintf("%v", event["user_id"])
	content, _ := event["message"].(string)
	messageID := fmt.Sprintf("%v", event["message_id"])

	msg := &types.Message{
		ID:       messageID,
		Platform: "qq",
		UserID:   userID,
		Content:  content,
		Metadata: make(map[string]string),
	}

	// 群消息
	if messageType == "group" {
		msg.GroupID = fmt.Sprintf("%v", event["group_id"])
	}

	if a.messageHandler != nil {
		a.messageHandler(msg)
	}
}

// SendMessage 发送消息
func (a *QQAdapter) SendMessage(target string, text string) error {
	// 判断是群消息还是私聊
	// 简化实现：如果 target 包含 "group_"，则为群消息
	var messageType string
	var targetID string

	if len(target) > 6 && target[:6] == "group_" {
		messageType = "group"
		targetID = target[6:]
	} else {
		messageType = "private"
		targetID = target
	}

	data := map[string]interface{}{
		"message_type": messageType,
		"message":      text,
	}

	if messageType == "group" {
		data["group_id"] = targetID
	} else {
		data["user_id"] = targetID
	}

	return a.callAPI("/send_msg", data)
}

// SendImage 发送图片
func (a *QQAdapter) SendImage(target string, imageURL string) error {
	message := fmt.Sprintf("[CQ:image,file=%s]", imageURL)
	return a.SendMessage(target, message)
}

// SendFile 发送文件
func (a *QQAdapter) SendFile(target string, filePath string) error {
	// go-cqhttp 文件上传实现
	return fmt.Errorf("file sending not implemented yet")
}

// GetUserInfo 获取用户信息
func (a *QQAdapter) GetUserInfo(userID string) (*UserInfo, error) {
	var result map[string]interface{}
	if err := a.callAPIWithResult("/get_stranger_info", map[string]interface{}{
		"user_id": userID,
	}, &result); err != nil {
		return nil, err
	}

	data, _ := result["data"].(map[string]interface{})
	return &UserInfo{
		UserID:   userID,
		Nickname: fmt.Sprintf("%v", data["nickname"]),
		Avatar:   fmt.Sprintf("https://q1.qlogo.cn/g?b=qq&nk=%s&s=640", userID),
		Extra:    make(map[string]string),
	}, nil
}

// GetGroupInfo 获取群组信息
func (a *QQAdapter) GetGroupInfo(groupID string) (*GroupInfo, error) {
	var result map[string]interface{}
	if err := a.callAPIWithResult("/get_group_info", map[string]interface{}{
		"group_id": groupID,
	}, &result); err != nil {
		return nil, err
	}

	data, _ := result["data"].(map[string]interface{})
	memberCount, _ := data["member_count"].(float64)

	return &GroupInfo{
		GroupID:     groupID,
		Name:        fmt.Sprintf("%v", data["group_name"]),
		MemberCount: int(memberCount),
		Extra:       make(map[string]string),
	}, nil
}

// AtUser @某人
func (a *QQAdapter) AtUser(groupID string, userID string) error {
	message := fmt.Sprintf("[CQ:at,qq=%s]", userID)
	return a.SendMessage("group_"+groupID, message)
}

// callAPI 调用 go-cqhttp API
func (a *QQAdapter) callAPI(endpoint string, data map[string]interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	resp, err := http.Post(a.apiURL+endpoint, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API call failed with status: %d", resp.StatusCode)
	}

	return nil
}

// callAPIWithResult 调用 API 并返回结果
func (a *QQAdapter) callAPIWithResult(endpoint string, data map[string]interface{}, result interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	resp, err := http.Post(a.apiURL+endpoint, "application/json", bytes.NewBuffer(jsonData))
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
