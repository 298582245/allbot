package web

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/allbot/allbot/core/updater"
	"github.com/allbot/allbot/core/version"
)

type fakeReleaseClient struct {
	release *updater.ReleaseInfo
	err     error
}

func (f fakeReleaseClient) LatestRelease(ctx context.Context) (*updater.ReleaseInfo, error) {
	return f.release, f.err
}

func TestHandleSystemUpdateDetectsNewVersion(t *testing.T) {
	withVersionValues(t, "v1.0.0", "abc123", "2026-06-03T10:00:00+08:00")
	server := &Server{}
	server.SetReleaseClient(fakeReleaseClient{release: &updater.ReleaseInfo{
		Version: "v1.0.1",
		Name:    "v1.0.1",
		Body:    "修复问题",
		URL:     "https://github.com/298582245/allbot/releases/tag/v1.0.1",
		Assets: []updater.ReleaseAsset{
			{Name: "allbot-windows-amd64.exe", DownloadURL: "https://example.com/allbot.exe", Size: 123},
			{Name: "checksums-v1.0.1.txt", DownloadURL: "https://example.com/checksums-v1.0.1.txt", Size: 64},
		},
	}})

	response := performSystemUpdateRequest(t, server, http.MethodGet)

	if response.CurrentVersion != "v1.0.0" || response.DisplayVersion != "AllBot v1.0.0" {
		t.Fatalf("current version = %q, display = %q", response.CurrentVersion, response.DisplayVersion)
	}
	if response.Commit != "abc123" || response.BuildTime != "2026-06-03T10:00:00+08:00" || response.GoVersion == "" {
		t.Fatalf("build info = commit %q, buildTime %q, goVersion %q", response.Commit, response.BuildTime, response.GoVersion)
	}
	if !response.HasUpdate || response.LatestVersion != "v1.0.1" {
		t.Fatalf("hasUpdate = %v, latestVersion = %q", response.HasUpdate, response.LatestVersion)
	}
	if response.ReleaseName != "v1.0.1" || response.ReleaseBody != "修复问题" || response.ReleaseURL == "" {
		t.Fatalf("release info = %#v", response)
	}
	if len(response.Assets) != 2 || response.Assets[0].DownloadURL != "https://example.com/allbot.exe" || response.Assets[0].Size != 123 {
		t.Fatalf("assets = %#v", response.Assets)
	}
	if !response.UpgradeSupported || !strings.Contains(response.UpgradeMessage, "可一键升级") {
		t.Fatalf("upgrade = %v, message = %q", response.UpgradeSupported, response.UpgradeMessage)
	}
	if response.MatchedAsset.Name != "allbot-windows-amd64.exe" {
		t.Fatalf("matched asset = %#v", response.MatchedAsset)
	}
	if response.ChecksumAsset.Name != "checksums-v1.0.1.txt" {
		t.Fatalf("checksum asset = %#v", response.ChecksumAsset)
	}
	if response.Error != "" || !strings.Contains(response.Message, "发现新版本") {
		t.Fatalf("error = %q, message = %q", response.Error, response.Message)
	}
}

func TestHandleSystemUpdateReportsUnsupportedWhenChecksumMissing(t *testing.T) {
	withVersionValues(t, "v1.0.0", "abc123", "build")
	server := &Server{}
	server.SetReleaseClient(fakeReleaseClient{release: &updater.ReleaseInfo{
		Version: "v1.0.1",
		Name:    "v1.0.1",
		Assets:  []updater.ReleaseAsset{{Name: "allbot-windows-amd64.exe", DownloadURL: "https://example.com/allbot.exe", Size: 123}},
	}})

	response := performSystemUpdateRequest(t, server, http.MethodGet)

	if !response.HasUpdate {
		t.Fatal("expected update")
	}
	if response.UpgradeSupported || !strings.Contains(response.UpgradeMessage, "校验文件") {
		t.Fatalf("upgrade = %v, message = %q", response.UpgradeSupported, response.UpgradeMessage)
	}
}

func TestHandleSystemUpdateReportsUnsupportedWhenAssetMissing(t *testing.T) {
	withVersionValues(t, "v1.0.0", "abc123", "build")
	server := &Server{}
	server.SetReleaseClient(fakeReleaseClient{release: &updater.ReleaseInfo{
		Version: "v1.0.1",
		Name:    "v1.0.1",
		Assets:  []updater.ReleaseAsset{{Name: "allbot-linux-arm64", DownloadURL: "https://example.com/allbot", Size: 123}},
	}})

	response := performSystemUpdateRequest(t, server, http.MethodGet)

	if !response.HasUpdate {
		t.Fatal("expected update")
	}
	if response.UpgradeSupported || !strings.Contains(response.UpgradeMessage, "未找到当前平台") {
		t.Fatalf("upgrade = %v, message = %q", response.UpgradeSupported, response.UpgradeMessage)
	}
}

