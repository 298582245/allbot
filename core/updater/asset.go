package updater

import (
	"runtime"
	"strings"
)

func SelectAssetForCurrentPlatform(assets []ReleaseAsset) (ReleaseAsset, bool) {
	return SelectAssetForPlatform(assets, runtime.GOOS, runtime.GOARCH)
}

func SelectAssetForPlatform(assets []ReleaseAsset, goos string, goarch string) (ReleaseAsset, bool) {
	goos = strings.ToLower(strings.TrimSpace(goos))
	goarch = strings.ToLower(strings.TrimSpace(goarch))
	if goos == "" || goarch == "" {
		return ReleaseAsset{}, false
	}
	for _, asset := range assets {
		name := strings.ToLower(strings.TrimSpace(asset.Name))
		if name == "" || !strings.Contains(name, "allbot") || !strings.Contains(name, goos) || !strings.Contains(name, goarch) {
			continue
		}
		if goos == "windows" && !strings.HasSuffix(name, ".exe") {
			continue
		}
		return asset, true
	}
	return ReleaseAsset{}, false
}
