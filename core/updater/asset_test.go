package updater

import "testing"

func TestSelectAssetForPlatform(t *testing.T) {
	assets := []ReleaseAsset{
		{Name: "allbot-linux-amd64", DownloadURL: "linux"},
		{Name: "allbot-windows-amd64.exe", DownloadURL: "windows"},
	}
	asset, ok := SelectAssetForPlatform(assets, "windows", "amd64")
	if !ok {
		t.Fatal("expected windows asset")
	}
	if asset.DownloadURL != "windows" {
		t.Fatalf("asset = %#v", asset)
	}
}

func TestSelectAssetForPlatformRequiresWindowsExe(t *testing.T) {
	assets := []ReleaseAsset{{Name: "allbot-windows-amd64", DownloadURL: "bad"}}
	if asset, ok := SelectAssetForPlatform(assets, "windows", "amd64"); ok {
		t.Fatalf("unexpected asset: %#v", asset)
	}
}

func TestSelectAssetForPlatformMissing(t *testing.T) {
	assets := []ReleaseAsset{{Name: "allbot-linux-arm64", DownloadURL: "linux"}}
	if asset, ok := SelectAssetForPlatform(assets, "windows", "amd64"); ok {
		t.Fatalf("unexpected asset: %#v", asset)
	}
}
