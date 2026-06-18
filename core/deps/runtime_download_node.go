package deps

import (
	"fmt"
	"path/filepath"
)

func (d *HTTPRuntimeDownloader) nodeDownloadSpec(version, architecture string) (runtimeDownloadSpec, error) {
	if architecture != "win-x64" && architecture != "win-arm64" {
		return runtimeDownloadSpec{}, fmt.Errorf("Node.js 架构不支持: %s", architecture)
	}
	archiveName := fmt.Sprintf("node-v%s-%s.zip", version, architecture)
	rootDir := filepath.Join(d.rootDir, "nodejs", fmt.Sprintf("%s-%s", version, architecture))
	return runtimeDownloadSpec{
		Runtime:          "nodejs",
		Version:          version,
		Architecture:     architecture,
		URL:              fmt.Sprintf("https://nodejs.org/dist/v%s/%s", version, archiveName),
		SHA256URL:        fmt.Sprintf("https://nodejs.org/dist/v%s/SHASUMS256.txt", version),
		ArchiveName:      archiveName,
		RootDir:          rootDir,
		Executable:       filepath.Join(rootDir, "node.exe"),
		TrustedHosts:     []string{"nodejs.org"},
		HashTrustedHosts: []string{"nodejs.org"},
	}, nil
}
