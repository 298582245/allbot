package session

import (
	"testing"
	"time"
)

func TestSessionScopeIsolation(t *testing.T) {
	tests := []struct {
		name  string
		left  Scope
		right Scope
	}{
		{
			name:  "platform",
			left:  Scope{Platform: "feishu", AdapterID: "a1", UserID: "u1", GroupID: "g1", Namespace: "plugin"},
			right: Scope{Platform: "wechat", AdapterID: "a1", UserID: "u1", GroupID: "g1", Namespace: "plugin"},
		},
		{
			name:  "adapter",
			left:  Scope{Platform: "feishu", AdapterID: "a1", UserID: "u1", GroupID: "g1", Namespace: "plugin"},
			right: Scope{Platform: "feishu", AdapterID: "a2", UserID: "u1", GroupID: "g1", Namespace: "plugin"},
		},
		{
			name:  "user",
			left:  Scope{Platform: "feishu", AdapterID: "a1", UserID: "u1", GroupID: "g1", Namespace: "plugin"},
			right: Scope{Platform: "feishu", AdapterID: "a1", UserID: "u2", GroupID: "g1", Namespace: "plugin"},
		},
		{
			name:  "group",
			left:  Scope{Platform: "feishu", AdapterID: "a1", UserID: "u1", GroupID: "g1", Namespace: "plugin"},
			right: Scope{Platform: "feishu", AdapterID: "a1", UserID: "u1", GroupID: "g2", Namespace: "plugin"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewManager()
			ch, cancel := manager.CreateCancellableSession(tt.left, 30)
			defer cancel()

			if manager.HandleMessage(tt.right, "wrong") {
				t.Fatal("不同作用域不应消费等待会话")
			}
			if !manager.HandleMessage(tt.left, "right") {
				t.Fatal("相同作用域应消费等待会话")
			}
			select {
			case got := <-ch:
				if got != "right" {
					t.Fatalf("收到内容 %q", got)
				}
			case <-time.After(time.Second):
				t.Fatal("未收到等待消息")
			}
		})
	}
}

func TestSessionNamespaceRestrictionAndReplacement(t *testing.T) {
	manager := NewManager()
	base := Scope{Platform: "feishu", AdapterID: "a1", UserID: "u1", GroupID: "g1"}
	first := base
	first.Namespace = "first"
	firstCh := manager.CreateSession(first, 30)

	second := base
	second.Namespace = "second"
	secondCh, cancel := manager.CreateCancellableSession(second, 30)
	defer cancel()

	select {
	case _, ok := <-firstCh:
		if ok {
			t.Fatal("被替换会话不应收到消息")
		}
	case <-time.After(time.Second):
		t.Fatal("被替换会话未关闭")
	}
	if manager.HandleMessageForPlugin(base, "first", "wrong") {
		t.Fatal("错误命名空间不应消费等待会话")
	}
	if !manager.HandleMessageForPlugin(base, "second", "right") {
		t.Fatal("正确命名空间应消费等待会话")
	}
	select {
	case got := <-secondCh:
		if got != "right" {
			t.Fatalf("收到内容 %q", got)
		}
	case <-time.After(time.Second):
		t.Fatal("未收到等待消息")
	}
}

func TestCancellableSessionClosesChannel(t *testing.T) {
	manager := NewManager()
	scope := Scope{Platform: "feishu", AdapterID: "a1", UserID: "u1", Namespace: "plugin"}
	ch, cancel := manager.CreateCancellableSession(scope, 30)
	cancel()
	cancel()

	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("取消后通道仍可读")
		}
	case <-time.After(time.Second):
		t.Fatal("取消后通道未关闭")
	}
	if manager.GetSession(scope) != nil {
		t.Fatal("取消后会话仍存在")
	}
}
