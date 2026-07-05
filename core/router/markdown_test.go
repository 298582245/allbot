package router

import (
	"strings"
	"sync"
	"testing"

	"github.com/allbot/allbot/core/types"
)

type markdownRecordingAdapter struct {
	*keywordReplyFakeAdapter
	mu        sync.Mutex
	markdowns []sentKeywordReplyMessage
}

func (a *markdownRecordingAdapter) SendMarkdown(target string, markdown string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.markdowns = append(a.markdowns, sentKeywordReplyMessage{target: target, text: markdown})
	return nil
}

func (a *markdownRecordingAdapter) sentMarkdowns() []sentKeywordReplyMessage {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]sentKeywordReplyMessage(nil), a.markdowns...)
}

type prefixReplyFormatter struct{}

type buttonRecordingAdapter struct {
	*keywordReplyFakeAdapter
	buttons []struct {
		target string
		text   string
	}
}

func (a *buttonRecordingAdapter) SendButtons(target string, text string, buttons [][]types.ButtonOption) error {
	a.buttons = append(a.buttons, struct {
		target string
		text   string
	}{target: target, text: text})
	return nil
}

func (prefixReplyFormatter) FormatReplyText(msg *types.Message, text string) string {
	return "@" + msg.UserID + " " + text
}

func TestSendMarkdownWithFallbackUsesNativeSender(t *testing.T) {
	adapter := &markdownRecordingAdapter{keywordReplyFakeAdapter: &keywordReplyFakeAdapter{}}
	if err := sendMarkdownWithFallback(adapter, "target", "**hi**"); err != nil {
		t.Fatalf("sendMarkdownWithFallback returned error: %v", err)
	}
	if len(adapter.sentMessages()) != 0 {
		t.Fatalf("text fallback should not be used: %#v", adapter.sentMessages())
	}
	markdowns := adapter.sentMarkdowns()
	if len(markdowns) != 1 || markdowns[0].target != "target" || markdowns[0].text != "**hi**" {
		t.Fatalf("markdowns = %#v", markdowns)
	}
}

func TestSendMarkdownWithFallbackConvertsToPlainText(t *testing.T) {
	adapter := &keywordReplyFakeAdapter{}
	if err := sendMarkdownWithFallback(adapter, "target", "## 标题\n**内容**"); err != nil {
		t.Fatalf("sendMarkdownWithFallback returned error: %v", err)
	}
	messages := adapter.sentMessages()
	if len(messages) != 1 || messages[0].target != "target" || messages[0].text != "标题\n内容" {
		t.Fatalf("messages = %#v", messages)
	}
}

func TestSendReplyMarkdownWithFallbackFormatsPlainText(t *testing.T) {
	adapter := &keywordReplyFakeAdapter{textFormatter: prefixReplyFormatter{}}
	msg := &types.Message{UserID: "u1"}
	if err := sendReplyMarkdownWithFallback(adapter, msg, "target", "## 标题"); err != nil {
		t.Fatalf("sendReplyMarkdownWithFallback returned error: %v", err)
	}
	messages := adapter.sentMessages()
	if len(messages) != 1 || messages[0].text != "@u1 标题" {
		t.Fatalf("messages = %#v", messages)
	}
}

func TestSendReplyButtonsWithFallbackUsesNativeSender(t *testing.T) {
	adapter := &buttonRecordingAdapter{keywordReplyFakeAdapter: &keywordReplyFakeAdapter{}}
	if err := sendReplyButtonsWithFallback(adapter, &types.Message{UserID: "u1"}, "target", "请选择", [][]types.ButtonOption{{{Text: "A", Value: "1"}}}); err != nil {
		t.Fatalf("sendReplyButtonsWithFallback returned error: %v", err)
	}
	if len(adapter.sentMessages()) != 0 {
		t.Fatalf("text fallback should not be used: %#v", adapter.sentMessages())
	}
	if len(adapter.buttons) != 1 || adapter.buttons[0].target != "target" || adapter.buttons[0].text != "请选择" {
		t.Fatalf("buttons = %#v", adapter.buttons)
	}
}

func TestSendReplyButtonsWithFallbackPreservesPlainPrompt(t *testing.T) {
	adapter := &keywordReplyFakeAdapter{textFormatter: prefixReplyFormatter{}}
	prompt := "请选择支付方式\n1. 支付宝\n\nPS：发送对应数字进行选择"
	if err := sendReplyButtonsWithFallback(adapter, &types.Message{UserID: "u1"}, "target", prompt, [][]types.ButtonOption{{{Text: "支付宝", Value: "1"}}}); err != nil {
		t.Fatalf("sendReplyButtonsWithFallback returned error: %v", err)
	}
	messages := adapter.sentMessages()
	if len(messages) != 1 || messages[0].text != "@u1 "+prompt {
		t.Fatalf("messages = %#v", messages)
	}
}

func TestSendReplyMarkdownWithFallbackRejectsEmptyContent(t *testing.T) {
	err := sendReplyMarkdownWithFallback(&keywordReplyFakeAdapter{}, &types.Message{}, "target", "  ")
	if err == nil || !strings.Contains(err.Error(), "消息内容不能为空") {
		t.Fatalf("error = %v", err)
	}
}
