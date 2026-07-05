package builtin

import (
	"context"
	"strings"
	"time"

	"github.com/allbot/allbot/core/config"
	plugincore "github.com/allbot/allbot/core/plugin"
	"github.com/allbot/allbot/core/types"
	"github.com/allbot/allbot/core/updater"
)

type UpdateHandler interface {
	StartUpgrade(ctx context.Context) (updater.UpgradeState, error)
	CurrentState() updater.UpgradeState
}

type PluginAdminStore interface {
	GetAllPlugins() []*plugincore.PluginProcess
	TogglePlugin(pluginID string, enabled bool) error
	SavePluginAccessControl(pluginID string, accessControl types.AccessControlConfig) error
}

type ListenFunc func(msg *types.Message, timeout int) string

type ListenUntilFunc func(msg *types.Message, timeout int, done <-chan struct{}) string

type RestartHandler func(RestartRequest) error

type Context struct {
	Database       *config.Database
	Message        *types.Message
	Target         string
	StartTime      time.Time
	ReleaseClient  updater.ReleaseClient
	UpdateHandler  UpdateHandler
	PluginStore    PluginAdminStore
	RegisterPlugin func(*types.Plugin) error
	Listen         ListenFunc
	ListenUntil    ListenUntilFunc
	AdminCheck     func(platform, userID string) bool
	Reply          func(text string) error
	ReplyButtons   func(text string, buttons [][]types.ButtonOption) error
	SendImage      func(imageURL string) error
	SendRich       func(message types.RichMessage) error
	ReserveRestart func() (RestartHandler, bool)
	ReleaseRestart func()
	MessageKey     func(msg *types.Message) string
}

func (c *Context) SendText(text string) error {
	if c == nil || c.Reply == nil {
		return nil
	}
	return c.Reply(text)
}

func (c *Context) IsAdmin() bool {
	return c != nil && c.Message != nil && c.AdminCheck != nil && c.AdminCheck(c.Message.Platform, c.Message.UserID)
}

func (c *Context) ListenText(timeout int) string {
	if c == nil || c.Listen == nil || c.Message == nil {
		return ""
	}
	return strings.TrimSpace(c.Listen(c.Message, timeout))
}

func (c *Context) ListenUntilDone(timeout int, done <-chan struct{}) string {
	if c == nil || c.Message == nil {
		return ""
	}
	if c.ListenUntil != nil {
		return strings.TrimSpace(c.ListenUntil(c.Message, timeout, done))
	}
	if c.Listen == nil {
		return ""
	}
	ch := make(chan string, 1)
	go func() {
		ch <- c.Listen(c.Message, timeout)
	}()
	select {
	case value := <-ch:
		return strings.TrimSpace(value)
	case <-done:
		return ""
	}
}

func (c *Context) adapterID() string {
	if c == nil || c.Message == nil {
		return ""
	}
	if c.Message.AdapterID != "" {
		return c.Message.AdapterID
	}
	if c.Message.Metadata != nil {
		return c.Message.Metadata["adapter_id"]
	}
	return ""
}
