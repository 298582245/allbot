package deps

import (
	"fmt"
	"path/filepath"
	"strings"
)

func (d *HTTPRuntimeDownloader) pythonDownloadSpec(version, architecture string, options RuntimeDownloadOptions) (runtimeDownloadSpec, error) {
	if architecture != "win-x64" {
		return runtimeDownloadSpec{}, fmt.Errorf("Python 自动下载暂只支持 win-x64")
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

func normalizePythonNuGetVersion(version string) string {
	parts := strings.Split(version, ".")
	if len(parts) == 2 {
		return version + ".0"
	}
	return version
}
