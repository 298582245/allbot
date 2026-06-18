package imagehost

import (
	"errors"
	"io"

	"github.com/allbot/allbot/core/config"
)

var (
	ErrNotFound     = errors.New("图片不存在")
	ErrInvalidInput = errors.New("图片参数无效")
)

type UploadInput struct {
	Reader        io.Reader
	OriginalName  string
	DisplayName   string
	RequestHost   string
	RequestScheme string
}

type ImageAssetResponse struct {
	*config.ImageAsset
	URL string `json:"url"`
}

type PublicAsset struct {
	Path        string
	ContentType string
	SHA256      string
}

const (
	StorageDirActionMigrateDeleteOld = "migrate_delete_old"
	StorageDirActionKeepOld          = "keep_old"
)

type SaveSettingsOptions struct {
	StorageDirAction string
}

type SettingsMigrationResult struct {
	Changed       bool   `json:"changed"`
	Action        string `json:"action"`
	OldStorageDir string `json:"old_storage_dir"`
	NewStorageDir string `json:"new_storage_dir"`
	MigratedFiles int    `json:"migrated_files"`
	DeletedOldDir bool   `json:"deleted_old_dir"`
	Warning       string `json:"warning,omitempty"`
}

type SaveSettingsResult struct {
	config.ImageHostSettings
	Migration SettingsMigrationResult `json:"migration"`
}
