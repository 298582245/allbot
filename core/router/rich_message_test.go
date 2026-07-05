package router

import (
	"errors"
	"strings"
	"testing"

	coreadapter "github.com/allbot/allbot/core/adapter"
	plugincore "github.com/allbot/allbot/core/plugin"
	"github.com/allbot/allbot/core/types"
)

type richNativeAdapter struct {
	*keywordReplyFakeAdapter
	platform string
	rich     []types.RichMessage
}

func (a *richNativeAdapter) GetPlatform() string { return a.platform }

func (a *richNativeAdapter) SendRichMessage(target string, message types.RichMessage) error {
	a.rich = append(a.rich, message)
	return nil
}

type richMarkdownAdapter struct {
	*keywordReplyFakeAdapter
	platform    string
	markdowns   []sentKeywordReplyMessage
	markdownErr error
	images      []sentKeywordReplyMessage
}

func (a *richMarkdownAdapter) GetPlatform() string { return a.platform }

func (a *richMarkdownAdapter) SendMarkdown(target string, markdown string) error {
	if a.markdownErr != nil {
		return a.markdownErr
	}
	a.markdowns = append(a.markdowns, sentKeywordReplyMessage{target: target, text: markdown})
	return nil
}

func (a *richMarkdownAdapter) SendImage(target string, imageURL string) error {
	a.images = append(a.images, sentKeywordReplyMessage{target: target, text: imageURL})
	return nil
}

type richPlainAdapter struct {
	*keywordReplyFakeAdapter
	platform string
	images   []sentKeywordReplyMessage
}

func (a *richPlainAdapter) GetPlatform() string { return a.platform }

func (a *richPlainAdapter) SendImage(target string, imageURL string) error {
	a.images = append(a.images, sentKeywordReplyMessage{target: target, text: imageURL})
	return nil
}

func sampleRichMessage() types.RichMessage {
	return types.RichMessage{Parts: []types.RichMessagePart{
		{Type: "text", Text: "商品信息\n"},
		{Type: "image", URL: "https://example.com/a.png", Alt: "商品图"},
		{Type: "markdown", Markdown: "\n**价格**：9.90 元"},
	}}
}

func TestSendRichWithFallbackUsesNativeSender(t *testing.T) {
	adapter := &richNativeAdapter{keywordReplyFakeAdapter: &keywordReplyFakeAdapter{}, platform: "telegram"}
	if err := sendRichWithFallback(adapter, "target", sampleRichMessage()); err != nil {
		t.Fatalf("sendRichWithFallback returned error: %v", err)
	}
	if len(adapter.rich) != 1 || len(adapter.sentMessages()) != 0 {
		t.Fatalf("rich=%#v messages=%#v", adapter.rich, adapter.sentMessages())
	}
}

func TestSendRichWithFallbackQQUsesCQSingleMessage(t *testing.T) {
	adapter := &richPlainAdapter{keywordReplyFakeAdapter: &keywordReplyFakeAdapter{}, platform: "qq"}
	if err := sendRichWithFallback(adapter, "group", sampleRichMessage()); err != nil {
		t.Fatalf("sendRichWithFallback returned error: %v", err)
	}
	messages := adapter.sentMessages()
	if len(messages) != 1 || !strings.Contains(messages[0].text, "[CQ:image,file=https://example.com/a.png]") || len(adapter.images) != 0 {
		t.Fatalf("messages=%#v images=%#v", messages, adapter.images)
	}
}

func TestSendRichWithFallbackMarkdownSingleMessage(t *testing.T) {
	adapter := &richMarkdownAdapter{keywordReplyFakeAdapter: &keywordReplyFakeAdapter{}, platform: "telegram"}
	if err := sendRichWithFallback(adapter, "chat", sampleRichMessage()); err != nil {
		t.Fatalf("sendRichWithFallback returned error: %v", err)
	}
	if len(adapter.markdowns) != 1 || adapter.markdowns[0].text != "商品信息\n![商品图](https://example.com/a.png)\n**价格**：9.90 元" {
		t.Fatalf("markdowns=%#v", adapter.markdowns)
	}
	if len(adapter.sentMessages()) != 0 || len(adapter.images) != 0 {
		t.Fatalf("messages=%#v images=%#v", adapter.sentMessages(), adapter.images)
	}
}

