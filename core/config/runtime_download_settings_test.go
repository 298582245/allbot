package config

import "testing"

func TestRuntimeDownloadSettingsDefaults(t *testing.T) {
	db, err := NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("NewDatabase returned error: %v", err)
	}
	defer db.Close()

	settings, err := db.GetRuntimeDownloadSettings()
	if err != nil {
		t.Fatalf("GetRuntimeDownloadSettings returned error: %v", err)
	}
	defaults := DefaultRuntimeDownloadSettings()
	if settings != defaults {
		t.Fatalf("expected defaults %#v, got %#v", defaults, settings)
	}
}

func TestSaveRuntimeDownloadSettings(t *testing.T) {
	db, err := NewDatabase(":memory:")
	if err != nil {
		t.Fatalf("NewDatabase returned error: %v", err)
	}
	defer db.Close()

	input := RuntimeDownloadSettings{
		ProxyURL:               " http://127.0.0.1:7890/ ",
		NodeMirrorURL:          " https://npmmirror.com/mirrors/node/ ",
		PythonPackageMirrorURL: " https://mirror.example.com/nuget/python/ ",
		PythonMetadataURL:      " https://mirror.example.com/nuget/python/index.json/ ",
	}
	if err := db.SaveRuntimeDownloadSettings(input); err != nil {
		t.Fatalf("SaveRuntimeDownloadSettings returned error: %v", err)
	}
	settings, err := db.GetRuntimeDownloadSettings()
	if err != nil {
		t.Fatalf("GetRuntimeDownloadSettings returned error: %v", err)
	}
	if settings.ProxyURL != "http://127.0.0.1:7890" || settings.NodeMirrorURL != "https://npmmirror.com/mirrors/node" || settings.PythonPackageMirrorURL != "https://mirror.example.com/nuget/python" || settings.PythonMetadataURL != "https://mirror.example.com/nuget/python/index.json" {
		t.Fatalf("settings not normalized: %#v", settings)
	}
}

func TestRuntimeDownloadSettingsFillsEmptyMirrors(t *testing.T) {
	normalized, err := NormalizeRuntimeDownloadSettings(RuntimeDownloadSettings{ProxyURL: "https://proxy.example.com"})
	if err != nil {
		t.Fatalf("NormalizeRuntimeDownloadSettings returned error: %v", err)
	}
	defaults := DefaultRuntimeDownloadSettings()
	if normalized.NodeMirrorURL != defaults.NodeMirrorURL || normalized.PythonPackageMirrorURL != defaults.PythonPackageMirrorURL || normalized.PythonMetadataURL != defaults.PythonMetadataURL {
		t.Fatalf("expected default mirrors, got %#v", normalized)
	}
}

func TestRuntimeDownloadSettingsRejectsInvalidProxy(t *testing.T) {
	cases := []string{"127.0.0.1:7890", "ftp://proxy.example.com", "http://"}
	for _, value := range cases {
		t.Run(value, func(t *testing.T) {
			_, err := NormalizeRuntimeDownloadSettings(RuntimeDownloadSettings{ProxyURL: value})
			if err == nil {
				t.Fatal("expected invalid proxy to fail")
			}
		})
	}
}

func TestRuntimeDownloadSettingsRejectsInvalidMirrors(t *testing.T) {
	cases := []RuntimeDownloadSettings{
		{NodeMirrorURL: "http://node.example.com"},
		{PythonPackageMirrorURL: "ftp://python.example.com"},
		{PythonMetadataURL: "https://"},
	}
	for _, settings := range cases {
		if _, err := NormalizeRuntimeDownloadSettings(settings); err == nil {
			t.Fatalf("expected invalid mirror to fail: %#v", settings)
		}
	}
}