func TestHandleSystemUpdateReportsLatestVersion(t *testing.T) {
	withVersionValues(t, "v1.0.1", "abc123", "build")
	server := &Server{}
	server.SetReleaseClient(fakeReleaseClient{release: &updater.ReleaseInfo{Version: "v1.0.1", Name: "v1.0.1"}})

	response := performSystemUpdateRequest(t, server, http.MethodGet)

	if response.HasUpdate {
		t.Fatalf("hasUpdate = true")
	}
	if response.Error != "" || response.Message != "当前已是最新版本。" {
		t.Fatalf("error = %q, message = %q", response.Error, response.Message)
	}
}

func TestHandleSystemUpdateKeepsCurrentInfoOnReleaseError(t *testing.T) {
	withVersionValues(t, "v1.0.0", "abc123", "build")
	server := &Server{}
	server.SetReleaseClient(fakeReleaseClient{err: errors.New("网络失败")})

	response := performSystemUpdateRequest(t, server, http.MethodGet)

	if response.CurrentVersion != "v1.0.0" || response.DisplayVersion != "AllBot v1.0.0" {
		t.Fatalf("current info = %#v", response)
	}
	if response.HasUpdate || response.LatestVersion != "" {
		t.Fatalf("hasUpdate = %v, latestVersion = %q", response.HasUpdate, response.LatestVersion)
	}
	if response.Error != "网络失败" || !strings.Contains(response.Message, "检查更新失败") {
		t.Fatalf("error = %q, message = %q", response.Error, response.Message)
	}
}

func TestHandleSystemUpdateHandlesMissingRelease(t *testing.T) {
	withVersionValues(t, "v1.0.0", "abc123", "build")
	server := &Server{}
	server.SetReleaseClient(fakeReleaseClient{})

	response := performSystemUpdateRequest(t, server, http.MethodGet)

	if response.HasUpdate {
		t.Fatalf("hasUpdate = true")
	}
	if response.Error != "未检测到 GitHub Release" || !strings.Contains(response.Message, "未检测到 GitHub Release") {
		t.Fatalf("error = %q, message = %q", response.Error, response.Message)
	}
}

func TestHandleSystemUpdateReturnsReleaseInfoWhenVersionInvalid(t *testing.T) {
	withVersionValues(t, "v1.0.0", "abc123", "build")
	server := &Server{}
	server.SetReleaseClient(fakeReleaseClient{release: &updater.ReleaseInfo{Version: "v1.0.1-beta", Name: "预发布", Body: "测试版本"}})

	response := performSystemUpdateRequest(t, server, http.MethodGet)

	if response.HasUpdate {
		t.Fatalf("hasUpdate = true")
	}
	if response.LatestVersion != "v1.0.1-beta" || response.ReleaseName != "预发布" || response.ReleaseBody != "测试版本" {
		t.Fatalf("release info = %#v", response)
	}
	if !strings.Contains(response.Error, "最新版本无效") || !strings.Contains(response.Message, "版本比较失败") {
		t.Fatalf("error = %q, message = %q", response.Error, response.Message)
	}
}

func TestHandleSystemUpdateStatusDefault(t *testing.T) {
	server := &Server{}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/system/update/status", nil)

	server.handleSystemUpdateStatus(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}
	var response updater.UpgradeState
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if response.Status != updater.UpgradeStatusIdle {
		t.Fatalf("upgrade status = %#v", response)
	}
}

func TestHandleSystemUpgradeRejectsMissingPlatformAsset(t *testing.T) {
	withVersionValues(t, "v1.0.0", "abc123", "build")
	server := &Server{}
	server.SetReleaseClient(fakeReleaseClient{release: &updater.ReleaseInfo{
		Version: "v1.0.1",
		Name:    "v1.0.1",
		Assets:  []updater.ReleaseAsset{{Name: "allbot-linux-arm64", DownloadURL: "https://example.com/allbot", Size: 123}},
	}})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/system/update/upgrade", nil)

	server.handleSystemUpgrade(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	state := server.UpdateService().CurrentState()
	if state.Status != updater.UpgradeStatusFailed || !strings.Contains(state.Message, "未找到当前平台") {
		t.Fatalf("state = %#v", state)
	}
}

func TestHandleSystemUpdateRejectsNonGet(t *testing.T) {
	server := &Server{}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/system/update", nil)

	server.handleSystemUpdate(recorder, request)

	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d", recorder.Code)
	}
}

func performSystemUpdateRequest(t *testing.T, server *Server, method string) updateInfoResponse {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, "/api/system/update", nil)
	server.handleSystemUpdate(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var response updateInfoResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	return response
}

func withVersionValues(t *testing.T, current string, commit string, buildTime string) {
	t.Helper()
	oldVersion := version.Version
	oldCommit := version.Commit
	oldBuildTime := version.BuildTime
	version.Version = current
	version.Commit = commit
	version.BuildTime = buildTime
	t.Cleanup(func() {
		version.Version = oldVersion
		version.Commit = oldCommit
		version.BuildTime = oldBuildTime
	})
}
