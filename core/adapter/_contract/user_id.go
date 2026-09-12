package contract

import (
	"regexp"
	"strings"
)

var (
	qqOfficialMentionPattern       = regexp.MustCompile(`^<@!?([^>\s]+)>$`)
	telegramMentionLinkPattern     = regexp.MustCompile(`(?i)^<a\s+href=["']tg://user\?id=([^"'&]+)["'][^>]*>.*</a>$`)
	telegramMarkdownMentionPattern = regexp.MustCompile(`(?i)^\[[^\]]*\]\(tg://user\?id=([^)&\s]+)\)$`)
)

// NormalizeUserID 将常见平台的 @ 用户格式解析为适配器 API 所需的用户 ID。
// 未识别的格式会原样返回，便于不同适配器继续处理自己的用户标识格式。
func NormalizeUserID(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	if strings.HasPrefix(strings.ToLower(value), "[cq:at,") && strings.HasSuffix(value, "]") {
		body := strings.TrimSuffix(value[len("[CQ:at,"):], "]")
		for _, field := range strings.Split(body, ",") {
			key, rawValue, ok := strings.Cut(field, "=")
			if ok && strings.EqualFold(strings.TrimSpace(key), "qq") {
				if userID := strings.TrimSpace(rawValue); userID != "" {
					return userID
				}
			}
		}
	}

	if match := qqOfficialMentionPattern.FindStringSubmatch(value); len(match) == 2 {
		return strings.TrimSpace(match[1])
	}
	if match := telegramMentionLinkPattern.FindStringSubmatch(value); len(match) == 2 {
		return strings.TrimSpace(match[1])
	}
	if match := telegramMarkdownMentionPattern.FindStringSubmatch(value); len(match) == 2 {
		return strings.TrimSpace(match[1])
	}
	if strings.HasPrefix(value, "tg://user?id=") {
		return strings.TrimSpace(strings.TrimPrefix(value, "tg://user?id="))
	}
	if strings.HasPrefix(value, "@") {
		return strings.TrimSpace(strings.TrimPrefix(value, "@"))
	}
	return value
}
