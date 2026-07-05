package utils

import (
	"strings"

	"github.com/allbot/allbot/core/types"
)

func RichMessageToPlainText(message types.RichMessage) string {
	if text := strings.TrimSpace(message.FallbackText); text != "" {
		return text
	}
	items := make([]string, 0, len(message.Parts))
	for _, part := range message.Parts {
		switch strings.ToLower(strings.TrimSpace(part.Type)) {
		case "text":
			items = appendRichText(items, part.Text)
		case "markdown":
			items = appendRichText(items, richMarkdownPlainText(part.Markdown))
		case "image":
			items = appendRichText(items, richImagePlainText(part))
		}
	}
	return strings.TrimSpace(strings.Join(items, ""))
}

func RichMessageToMarkdown(message types.RichMessage) string {
	items := make([]string, 0, len(message.Parts))
	for _, part := range message.Parts {
		switch strings.ToLower(strings.TrimSpace(part.Type)) {
		case "text":
			items = appendRichText(items, part.Text)
		case "markdown":
			items = appendRichText(items, part.Markdown)
		case "image":
			url := strings.TrimSpace(part.URL)
			if url == "" {
				continue
			}
			items = appendRichText(items, "!["+escapeMarkdownImageAlt(part.Alt)+"]("+url+")")
		}
	}
	return strings.TrimSpace(strings.Join(items, ""))
}

func RichMessageToCQ(message types.RichMessage) string {
	items := make([]string, 0, len(message.Parts))
	for _, part := range message.Parts {
		switch strings.ToLower(strings.TrimSpace(part.Type)) {
		case "text":
			items = appendRichText(items, escapeCQText(part.Text))
		case "markdown":
			items = appendRichText(items, escapeCQText(richMarkdownPlainText(part.Markdown)))
		case "image":
			url := strings.TrimSpace(part.URL)
			if url == "" {
				continue
			}
			items = appendRichText(items, "[CQ:image,file="+escapeCQParam(url)+"]")
		}
	}
	return strings.TrimSpace(strings.Join(items, ""))
}

func RichMessageHasImage(message types.RichMessage) bool {
	for _, part := range message.Parts {
		if strings.EqualFold(strings.TrimSpace(part.Type), "image") && strings.TrimSpace(part.URL) != "" {
			return true
		}
	}
	return false
}

func RichMessageParts(message types.RichMessage) []types.RichMessagePart {
	parts := make([]types.RichMessagePart, 0, len(message.Parts))
	for _, part := range message.Parts {
		part.Type = strings.ToLower(strings.TrimSpace(part.Type))
		part.Text = normalizeNewlines(part.Text)
		part.Markdown = normalizeNewlines(part.Markdown)
		part.URL = strings.TrimSpace(part.URL)
		part.Alt = strings.TrimSpace(part.Alt)
		switch part.Type {
		case "text":
			if part.Text != "" {
				parts = append(parts, part)
			}
		case "markdown":
			if part.Markdown != "" {
				parts = append(parts, part)
			}
		case "image":
			if part.URL != "" {
				parts = append(parts, part)
			}
		}
	}
	return parts
}

func appendRichText(items []string, text string) []string {
	text = normalizeNewlines(text)
	if text == "" {
		return items
	}
	return append(items, text)
}

func richImagePlainText(part types.RichMessagePart) string {
	url := strings.TrimSpace(part.URL)
	alt := strings.TrimSpace(part.Alt)
	if alt == "" {
		return url
	}
	if url == "" {
		return alt
	}
	return alt + " (" + url + ")"
}

func richMarkdownPlainText(markdown string) string {
	text := MarkdownToPlainText(markdown)
	if text == "" {
		return text
	}
	if strings.HasPrefix(markdown, "\n") || strings.HasPrefix(markdown, "\r") {
		return "\n" + text
	}
	return text
}

func escapeMarkdownImageAlt(text string) string {
	text = strings.ReplaceAll(text, "\\", "\\\\")
	text = strings.ReplaceAll(text, "]", "\\]")
	return text
}

func escapeCQText(text string) string {
	text = strings.ReplaceAll(text, "&", "&amp;")
	text = strings.ReplaceAll(text, "[", "&#91;")
	text = strings.ReplaceAll(text, "]", "&#93;")
	return text
}

func escapeCQParam(text string) string {
	text = escapeCQText(text)
	text = strings.ReplaceAll(text, ",", "&#44;")
	return text
}

func normalizeNewlines(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	return text
}
