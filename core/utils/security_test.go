package utils

import (
	"errors"
	"testing"
)

func TestMaskSensitiveConfig(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Telegram bot_token",
			input:    `{"bot_token":"7875955124:AAHG0vzsGrrTwTKVWrWSAtQok6bjcZfXFaY","proxy_url":"http://127.0.0.1:7890"}`,
			expected: `{"bot_token":"****XFaY","proxy_url":"http://127.0.0.1:7890"}`,
		},
		{
			name:     "WeChat app_secret",
			input:    `{"app_id":"wx1234567890","app_secret":"abcdef1234567890abcdef1234567890"}`,
			expected: `{"app_id":"wx1234567890","app_secret":"****7890"}`,
		},
		{
			name:     "Multiple sensitive fields",
			input:    `{"api_key":"sk-1234567890abcdef","password":"mypassword123","username":"admin"}`,
			expected: `{"api_key":"****cdef","password":"****d123","username":"admin"}`,
		},
		{
			name:     "Short token",
			input:    `{"token":"abc"}`,
			expected: `{"token":"****"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MaskSensitiveConfig(tt.input)
			if result != tt.expected {
				t.Errorf("MaskSensitiveConfig() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestMaskSensitiveError(t *testing.T) {
	tests := []struct {
		name     string
		input    error
		expected string
	}{
		{
			name:     "Telegram bot token in URL",
			input:    errors.New(`HTTP请求失败: Get "https://api.telegram.org/bot7875955124:AAHG0vzsGrrTwTKVWrWSAtQok6bjcZfXFaY/getUpdates?offset=1&timeout=30": context deadline exceeded`),
			expected: `HTTP请求失败: Get "https://api.telegram.org/bot7875955124:****XFaY/getUpdates?offset=1&timeout=30": context deadline exceeded`,
		},
		{
			name:     "Bearer token",
			input:    errors.New(`Authorization failed: Bearer sk-1234567890abcdefghijklmnop`),
			expected: `Authorization failed: Bearer ****mnop`,
		},
		{
			name:     "No sensitive data",
			input:    errors.New(`Connection timeout`),
			expected: `Connection timeout`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MaskSensitiveError(tt.input)
			if result != tt.expected {
				t.Errorf("MaskSensitiveError() = %v, want %v", result, tt.expected)
			}
		})
	}
}
