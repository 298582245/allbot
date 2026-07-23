package types

import "regexp"

type Message struct {
	ID        string
	Platform  string
	AdapterID string
	UserID    string
	GroupID   string
	Content   string
	Metadata  map[string]string
}

type ButtonOption struct {
	Text   string `json:"text"`
	Value  string `json:"value"`
	UserID string `json:"user_id,omitempty"`
}

type RichMessagePart struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	Markdown string `json:"markdown,omitempty"`
	URL      string `json:"url,omitempty"`
	Alt      string `json:"alt,omitempty"`
}

type RichMessage struct {
	Parts        []RichMessagePart `json:"parts"`
	FallbackText string            `json:"fallback_text,omitempty"`
	Prefer       string            `json:"prefer,omitempty"`
}

type AccessControlConfig struct {
	InheritSystem     bool     `json:"inherit_system"`
	WhitelistGroups   []string `json:"whitelist_groups"`
	BlockedGroups     []string `json:"blocked_groups"`
	WhitelistUserIDs  []string `json:"whitelist_user_ids"`
	BlockedUserIDs    []string `json:"blocked_user_ids"`
	WhitelistUnionIDs []string `json:"whitelist_union_ids"`
	BlockedUnionIDs   []string `json:"blocked_union_ids"`
}

type Plugin struct {
	ID                string
	Name              string
	Version           string
	Runtime           string
	RuntimeProfile    string
	Entry             string
	Platforms         []string
	AllowedAdapterIDs []string
	Priority          int
	Pinned            bool
	Trigger           string
	TriggerRegex      *regexp.Regexp
	Order             int
	Enabled           bool
	UserConfig        map[string]interface{}
	AccessControl     AccessControlConfig
	OpenAPI           OpenAPIConfig
	ScriptEnv         ScriptEnvConfig
	WebUI             PluginWebUIConfig
	WebChat           PluginWebChatConfig
	Template          string
	TemplateVersion   string
	TemplateMetadata  map[string]interface{}
	TemplateSource    map[string]interface{}
}

type OpenAPIConfig struct {
	Enabled        bool   `json:"enabled"`
	Path           string `json:"path"`
	Method         string `json:"method"`
	Token          string `json:"token"`
	Runtime        string `json:"runtime,omitempty"`
	RuntimeProfile string `json:"runtime_profile,omitempty"`
}

type ScriptEnvConfig struct {
	Enabled bool     `json:"enabled"`
	Names   []string `json:"names"`
}

type PluginWebUIConfig struct {
	Enabled bool   `json:"enabled"`
	Title   string `json:"title"`
	Entry   string `json:"entry"`
	Icon    string `json:"icon,omitempty"`
	Order   int    `json:"order,omitempty"`
}

type PluginWebChatQuickAction struct {
	Label  string `json:"label"`
	Text   string `json:"text"`
	Action string `json:"action,omitempty"`
}

type PluginWebChatConfig struct {
	Enabled      *bool                      `json:"enabled,omitempty"`
	Title        string                     `json:"title"`
	Description  string                     `json:"description"`
	Placeholder  string                     `json:"placeholder"`
	EntryText    string                     `json:"entry_text"`
	QuickActions []PluginWebChatQuickAction `json:"quick_actions"`
	Keywords     []string                   `json:"keywords"`
}

type OpenAPIEndpoint struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Path           string    `json:"path"`
	Method         string    `json:"method"`
	Enabled        bool      `json:"enabled"`
	Token          string    `json:"token"`
	Runtime        string    `json:"runtime"`
	RuntimeProfile string    `json:"runtime_profile,omitempty"`
	Entry          string    `json:"entry"`
	Description    string    `json:"description"`
	Builtin        string    `json:"builtin,omitempty"`
	IPWhitelist    *[]string `json:"ip_whitelist,omitempty"`
}

type OpenAPIRequest struct {
	Method       string                 `json:"method"`
	Path         string                 `json:"path"`
	RawPath      string                 `json:"raw_path"`
	Query        map[string][]string    `json:"query"`
	Headers      map[string][]string    `json:"headers"`
	Body         string                 `json:"body"`
	JSON         map[string]interface{} `json:"json,omitempty"`
	Form         map[string][]string    `json:"form,omitempty"`
	TokenSources map[string]string      `json:"token_sources"`
	ClientIP     string                 `json:"client_ip"`
}

type OpenAPIResponse struct {
	Status  int                    `json:"status"`
	Headers map[string]string      `json:"headers"`
	Body    string                 `json:"body"`
	JSON    interface{}            `json:"json,omitempty"`
	Data    map[string]interface{} `json:"data,omitempty"`
}

type PluginUserConfigField struct {
	Key         string      `json:"key"`
	Label       string      `json:"label"`
	Type        string      `json:"type"`
	Required    bool        `json:"required,omitempty"`
	Default     interface{} `json:"default,omitempty"`
	Placeholder string      `json:"placeholder,omitempty"`
	Description string      `json:"description,omitempty"`
}

type PluginConfig struct {
	Name              string                  `json:"name"`
	Version           string                  `json:"version"`
	Runtime           string                  `json:"runtime"`
	RuntimeProfile    string                  `json:"runtime_profile,omitempty"`
	Entry             string                  `json:"entry"`
	Platforms         []string                `json:"platforms"`
	AllowedAdapterIDs []string                `json:"allowed_adapter_ids,omitempty"`
	Priority          int                     `json:"priority"`
	Pinned            bool                    `json:"pinned"`
	Trigger           string                  `json:"trigger"`
	Enabled           bool                    `json:"enabled"`
	Dependencies      map[string]string       `json:"dependencies"`
	UserConfigSchema  []PluginUserConfigField `json:"user_config_schema,omitempty"`
	UserConfig        map[string]interface{}  `json:"user_config,omitempty"`
	AccessControl     *AccessControlConfig    `json:"access_control,omitempty"`
	OpenAPI           OpenAPIConfig           `json:"open_api,omitempty"`
	ScriptEnv         ScriptEnvConfig         `json:"script_env,omitempty"`
	WebUI             PluginWebUIConfig       `json:"web_ui,omitempty"`
	WebChat           PluginWebChatConfig     `json:"web_chat,omitempty"`
	Template          string                  `json:"template,omitempty"`
	TemplateVersion   string                  `json:"template_version,omitempty"`
	TemplateMetadata  map[string]interface{}  `json:"template_metadata,omitempty"`
	TemplateSource    map[string]interface{}  `json:"template_source,omitempty"`
}
