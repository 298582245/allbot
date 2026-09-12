package router

import (
	"testing"
	"time"

	"github.com/allbot/allbot/core/adapter"
	"github.com/allbot/allbot/core/types"
)

type messageDeletionTestAdapter struct {
	deleted chan *types.Message
}

func (a *messageDeletionTestAdapter) GetPlatform() string              { return "test" }
func (a *messageDeletionTestAdapter) SendMessage(string, string) error { return nil }
func (a *messageDeletionTestAdapter) SendImage(string, string) error   { return nil }
func (a *messageDeletionTestAdapter) SendFile(string, string) error    { return nil }
func (a *messageDeletionTestAdapter) GetUserInfo(userID string) (*adapter.UserInfo, error) {
	return &adapter.UserInfo{UserID: userID}, nil
}
func (a *messageDeletionTestAdapter) GetGroupInfo(groupID string) (*adapter.GroupInfo, error) {
	return &adapter.GroupInfo{GroupID: groupID}, nil
}
func (a *messageDeletionTestAdapter) AtUser(string, string) error            { return nil }
func (a *messageDeletionTestAdapter) Start() error                           { return nil }
func (a *messageDeletionTestAdapter) Stop() error                            { return nil }
func (a *messageDeletionTestAdapter) SetMessageHandler(func(*types.Message)) {}

func (a *messageDeletionTestAdapter) DeleteMessage(msg *types.Message) error {
	a.deleted <- msg
	return nil
}

func TestScheduleMessageDeletionUsesOptionalAdapterCapability(t *testing.T) {
	deleted := make(chan *types.Message, 1)
	adp := &messageDeletionTestAdapter{deleted: deleted}
	r := NewRouter(nil)
	r.SetAdapters(map[string]adapter.Adapter{"test": adp})
	message := &types.Message{ID: "message-1", Platform: "test", UserID: "user-1", Content: "input"}

	r.scheduleMessageDeletion(message, 1)
	select {
	case got := <-deleted:
		if got != message {
			t.Fatalf("deleted message = %#v, expected original message", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("消息未在撤回时间到达后撤回")
	}
}

func TestScheduleMessageDeletionSkipsUnsupportedAdapters(t *testing.T) {
	r := NewRouter(nil)
	r.SetAdapters(map[string]adapter.Adapter{"qq": &keywordReplyFakeAdapter{}})
	r.scheduleMessageDeletion(&types.Message{ID: "message-1", Platform: "qq"}, 1)
}