func TestSendRichWithFallbackMarkdownFailureSplits(t *testing.T) {
	adapter := &richMarkdownAdapter{keywordReplyFakeAdapter: &keywordReplyFakeAdapter{}, platform: "telegram", markdownErr: errors.New("markdown failed")}
	if err := sendRichWithFallback(adapter, "chat", sampleRichMessage()); err != nil {
		t.Fatalf("sendRichWithFallback returned error: %v", err)
	}
	messages := adapter.sentMessages()
	if len(messages) != 2 || messages[0].text != "商品信息" || messages[1].text != "价格：9.90 元" {
		t.Fatalf("messages=%#v", messages)
	}
	if len(adapter.images) != 1 || adapter.images[0].text != "https://example.com/a.png" {
		t.Fatalf("images=%#v", adapter.images)
	}
}

func TestSendRichWithFallbackFeishuSplitsInsteadOfMarkdown(t *testing.T) {
	adapter := &richMarkdownAdapter{keywordReplyFakeAdapter: &keywordReplyFakeAdapter{}, platform: "feishu"}
	message := types.RichMessage{Parts: []types.RichMessagePart{
		{Type: "text", Text: "订单号：P1\n"},
		{Type: "image", URL: "https://example.com/pay.png", Alt: "支付二维码"},
	}, FallbackText: "订单号：P1"}
	if err := sendRichWithFallback(adapter, "chat", message); err != nil {
		t.Fatalf("sendRichWithFallback returned error: %v", err)
	}
	messages := adapter.sentMessages()
	if len(adapter.markdowns) != 0 || len(messages) != 1 || messages[0].text != "订单号：P1" || len(adapter.images) != 1 || adapter.images[0].text != "https://example.com/pay.png" {
		t.Fatalf("markdowns=%#v messages=%#v images=%#v", adapter.markdowns, messages, adapter.images)
	}
}

func TestSendRichWithFallbackWechatPlainTextOnly(t *testing.T) {
	adapter := &richPlainAdapter{keywordReplyFakeAdapter: &keywordReplyFakeAdapter{}, platform: "wechat_official"}
	if err := sendRichWithFallback(adapter, "open-id", sampleRichMessage()); err != nil {
		t.Fatalf("sendRichWithFallback returned error: %v", err)
	}
	messages := adapter.sentMessages()
	if len(messages) != 1 || messages[0].text != "商品信息\n商品图 (https://example.com/a.png)\n价格：9.90 元" || len(adapter.images) != 0 {
		t.Fatalf("messages=%#v images=%#v", messages, adapter.images)
	}
}

func TestSendReplyRichWithFallbackFormatsPlainText(t *testing.T) {
	adapter := &richPlainAdapter{keywordReplyFakeAdapter: &keywordReplyFakeAdapter{textFormatter: prefixReplyFormatter{}}, platform: "wechat_official"}
	msg := &types.Message{UserID: "u1"}
	if err := sendReplyRichWithFallback(adapter, msg, "target", sampleRichMessage()); err != nil {
		t.Fatalf("sendReplyRichWithFallback returned error: %v", err)
	}
	messages := adapter.sentMessages()
	if len(messages) != 1 || !strings.HasPrefix(messages[0].text, "@u1 商品信息") {
		t.Fatalf("messages=%#v", messages)
	}
}

func TestSendPluginRichMessageUsesAdapterResolver(t *testing.T) {
	adapter := &richMarkdownAdapter{keywordReplyFakeAdapter: &keywordReplyFakeAdapter{}, platform: "telegram"}
	r := NewRouter(nil)
	r.SetAdapters(map[string]coreadapter.Adapter{"telegram": adapter})
	result := r.SendPluginRichMessage("plugin", plugincore.RichMessageAction{Platform: "telegram", UserID: "u1", Parts: sampleRichMessage().Parts})
	if !result.Success {
		t.Fatalf("SendPluginRichMessage failed: %#v", result)
	}
	if len(adapter.markdowns) != 1 || adapter.markdowns[0].target != "u1" {
		t.Fatalf("markdowns=%#v", adapter.markdowns)
	}
}
