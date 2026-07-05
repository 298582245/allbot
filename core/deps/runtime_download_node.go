package deps

import (
	"fmt"
	"path/filepath"
)

func (d *HTTPRuntimeDownloader) nodeDownloadSpec(version, architecture string, options RuntimeDownloadOptions) (runtimeDownloadSpec, error) {
	if architecture != "win-x64" && architecture != "win-arm64" {
		return runtimeDownloadSpec{}, fmt.Errorf("Node.js 架构不支持: %s", architecture)
	}
	archiveName := fmt.Sprintf("node-v%s-%s.zip", version, architecture)
	rootDir := filepath.Join(d.rootDir, "nodejs", fmt.Sprintf("%s-%s", version, architecture))
	mirrorURL := runtimeOptionOrDefault(options.NodeMirrorURL, "https://nodejs.org/dist")
	sourceURL := fmt.Sprintf("%s/v%s/%s", mirrorURL, version, archiveName)
	hashURL := fmt.Sprintf("%s/v%s/SHASUMS256.txt", mirrorURL, version)
	trustedHosts := appendTrustedHost([]string{"nodejs.org"}, mirrorURL)
	return runtimeDownloadSpec{
		Runtime:          "nodejs",
		Version:          version,
		Architecture:     architecture,
		URL:              sourceURL,
		SHA256URL:        hashURL,
		ArchiveName:      archiveName,
		RootDir:          rootDir,
		Executable:       filepath.Join(rootDir, "node.exe"),
		TrustedHosts:     trustedHosts,
		HashTrustedHosts: trustedHosts,
	}, nil
}
