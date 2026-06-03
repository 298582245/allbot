package registry

import "github.com/allbot/allbot/core/adapter/_contract"

// ConfigField 描述适配器配置表单中的一个字段。
type ConfigField struct {
	Key         string      `json:"key"`
	Label       string      `json:"label"`
	Type        string      `json:"type"`
	Required    bool        `json:"required"`
	Placeholder string      `json:"placeholder,omitempty"`
	Default     interface{} `json:"default,omitempty"`
	Help        string      `json:"help,omitempty"`
}

// Capabilities 描述适配器支持的消息能力。
type Capabilities struct {
	SendText       bool `json:"send_text"`
	SendImage      bool `json:"send_image"`
	SendFile       bool `json:"send_file"`
	PrivateMessage bool `json:"private_message"`
	GroupMessage   bool `json:"group_message"`
	Mention        bool `json:"mention"`
}

// Descriptor 描述一个可注册的平台适配器。
type Descriptor struct {
	Platform     string        `json:"platform"`
	DisplayName  string        `json:"display_name"`
	Description  string        `json:"description"`
	ConfigSchema []ConfigField `json:"config_schema"`
	Capabilities Capabilities  `json:"capabilities"`

	ParseConfig func(raw string) (interface{}, error)              `json:"-"`
	NewAdapter  func(config interface{}) (contract.Adapter, error) `json:"-"`
}
