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
