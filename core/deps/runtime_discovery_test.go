package deps

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestNodeDownloadCandidatesParseIndexAndMirror(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/dist/index.json" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`[
			{"version":"v20.11.1","files":["linux-arm64","win-x64-zip"],"lts":"Iron"},
			{"version":"v18.20.8","files":["linux-x64","win-x64-zip"],"lts":"Hydrogen"},
			{"version":"v16.20.2","files":["linux-arm64"],"lts":false}
		]`))
	}))
	defer server.Close()
	downloader := NewHTTPRuntimeDownloader(t.TempDir())
	result, err := downloader.listNodeDownloadCandidates(server.Client(), RuntimeDownloadCandidateQuery{Runtime: "nodejs", Architecture: "linux-x64", Q: "18", Limit: 10}, RuntimeDownloadOptions{NodeMirrorURL: server.URL + "/dist"})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Candidates) != 1 || result.Candidates[0].Version != "18.20.8" || result.Candidates[0].FileName != "node-v18.20.8-linux-x64.tar.gz" {
		t.Fatalf("unexpected candidates: %#v", result)
	}
	winResult, err := downloader.listNodeDownloadCandidates(server.Client(), RuntimeDownloadCandidateQuery{Runtime: "nodejs", Architecture: "win-x64", Limit: 10}, RuntimeDownloadOptions{NodeMirrorURL: server.URL + "/dist"})
	if err != nil {
		t.Fatal(err)
	}
	if len(winResult.Candidates) != 2 || winResult.Candidates[0].FileName != "node-v20.11.1-win-x64.zip" {
		t.Fatalf("unexpected win candidates: %#v", winResult)
	}
}

func TestWindowsPythonDownloadCandidatesSupportExpandedAndPagedNuGet(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/expanded.json":
			_, _ = w.Write([]byte(`{"items":[{"items":[{"catalogEntry":{"version":"3.10.11"}},{"catalogEntry":{"version":"3.11.9"}}]}]}`))
		case "/root.json":
			_, _ = w.Write([]byte(`{"items":[{"@id":"` + server.URL + `/page.json"}]}`))
		case "/page.json":
			_, _ = w.Write([]byte(`{"items":[{"catalogEntry":{"version":"3.12.3"}},{"catalogEntry":{"version":"3.9.13"}}]}`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()
	downloader := NewHTTPRuntimeDownloader(t.TempDir())
	expanded, err := downloader.listWindowsPythonDownloadCandidates(server.Client(), RuntimeDownloadCandidateQuery{Runtime: "python", Architecture: "win-x64", Q: "3.11", Limit: 10}, RuntimeDownloadOptions{PythonMetadataURL: server.URL + "/expanded.json"})
	if err != nil {
		t.Fatal(err)
	}
	if len(expanded.Candidates) != 1 || expanded.Candidates[0].Version != "3.11.9" || expanded.Candidates[0].FileName != "python.3.11.9.nupkg" {
		t.Fatalf("unexpected expanded candidates: %#v", expanded)
	}
	paged, err := downloader.listWindowsPythonDownloadCandidates(server.Client(), RuntimeDownloadCandidateQuery{Runtime: "python", Architecture: "win-x64", Limit: 10}, RuntimeDownloadOptions{PythonMetadataURL: server.URL + "/root.json"})
	if err != nil {
		t.Fatal(err)
	}
	if len(paged.Candidates) != 2 || paged.Candidates[0].Version != "3.12.3" {
		t.Fatalf("unexpected paged candidates: %#v", paged)
	}
	if _, err := downloader.listWindowsPythonDownloadCandidates(server.Client(), RuntimeDownloadCandidateQuery{Runtime: "python", Architecture: "win-arm64", Limit: 10}, RuntimeDownloadOptions{PythonMetadataURL: server.URL + "/expanded.json"}); err == nil {
		t.Fatal("expected win-arm64 to be unsupported")
	}
}

func TestLinuxPythonStandaloneCandidatesAndFindAssetShareSelection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.RawQuery, "per_page=30") {
			t.Fatalf("expected per_page=30, got %s", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`[
			{"assets":[
				{"name":"cpython-3.11.13+20250601-x86_64-unknown-linux-gnu-install_only.tar.zst","browser_download_url":"https://github.com/a/zst"},
				{"name":"cpython-3.11.13+20250601-x86_64-unknown-linux-gnu-install_only.tar.gz","browser_download_url":"https://github.com/a/gz","digest":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
				{"name":"cpython-3.11.13+20250601-aarch64-unknown-linux-gnu-install_only.tar.gz","browser_download_url":"https://github.com/a/arm"},
				{"name":"cpython-3.10.14+20250601-x86_64-unknown-linux-gnu-pgo_full.tar.gz","browser_download_url":"https://github.com/a/full"},
				{"name":"cpython-3.10.14+20250601-x86_64-unknown-linux-gnu-install_only.tar.zst","browser_download_url":"https://github.com/a/310zst"}
			]}
		]`))
	}))
	defer server.Close()
	oldURL := pythonStandaloneReleasesURL
	pythonStandaloneReleasesURL = server.URL
	t.Cleanup(func() { pythonStandaloneReleasesURL = oldURL })
	client := server.Client()
	downloader := NewHTTPRuntimeDownloader(t.TempDir())
	result, err := downloader.listLinuxPythonDownloadCandidates(client, RuntimeDownloadCandidateQuery{Runtime: "python", Architecture: "linux-x64", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Candidates) != 2 || result.Candidates[0].Version != "3.11.13" || result.Candidates[0].AssetName != "cpython-3.11.13+20250601-x86_64-unknown-linux-gnu-install_only.tar.gz" || !result.Candidates[0].Preferred {
		t.Fatalf("unexpected linux candidates: %#v", result)
	}
	asset, err := findPythonStandaloneAsset(client, "3.11.13", "linux-x64")
	if err != nil {
		t.Fatal(err)
	}
	if asset.Name != result.Candidates[0].AssetName {
		t.Fatalf("find asset should share selection rule, got %#v result %#v", asset, result.Candidates[0])
	}
}

func TestLinuxManagedPythonExecutableUsesPython3(t *testing.T) {
	root := filepath.Join("runtime", "python")
	if managedRuntimeExecutableInRoot("python", "linux-x64", root) != filepath.Join(root, "bin", "python3") {
		t.Fatal("linux managed python executable path should use bin/python3")
	}
	manager := NewManager(t.TempDir())
	profiles, err := manager.SaveRuntimeProfiles([]RuntimeProfile{
		{ID: "node-default", Name: "默认 Node.js", Runtime: "nodejs", Executable: "node", Enabled: true, Default: true},
		{ID: "python-default", Name: "默认 Python", Runtime: "python", Executable: "python", Enabled: true, Default: true},
		{ID: "managed-python", Name: "Managed Python", Runtime: "python", Source: "managed", RequestedVersion: "3.11.13", Architecture: "linux-x64", Enabled: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, profile := range profiles {
		if profile.ID == "managed-python" && !strings.HasSuffix(profile.Executable, filepath.Join("bin", "python3")) {
			t.Fatalf("expected normalized executable to use python3, got %s", profile.Executable)
		}
	}
	downloader := NewHTTPRuntimeDownloader(t.TempDir())
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"assets":[{"name":"cpython-3.11.13+20250601-x86_64-unknown-linux-gnu-install_only.tar.gz","browser_download_url":"https://github.com/a/gz","digest":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}]}]`))
	}))
	defer server.Close()
	oldURL := pythonStandaloneReleasesURL
	pythonStandaloneReleasesURL = server.URL
	t.Cleanup(func() { pythonStandaloneReleasesURL = oldURL })
	spec, err := downloader.linuxPythonDownloadSpec(server.Client(), "3.11.13", "linux-x64", RuntimeDownloadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(spec.Executable, filepath.Join("bin", "python3")) {
		t.Fatalf("expected download spec executable to use python3, got %#v", spec)
	}
}
