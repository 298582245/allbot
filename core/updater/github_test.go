package updater

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGitHubClientLatestRelease(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s", r.Method)
		}
		if r.Header.Get("User-Agent") != "AllBot-Updater" {
			t.Fatalf("User-Agent = %q", r.Header.Get("User-Agent"))
		}
		_, _ = w.Write([]byte(`{
			"tag_name":"v1.0.1",
			"name":"AllBot v1.0.1",
			"body":"## 更新内容\n- 修复问题",
			"html_url":"https://github.com/298582245/allbot/releases/tag/v1.0.1",
			"assets":[
				{"name":"allbot-windows-amd64.exe","browser_download_url":"https://example.com/allbot.exe","size":123}
			]
		}`))
	}))
	defer server.Close()

	client := &GitHubClient{HTTPClient: server.Client(), APIURL: server.URL}
	release, err := client.LatestRelease(context.Background())
	if err != nil {
		t.Fatalf("LatestRelease returned error: %v", err)
	}
	if release.Version != "v1.0.1" || release.Name != "AllBot v1.0.1" || !strings.Contains(release.Body, "修复问题") || release.URL == "" {
		t.Fatalf("release = %+v", release)
	}
	if len(release.Assets) != 1 || release.Assets[0].Name != "allbot-windows-amd64.exe" || release.Assets[0].DownloadURL == "" || release.Assets[0].Size != 123 {
		t.Fatalf("assets = %+v", release.Assets)
	}
}

func TestGitHubClientLatestReleaseNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer server.Close()

	client := &GitHubClient{HTTPClient: server.Client(), APIURL: server.URL}
	_, err := client.LatestRelease(context.Background())
	if err == nil || !strings.Contains(err.Error(), "未检测到 GitHub Release") {
		t.Fatalf("error = %v", err)
	}
}

func TestGitHubClientLatestReleaseHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "server error", http.StatusInternalServerError)
	}))
	defer server.Close()

	client := &GitHubClient{HTTPClient: server.Client(), APIURL: server.URL}
	_, err := client.LatestRelease(context.Background())
	if err == nil || !strings.Contains(err.Error(), "状态码 500") {
		t.Fatalf("error = %v", err)
	}
}

func TestGitHubClientLatestReleaseMissingTag(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"tag_name":""}`))
	}))
	defer server.Close()

	client := &GitHubClient{HTTPClient: server.Client(), APIURL: server.URL}
	_, err := client.LatestRelease(context.Background())
	if err == nil || !strings.Contains(err.Error(), "缺少 tag_name") {
		t.Fatalf("error = %v", err)
	}
}
