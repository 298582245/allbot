package utils

import (
	"regexp"
	"strings"
)

var (
	markdownCodeBlockRegexp      = regexp.MustCompile("(?s)```[^\n]*\n?(.*?)```")
	markdownImageRegexp          = regexp.MustCompile(`!\[([^\]]*)\]\(([^)]+)\)`)
	markdownLinkRegexp           = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
	markdownInlineCodeRegexp     = regexp.MustCompile("`([^`]*)`")
	markdownBoldRegexp           = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	markdownItalicRegexp         = regexp.MustCompile(`\*([^*]+)\*`)
	markdownStrikeRegexp         = regexp.MustCompile(`~~([^~]+)~~`)
	markdownHeadingRegexp        = regexp.MustCompile(`^\s{0,3}#{1,6}\s+`)
	markdownQuoteRegexp          = regexp.MustCompile(`^\s{0,3}>\s?`)
	markdownUnorderedListRegexp  = regexp.MustCompile(`^\s*[-+*]\s+`)
	markdownOrderedListRegexp    = regexp.MustCompile(`^\s*\d+[.)]\s+`)
	markdownHorizontalRuleRegexp = regexp.MustCompile(`^\s*[-*_](?:\s*[-*_]){2,}\s*$`)
	markdownTableSeparatorRegexp = regexp.MustCompile(`^\s*\|?[\s|:-]+\|?\s*$`)
)

// MarkdownToPlainText 将常见 Markdown 标记转换为便于普通文本平台展示的内容。
func MarkdownToPlainText(markdown string) string {
	text := strings.ReplaceAll(markdown, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	text = markdownCodeBlockRegexp.ReplaceAllString(text, "$1")
	text = markdownImageRegexp.ReplaceAllStringFunc(text, func(match string) string {
		parts := markdownImageRegexp.FindStringSubmatch(match)
		if len(parts) != 3 {
			return match
		}
		alt := strings.TrimSpace(parts[1])
		url := strings.TrimSpace(parts[2])
		if alt == "" {
			return url
		}
		return alt + " (" + url + ")"
	})
	text = markdownLinkRegexp.ReplaceAllString(text, "$1 ($2)")
	text = markdownInlineCodeRegexp.ReplaceAllString(text, "$1")
	text = markdownBoldRegexp.ReplaceAllString(text, "$1")
	text = markdownItalicRegexp.ReplaceAllString(text, "$1")
	text = markdownStrikeRegexp.ReplaceAllString(text, "$1")

	lines := strings.Split(text, "\n")
	plainLines := make([]string, 0, len(lines))
	blank := false
	for _, line := range lines {
		line = markdownHeadingRegexp.ReplaceAllString(line, "")
		line = markdownQuoteRegexp.ReplaceAllString(line, "")
		line = markdownUnorderedListRegexp.ReplaceAllString(line, "")
		line = markdownOrderedListRegexp.ReplaceAllString(line, "")
		if markdownTableSeparatorRegexp.MatchString(line) {
			continue
		}
		if markdownHorizontalRuleRegexp.MatchString(line) {
			line = ""
		}
		line = strings.TrimSpace(strings.ReplaceAll(line, "|", " "))
		if line == "" {
			if blank {
				continue
			}
			blank = true
			plainLines = append(plainLines, "")
			continue
		}
		blank = false
		plainLines = append(plainLines, line)
	}
	return strings.TrimSpace(strings.Join(plainLines, "\n"))
}
