package deps

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
)

type pythonStandaloneRelease struct {
	Assets []pythonStandaloneAsset `json:"assets"`
}

type pythonStandaloneAsset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}

func (d *HTTPRuntimeDownloader) pythonDownloadSpec(client *http.Client, version, architecture string, options RuntimeDownloadOptions) (runtimeDownloadSpec, error) {
	if isWindowsRuntimeArchitecture(architecture) {
		return d.windowsPythonDownloadSpec(version, architecture, options)
	}
	return d.linuxPythonDownloadSpec(client, version, architecture, options)
}

func (d *HTTPRuntimeDownloader) windowsPythonDownloadSpec(version, architecture string, options RuntimeDownloadOptions) (runtimeDownloadSpec, error) {
	if architecture != "win-x64" {
		return runtimeDownloadSpec{}, fmt.Errorf("Python Windows 嵌入包自动下载暂只支持 win-x64")
	}
	version = normalizePythonNuGetVersion(version)
	archiveName := fmt.Sprintf("python.%s.nupkg", version)
	rootDir := filepath.Join(d.rootDir, "python", fmt.Sprintf("%s-%s", version, architecture))
	packageMirrorURL := runtimeOptionOrDefault(options.PythonPackageMirrorURL, "https://www.nuget.org/api/v2/package/python")
	metadataURL := runtimeOptionOrDefault(options.PythonMetadataURL, "https://api.nuget.org/v3/registration5-gz-semver2/python/index.json")
	trustedHosts := appendTrustedHost([]string{"www.nuget.org", "globalcdn.nuget.org", "api.nuget.org", "nuget.azure.cn"}, packageMirrorURL)
	hashTrustedHosts := appendTrustedHost([]string{"api.nuget.org", "nuget.azure.cn"}, metadataURL)
	return runtimeDownloadSpec{
		Runtime:          "python",
		Version:          version,
		Architecture:     architecture,
		URL:              fmt.Sprintf("%s/%s", packageMirrorURL, version),
		NuGetIndexURL:    metadataURL,
		ArchiveName:      archiveName,
		RootDir:          rootDir,
		Executable:       filepath.Join(rootDir, "tools", "python.exe"),
		TrustedHosts:     trustedHosts,
		HashTrustedHosts: hashTrustedHosts,
		AllowMissingHash: true,
	}, nil
}

func (d *HTTPRuntimeDownloader) linuxPythonDownloadSpec(client *http.Client, version, architecture string, options RuntimeDownloadOptions) (runtimeDownloadSpec, error) {
	asset, err := findPythonStandaloneAsset(client, version, architecture)
	if err != nil {
		return runtimeDownloadSpec{}, err
	}
	rootDir := filepath.Join(d.rootDir, "python", fmt.Sprintf("%s-%s", version, architecture))
	return runtimeDownloadSpec{
		Runtime:          "python",
		Version:          version,
		Architecture:     architecture,
		URL:              asset.URL,
		ArchiveName:      asset.Name,
		RootDir:          rootDir,
		Executable:       filepath.Join(rootDir, "bin", "python3"),
		TrustedHosts:     []string{"github.com", "release-assets.githubusercontent.com", "objects.githubusercontent.com"},
		HashTrustedHosts: []string{"github.com", "release-assets.githubusercontent.com", "objects.githubusercontent.com"},
		AllowMissingHash: true,
	}, nil
}

func findPythonStandaloneAsset(client *http.Client, version, architecture string) (pythonStandaloneAsset, error) {
	platform, err := pythonStandalonePlatform(architecture)
	if err != nil {
		return pythonStandaloneAsset{}, err
	}
	request, err := http.NewRequest(http.MethodGet, "https://api.github.com/repos/astral-sh/python-build-standalone/releases?per_page=10", nil)
	if err != nil {
		return pythonStandaloneAsset{}, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	response, err := client.Do(request)
	if err != nil {
		return pythonStandaloneAsset{}, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return pythonStandaloneAsset{}, fmt.Errorf("读取 Python standalone 发布列表失败: HTTP %d", response.StatusCode)
	}
	var releases []pythonStandaloneRelease
	if err := json.NewDecoder(response.Body).Decode(&releases); err != nil {
		return pythonStandaloneAsset{}, err
	}
	prefix := "cpython-" + normalizePythonStandaloneVersion(version) + "+"
	var fallback pythonStandaloneAsset
	for _, release := range releases {
		for _, asset := range release.Assets {
			name := asset.Name
			if !strings.HasPrefix(name, prefix) || !strings.Contains(name, platform) || !strings.Contains(name, "install_only") {
				continue
			}
			if strings.HasSuffix(name, ".tar.gz") || strings.HasSuffix(name, ".tgz") {
				return asset, nil
			}
			if fallback.Name == "" && (strings.HasSuffix(name, ".tar.zst") || strings.HasSuffix(name, ".tar.zstd")) {
				fallback = asset
			}
		}
	}
	if fallback.Name != "" {
		return fallback, nil
	}
	return pythonStandaloneAsset{}, fmt.Errorf("未找到 Python %s 的 %s 预编译下载资产", version, architecture)
}

func pythonStandalonePlatform(architecture string) (string, error) {
	switch architecture {
	case "linux-x64":
		return "x86_64-unknown-linux-gnu", nil
	case "linux-arm64":
		return "aarch64-unknown-linux-gnu", nil
	default:
		return "", fmt.Errorf("Python standalone 不支持架构: %s", architecture)
	}
}

func normalizePythonStandaloneVersion(version string) string {
	parts := strings.Split(version, ".")
	if len(parts) == 2 {
		return version + ".0"
	}
	return version
}

func normalizePythonNuGetVersion(version string) string {
	parts := strings.Split(version, ".")
	if len(parts) == 2 {
		return version + ".0"
	}
	return version
}
