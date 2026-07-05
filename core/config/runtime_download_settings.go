package config

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

const (
	runtimeDownloadSettingsKey         = "runtime.download.config"
	runtimeDownloadSettingsDescription = "运行环境下载代理和镜像配置"
)

type RuntimeDownloadSettings struct {
	ProxyURL               string `json:"proxy_url"`
	NodeMirrorURL          string `json:"node_mirror_url"`
	PythonPackageMirrorURL string `json:"python_package_mirror_url"`
	PythonMetadataURL      string `json:"python_metadata_url"`
}

func DefaultRuntimeDownloadSettings() RuntimeDownloadSettings {
	return RuntimeDownloadSettings{
		NodeMirrorURL:          "https://nodejs.org/dist",
		PythonPackageMirrorURL: "https://www.nuget.org/api/v2/package/python",
		PythonMetadataURL:      "https://api.nuget.org/v3/registration5-gz-semver2/python/index.json",
	}
}

func NormalizeRuntimeDownloadSettings(settings RuntimeDownloadSettings) (RuntimeDownloadSettings, error) {
	defaults := DefaultRuntimeDownloadSettings()
	settings.ProxyURL = normalizeRuntimeDownloadURL(settings.ProxyURL)
	settings.NodeMirrorURL = normalizeRuntimeDownloadURL(settings.NodeMirrorURL)
	settings.PythonPackageMirrorURL = normalizeRuntimeDownloadURL(settings.PythonPackageMirrorURL)
	settings.PythonMetadataURL = normalizeRuntimeDownloadURL(settings.PythonMetadataURL)
	if settings.NodeMirrorURL == "" {
		settings.NodeMirrorURL = defaults.NodeMirrorURL
	}
	if settings.PythonPackageMirrorURL == "" {
		settings.PythonPackageMirrorURL = defaults.PythonPackageMirrorURL
	}
	if settings.PythonMetadataURL == "" {
		settings.PythonMetadataURL = defaults.PythonMetadataURL
	}
	if err := validateRuntimeDownloadProxyURL(settings.ProxyURL); err != nil {
		return RuntimeDownloadSettings{}, err
	}
	if err := validateRuntimeDownloadMirrorURL("Node.js 镜像地址", settings.NodeMirrorURL); err != nil {
		return RuntimeDownloadSettings{}, err
	}
	if err := validateRuntimeDownloadMirrorURL("Python 包镜像地址", settings.PythonPackageMirrorURL); err != nil {
		return RuntimeDownloadSettings{}, err
	}
	if err := validateRuntimeDownloadMirrorURL("Python 元数据地址", settings.PythonMetadataURL); err != nil {
		return RuntimeDownloadSettings{}, err
	}
	return settings, nil
}

func (d *Database) GetRuntimeDownloadSettings() (RuntimeDownloadSettings, error) {
	settings := DefaultRuntimeDownloadSettings()
	value, err := d.GetSetting(runtimeDownloadSettingsKey)
	if err == sql.ErrNoRows {
		return settings, nil
	}
	if err != nil {
		return settings, err
	}
	if strings.TrimSpace(value) == "" {
		return settings, nil
	}
	if err := json.Unmarshal([]byte(value), &settings); err != nil {
		return DefaultRuntimeDownloadSettings(), nil
	}
	return NormalizeRuntimeDownloadSettings(settings)
}

func (d *Database) SaveRuntimeDownloadSettings(settings RuntimeDownloadSettings) error {
	normalized, err := NormalizeRuntimeDownloadSettings(settings)
	if err != nil {
		return err
	}
	data, err := json.Marshal(normalized)
	if err != nil {
		return err
	}
	return d.SetSetting(runtimeDownloadSettingsKey, string(data), runtimeDownloadSettingsDescription)
}

func normalizeRuntimeDownloadURL(value string) string {
	return strings.TrimRight(strings.TrimSpace(value), "/")
}

func validateRuntimeDownloadProxyURL(value string) error {
	if value == "" {
		return nil
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return fmt.Errorf("代理地址必须是 HTTP/HTTPS URL 且包含 host")
	}
	return nil
}

func validateRuntimeDownloadMirrorURL(label, value string) error {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" || parsed.Scheme != "https" {
		return fmt.Errorf("%s必须是 HTTPS URL 且包含 host", label)
	}
	return nil
}
