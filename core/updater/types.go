package updater

import "context"

type ReleaseClient interface {
	LatestRelease(ctx context.Context) (*ReleaseInfo, error)
}

type UpgradeNotification struct {
	MessageKey  string `json:"messageKey,omitempty"`
	Platform    string `json:"platform,omitempty"`
	AdapterID   string `json:"adapterId,omitempty"`
	UserID      string `json:"userId,omitempty"`
	GroupID     string `json:"groupId,omitempty"`
	Target      string `json:"target,omitempty"`
	StartedAtNS int64  `json:"startedAtNs,omitempty"`
}

type upgradeNotificationContextKey struct{}

func WithUpgradeNotification(ctx context.Context, notification UpgradeNotification) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, upgradeNotificationContextKey{}, notification)
}

func UpgradeNotificationFromContext(ctx context.Context) (UpgradeNotification, bool) {
	if ctx == nil {
		return UpgradeNotification{}, false
	}
	notification, ok := ctx.Value(upgradeNotificationContextKey{}).(UpgradeNotification)
	return notification, ok
}

type ReleaseInfo struct {
	Version string         `json:"version"`
	Name    string         `json:"name"`
	Body    string         `json:"body"`
	URL     string         `json:"url"`
	Assets  []ReleaseAsset `json:"assets"`
}

type ReleaseAsset struct {
	Name        string `json:"name"`
	DownloadURL string `json:"download_url"`
	Size        int64  `json:"size"`
}

type UpgradeStatus string

const (
	UpgradeStatusIdle        UpgradeStatus = "idle"
	UpgradeStatusDownloading UpgradeStatus = "downloading"
	UpgradeStatusRestarting  UpgradeStatus = "restarting"
	UpgradeStatusFailed      UpgradeStatus = "failed"
)

type UpgradeState struct {
	Status       UpgradeStatus `json:"status"`
	Message      string        `json:"message"`
	Error        string        `json:"error,omitempty"`
	Version      string        `json:"version,omitempty"`
	AssetName    string        `json:"assetName,omitempty"`
	DownloadedAt string        `json:"downloadedAt,omitempty"`
}
