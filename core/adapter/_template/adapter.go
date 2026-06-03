package template

import (
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/allbot/allbot/core/adapter/_contract"
	"github.com/allbot/allbot/core/types"
)

type UserInfo = contract.UserInfo
type GroupInfo = contract.GroupInfo

const platformName = "template"

type ExampleAdapter struct {
	token          string
	messageHandler func(*types.Message)
	stopChan       chan struct{}
	stopOnce       sync.Once
}

func NewExampleAdapter(token string) *ExampleAdapter {
	return &ExampleAdapter{
		token:    strings.TrimSpace(token),
		stopChan: make(chan struct{}),
	}
}

func (a *ExampleAdapter) GetPlatform() string {
	return platformName
}

func (a *ExampleAdapter) SetMessageHandler(handler func(*types.Message)) {
	a.messageHandler = handler
}

func (a *ExampleAdapter) Start() error {
	if a.token == "" {
		return fmt.Errorf("Example token 不能为空")
	}
	go a.listen()
	log.Printf("Example Adapter 已启动")
	return nil
}

func (a *ExampleAdapter) Stop() error {
	a.stopOnce.Do(func() {
		close(a.stopChan)
	})
	log.Printf("Example Adapter 已停止")
	return nil
}

func (a *ExampleAdapter) listen() {
	for {
		select {
		case <-a.stopChan:
			return
		default:
			return
		}
	}
}

func (a *ExampleAdapter) handleIncomingMessage(messageID string, userID string, groupID string, content string) {
	content = strings.TrimSpace(content)
	if content == "" {
		return
	}
	msg := &types.Message{
		ID:       messageID,
		Platform: platformName,
		UserID:   userID,
		GroupID:  groupID,
		Content:  content,
		Metadata: map[string]string{
			"message_type": messageType(groupID),
		},
	}
	if a.messageHandler != nil {
		a.messageHandler(msg)
	}
}

func messageType(groupID string) string {
	if strings.TrimSpace(groupID) != "" {
		return "group"
	}
	return "private"
}

func (a *ExampleAdapter) ReplyTarget(msg *types.Message) string {
	if msg == nil {
		return ""
	}
	if msg.Metadata != nil {
		if target := strings.TrimSpace(msg.Metadata["reply_target"]); target != "" {
			return target
		}
	}
	return a.SendTarget(msg.UserID, msg.GroupID)
}

func (a *ExampleAdapter) FormatReplyText(msg *types.Message, text string) string {
	if msg == nil || msg.GroupID == "" {
		return text
	}
	return "@" + msg.UserID + " " + text
}

func (a *ExampleAdapter) SendTarget(userID string, groupID string) string {
	if groupID != "" {
		return "group_" + groupID
	}
	return "user_" + userID
}

func (a *ExampleAdapter) SendMessage(target string, text string) error {
	if strings.TrimSpace(target) == "" {
		return fmt.Errorf("Example 发送目标不能为空")
	}
	log.Printf("[发送][Example][%s]：%s", target, text)
	return nil
}

func (a *ExampleAdapter) SendImage(target string, imageURL string) error {
	return fmt.Errorf("Example 图片发送暂未实现")
}

func (a *ExampleAdapter) SendFile(target string, filePath string) error {
	return fmt.Errorf("Example 文件发送暂未实现")
}

func (a *ExampleAdapter) GetUserInfo(userID string) (*UserInfo, error) {
	return &UserInfo{UserID: userID, Nickname: userID, Extra: map[string]string{"platform": platformName}}, nil
}

func (a *ExampleAdapter) GetGroupInfo(groupID string) (*GroupInfo, error) {
	return &GroupInfo{GroupID: groupID, Name: groupID, Extra: map[string]string{"platform": platformName}}, nil
}

func (a *ExampleAdapter) AtUser(groupID string, userID string) error {
	return a.SendMessage("group_"+groupID, "@"+userID)
}
