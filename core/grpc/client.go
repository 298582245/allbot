package grpc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client gRPC 客户端（简化实现：HTTP + JSON）
type Client struct {
	serverURL string
	client    *http.Client
}

// NewClient 创建 gRPC 客户端
func NewClient(port int) *Client {
	return &Client{
		serverURL: fmt.Sprintf("http://localhost:%d", port),
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// MessageRequest 消息请求
type MessageRequest struct {
	PluginID string            `json:"plugin_id"`
	Platform string            `json:"platform"`
	UserID   string            `json:"user_id"`
	GroupID  string            `json:"group_id"`
	Content  string            `json:"content"`
	MessageID string           `json:"message_id"`
	Metadata map[string]string `json:"metadata"`
}

// MessageResponse 消息响应
type MessageResponse struct {
	Success bool     `json:"success"`
	Error   string   `json:"error"`
	Replies []string `json:"replies"` // 要发送的回复消息列表
}

// ListenRequest 等待消息请求
type ListenRequest struct {
	PluginID string `json:"plugin_id"`
	UserID   string `json:"user_id"`
	GroupID  string `json:"group_id"`
	Timeout  int    `json:"timeout"`
}

// ListenResponse 等待消息响应
type ListenResponse struct {
	Content string `json:"content"`
}

// ReplyRequest 回复消息请求
type ReplyRequest struct {
	Platform string `json:"platform"`
	UserID   string `json:"user_id"`
	GroupID  string `json:"group_id"`
	Text     string `json:"text"`
}

// ReplyResponse 回复消息响应
type ReplyResponse struct {
	Success   bool   `json:"success"`
	Error     string `json:"error"`
	MessageID string `json:"message_id"`
}

// Handle 调用插件处理消息
func (c *Client) Handle(req *MessageRequest) (*MessageResponse, error) {
	resp := &MessageResponse{}
	if err := c.post("/handle", req, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// Listen 等待用户消息
func (c *Client) Listen(req *ListenRequest) (*ListenResponse, error) {
	resp := &ListenResponse{}
	if err := c.post("/listen", req, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// Reply 回复消息
func (c *Client) Reply(req *ReplyRequest) (*ReplyResponse, error) {
	resp := &ReplyResponse{}
	if err := c.post("/reply", req, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// post 发送 POST 请求
func (c *Client) post(path string, req interface{}, resp interface{}) error {
	data, err := json.Marshal(req)
	if err != nil {
		return err
	}

	httpResp, err := c.client.Post(c.serverURL+path, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	defer httpResp.Body.Close()

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(body, resp); err != nil {
		return err
	}

	return nil
}
