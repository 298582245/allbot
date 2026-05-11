package utils

import (
	"encoding/json"
	"regexp"
	"strings"
)

// MaskSensitiveConfig 脱敏配置信息
// 将敏感字段（token、secret、password等）替换为掩码形式
func MaskSensitiveConfig(configJSON string) string {
	var config map[string]interface{}
	if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
		return configJSON // 解析失败，返回原值
	}

	// 需要脱敏的字段名（不区分大小写）
	sensitiveFields := []string{
		"token", "bot_token", "access_token", "refresh_token",
		"secret", "app_secret", "client_secret",
		"password", "passwd", "pwd",
		"key", "api_key", "private_key",
	}

	// 遍历配置，脱敏敏感字段
	for key, value := range config {
		keyLower := strings.ToLower(key)
		for _, field := range sensitiveFields {
			if strings.Contains(keyLower, field) {
				if strValue, ok := value.(string); ok && strValue != "" {
					config[key] = maskString(strValue)
				}
				break
			}
		}
	}

	// 序列化回 JSON
	masked, err := json.Marshal(config)
	if err != nil {
		return configJSON
	}
	return string(masked)
}

// maskString 脱敏字符串，只保留最后4位
func maskString(s string) string {
	if len(s) <= 4 {
		return "****"
	}
	return "****" + s[len(s)-4:]
}

// MaskSensitiveError 脱敏错误信息中的敏感数据
// 主要用于脱敏 URL 中的 token
func MaskSensitiveError(err error) string {
	if err == nil {
		return ""
	}

	errMsg := err.Error()

	// 匹配 Telegram bot token 格式：/botXXXXXXXXXX:XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX/
	// 格式：数字:字母数字混合
	botTokenRegex := regexp.MustCompile(`/bot(\d+):([A-Za-z0-9_-]+)/`)
	errMsg = botTokenRegex.ReplaceAllStringFunc(errMsg, func(match string) string {
		// 提取 token 部分
		parts := botTokenRegex.FindStringSubmatch(match)
		if len(parts) == 3 {
			botID := parts[1]
			token := parts[2]
			// 只保留最后4位
			maskedToken := maskString(token)
			return "/bot" + botID + ":" + maskedToken + "/"
		}
		return match
	})

	// 匹配其他可能的 token 格式（Bearer token 等）
	bearerRegex := regexp.MustCompile(`Bearer\s+([A-Za-z0-9_\-\.]+)`)
	errMsg = bearerRegex.ReplaceAllStringFunc(errMsg, func(match string) string {
		parts := bearerRegex.FindStringSubmatch(match)
		if len(parts) == 2 {
			return "Bearer " + maskString(parts[1])
		}
		return match
	})

	return errMsg
}
