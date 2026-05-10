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
	Success bool   `json:"success"`
	Error   string `json:"error"`
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
	return c.post("/handle", req, &MessageResponse{})
}

// Listen 等待用户消息
func (c *Client) Listen(req *ListenRequest) (*ListenResponse, error) {
	return c.post("/listen", req, &ListenResponse{})
}

// Reply 回复消息
func (c *Client) Reply(req *ReplyRequest) (*ReplyResponse, error) {
	return c.post("/reply", req, &ReplyResponse{})
}

// post 发送 POST 请求
func (c *Client) post(path string, req interface{}, resp interface{}) (interface{}, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpResp, err := c.client.Post(c.serverURL+path, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}
	defer httpResp.Body.Close()

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(body, resp); err != nil {
		return nil, err
	}

	return resp, nil
}
