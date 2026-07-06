package deps

import (
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
	assets, err := listPythonStandaloneCandidateAssets(client, architecture, pythonStandaloneReleasePageSize)
	if err != nil {
		return pythonStandaloneAsset{}, err
	}
	version = normalizePythonStandaloneVersion(version)
	for _, item := range assets {
		if strings.EqualFold(item.Version, version) {
			return item.Asset, nil
		}
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
