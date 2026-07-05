package utils

import (
	"testing"

	"github.com/allbot/allbot/core/types"
)

func TestMarkdownToPlainText(t *testing.T) {
	input := "# 标题\n\n**粗体** 和 *斜体* 以及 ~~删除~~\n`代码`\n\n[链接](https://example.com)\n![图片](https://example.com/a.png)\n![](https://example.com/empty.png)\n> 引用\n- 列表一\n1. 列表二\n\n```go\nfmt.Println(\"hi\")\n```\n\n| A | B |\n|---|---|\n| 1 | 2 |\n\n\n结尾"
	want := "标题\n\n粗体 和 斜体 以及 删除\n代码\n\n链接 (https://example.com)\n图片 (https://example.com/a.png)\nhttps://example.com/empty.png\n引用\n列表一\n列表二\n\nfmt.Println(\"hi\")\n\nA   B\n1   2\n\n结尾"
	if got := MarkdownToPlainText(input); got != want {
		t.Fatalf("MarkdownToPlainText() = %q, want %q", got, want)
	}
}

func TestMarkdownToPlainTextCompressesBlankLines(t *testing.T) {
	input := "标题\n\n\n\n内容\n---\n\n\n尾部"
	want := "标题\n\n内容\n\n尾部"
	if got := MarkdownToPlainText(input); got != want {
		t.Fatalf("MarkdownToPlainText() = %q, want %q", got, want)
	}
}

func TestRichMessageToPlainTextMarkdownAndCQ(t *testing.T) {
	message := types.RichMessage{Parts: []types.RichMessagePart{
		{Type: "text", Text: "商品信息\n"},
		{Type: "image", URL: "https://example.com/a.png?x=1,y=2", Alt: "商品图"},
		{Type: "markdown", Markdown: "\n**价格**：9.90 元"},
	}}
	if got := RichMessageToPlainText(message); got != "商品信息\n商品图 (https://example.com/a.png?x=1,y=2)\n价格：9.90 元" {
		t.Fatalf("RichMessageToPlainText() = %q", got)
	}
	if got := RichMessageToMarkdown(message); got != "商品信息\n![商品图](https://example.com/a.png?x=1,y=2)\n**价格**：9.90 元" {
		t.Fatalf("RichMessageToMarkdown() = %q", got)
	}
	if got := RichMessageToCQ(message); got != "商品信息\n[CQ:image,file=https://example.com/a.png?x=1&#44;y=2]\n价格：9.90 元" {
		t.Fatalf("RichMessageToCQ() = %q", got)
	}
}

func TestRichMessageFallbackTextAndEscaping(t *testing.T) {
	message := types.RichMessage{
		FallbackText: "自定义降级",
		Parts: []types.RichMessagePart{
			{Type: "text", Text: "a&[b]"},
			{Type: "image", URL: "https://example.com/a,b&c.png", Alt: "图]"},
		},
	}
	if got := RichMessageToPlainText(message); got != "自定义降级" {
		t.Fatalf("RichMessageToPlainText() = %q", got)
	}
	if got := RichMessageToCQ(message); got != "a&amp;&#91;b&#93;[CQ:image,file=https://example.com/a&#44;b&amp;c.png]" {
		t.Fatalf("RichMessageToCQ() = %q", got)
	}
	if got := RichMessageToMarkdown(message); got != "a&[b]![图\\]](https://example.com/a,b&c.png)" {
		t.Fatalf("RichMessageToMarkdown() = %q", got)
	}
}

func TestRichMessagePartsFiltersEmptyParts(t *testing.T) {
	parts := RichMessageParts(types.RichMessage{Parts: []types.RichMessagePart{
		{Type: "text", Text: ""},
		{Type: "text", Text: "hi"},
		{Type: "image", Alt: "empty"},
		{Type: "image", URL: " https://example.com/a.png "},
		{Type: "unknown", Text: "skip"},
	}})
	if len(parts) != 2 || parts[0].Text != "hi" || parts[1].URL != "https://example.com/a.png" {
		t.Fatalf("RichMessageParts() = %#v", parts)
	}
}
