package web

import (
	"context"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/allbot/allbot/core/updater"
	"github.com/allbot/allbot/core/version"
)

type updateInfoResponse struct {
	CurrentVersion   string                `json:"currentVersion"`
	DisplayVersion   string                `json:"displayVersion"`
	Commit           string                `json:"commit"`
	BuildTime        string                `json:"buildTime"`
	GoVersion        string                `json:"goVersion"`
	LatestVersion    string                `json:"latestVersion"`
	HasUpdate        bool                  `json:"hasUpdate"`
	ReleaseName      string                `json:"releaseName"`
	ReleaseBody      string                `json:"releaseBody"`
	ReleaseURL       string                `json:"releaseUrl"`
	Assets           []updateAssetResponse `json:"assets"`
	Error            string                `json:"error"`
	Message          string                `json:"message"`
	UpgradeSupported bool                  `json:"upgradeSupported"`
	UpgradeMessage   string                `json:"upgradeMessage"`
}

type updateAssetResponse struct {
	Name        string `json:"name"`
	DownloadURL string `json:"downloadUrl"`
	Size        int64  `json:"size"`
}

func (s *Server) SetReleaseClient(client updater.ReleaseClient) {
	if client == nil {
		s.releaseClient = updater.NewGitHubClient()
		return
	}
	s.releaseClient = client
}

func (s *Server) handleSystemUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	response := currentUpdateInfo()
	client := s.releaseClient
	if client == nil {
		client = updater.NewGitHubClient()
	}

	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()

	release, err := client.LatestRelease(ctx)
	if err != nil {
		response.Error = err.Error()
		response.Message = "检查更新失败: " + err.Error()
		s.jsonResponse(w, response)
		return
	}
	if release == nil {
		response.Error = "未检测到 GitHub Release"
		response.Message = "未检测到 GitHub Release。"
		s.jsonResponse(w, response)
		return
	}

	fillReleaseInfo(&response, release)
	compare, err := updater.CompareVersion(response.CurrentVersion, response.LatestVersion)
	if err != nil {
		response.Error = err.Error()
		response.Message = "版本比较失败: " + err.Error()
		s.jsonResponse(w, response)
		return
	}
	if compare < 0 {
		response.HasUpdate = true
		response.Message = "发现新版本，请前往 Release 手动更新。"
	} else {
		response.Message = "当前已是最新版本。"
	}
	s.jsonResponse(w, response)
}

func currentUpdateInfo() updateInfoResponse {
	return updateInfoResponse{
		CurrentVersion:   normalizeVersion(version.Version),
		DisplayVersion:   version.DisplayVersion(),
		Commit:           fallbackUnknown(version.Commit),
		BuildTime:        fallbackUnknown(version.BuildTime),
		GoVersion:        runtime.Version(),
		Assets:           []updateAssetResponse{},
		Message:          "点击检查更新获取最新 Release 信息。",
		UpgradeSupported: false,
		UpgradeMessage:   "当前版本暂不支持 Web 一键升级，请前往 GitHub Release 手动下载更新。",
	}
}

func fillReleaseInfo(response *updateInfoResponse, release *updater.ReleaseInfo) {
	response.LatestVersion = strings.TrimSpace(release.Version)
	response.ReleaseName = strings.TrimSpace(release.Name)
	response.ReleaseBody = release.Body
	response.ReleaseURL = strings.TrimSpace(release.URL)
	response.Assets = make([]updateAssetResponse, 0, len(release.Assets))
	for _, asset := range release.Assets {
		response.Assets = append(response.Assets, updateAssetResponse{Name: asset.Name, DownloadURL: asset.DownloadURL, Size: asset.Size})
	}
}

func normalizeVersion(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown"
	}
	return value
}

func fallbackUnknown(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown"
	}
	return value
}
