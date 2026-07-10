package config

import "time"

// AdapterConfig 表示一个机器人账号的适配器配置，同一平台可以有多个账号。
type AdapterConfig struct {
	ID          int64     `json:"id"`
	Platform    string    `json:"platform"`
	Remark      string    `json:"remark"`
	Description string    `json:"description"`
	Enabled     bool      `json:"enabled"`
	Pinned      bool      `json:"pinned"`
	Config      string    `json:"config"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type QQConfig struct {
	ServerURL   string `json:"server_url"`
	AccessToken string `json:"access_token,omitempty"`
}

type QQOfficeConfig struct {
	AppID        string `json:"app_id"`
	ClientSecret string `json:"client_secret"`
	APIBaseURL   string `json:"api_base_url,omitempty"`
	TokenURL     string `json:"token_url,omitempty"`
}

type WeChatConfig struct {
	AppID     string `json:"app_id"`
	AppSecret string `json:"app_secret"`
}

type TelegramConfig struct {
	BotToken string `json:"bot_token"`
	ProxyURL string `json:"proxy_url,omitempty"`
}

type UserSummary struct {
	UnionID            string    `json:"union_id"`
	Points             int64     `json:"points"`
	Disabled           bool      `json:"disabled"`
	AccountCount       int64     `json:"account_count"`
	PlatformCount      int64     `json:"platform_count"`
	Platforms          []string  `json:"platforms"`
	DuplicatePlatforms []string  `json:"duplicate_platforms"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type UserDetail struct {
	UserSummary
	Accounts []*UserAccountSummary `json:"accounts"`
}

type UserQuery struct {
	Keyword  string
	Platform string
	Disabled *bool
	Limit    int
	Offset   int
}

type UserAccountSummary struct {
	ID                int64     `json:"id"`
	Platform          string    `json:"platform"`
	UserID            string    `json:"user_id"`
	UnionID           string    `json:"union_id"`
	Points            int64     `json:"points"`
	Disabled          bool      `json:"disabled"`
	DuplicatePlatform bool      `json:"duplicate_platform"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type PointAdjustment struct {
	UnionID      string            `json:"union_id"`
	Delta        int64             `json:"delta"`
	BalanceAfter int64             `json:"balance_after"`
	Transaction  *PointTransaction `json:"transaction"`
}

type UserAccount struct {
	ID        int64     `json:"id"`
	Platform  string    `json:"platform"`
	UserID    string    `json:"user_id"`
	UnionID   string    `json:"union_id"`
	Points    int64     `json:"points"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type UserBindCode struct {
	Code      string    `json:"code"`
	Platform  string    `json:"platform"`
	UserID    string    `json:"user_id"`
	UnionID   string    `json:"union_id"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}
