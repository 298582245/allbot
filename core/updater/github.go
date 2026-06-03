package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const DefaultLatestReleaseURL = "https://api.github.com/repos/298582245/allbot/releases/latest"

type GitHubClient struct {
	HTTPClient *http.Client
	APIURL     string
}

func NewGitHubClient() *GitHubClient {
	return &GitHubClient{
		HTTPClient: &http.Client{Timeout: 10 * time.Second},
		APIURL:     DefaultLatestReleaseURL,
	}
}

func (c *GitHubClient) LatestRelease(ctx context.Context) (*ReleaseInfo, error) {
	apiURL := strings.TrimSpace(c.APIURL)
	if apiURL == "" {
		apiURL = DefaultLatestReleaseURL
	}
	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "AllBot-Updater")

	response, err := httpClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("未检测到 GitHub Release")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("GitHub Release API 状态码 %d", response.StatusCode)
	}

	var payload githubReleasePayload
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return nil, err
	}
	version := strings.TrimSpace(payload.TagName)
	if version == "" {
		return nil, fmt.Errorf("GitHub Release 响应缺少 tag_name")
	}
	assets := make([]ReleaseAsset, 0, len(payload.Assets))
	for _, item := range payload.Assets {
		assets = append(assets, ReleaseAsset{Name: item.Name, DownloadURL: item.BrowserDownloadURL, Size: item.Size})
	}
	return &ReleaseInfo{Version: version, Name: payload.Name, Body: payload.Body, URL: payload.HTMLURL, Assets: assets}, nil
}

type githubReleasePayload struct {
	TagName string               `json:"tag_name"`
	Name    string               `json:"name"`
	Body    string               `json:"body"`
	HTMLURL string               `json:"html_url"`
	Assets  []githubReleaseAsset `json:"assets"`
}

type githubReleaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}
