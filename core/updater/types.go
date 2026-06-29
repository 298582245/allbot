package updater

import "context"

type ReleaseClient interface {
	LatestRelease(ctx context.Context) (*ReleaseInfo, error)
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
