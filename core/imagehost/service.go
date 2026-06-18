package imagehost

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/allbot/allbot/core/config"
)

const maxSniffBytes = 512

type Service struct {
	db *config.Database
}

func NewService(db *config.Database) *Service {
	return &Service{db: db}
}

func (s *Service) Settings() (config.ImageHostSettings, error) {
	return s.db.GetImageHostSettings()
}

func (s *Service) SaveSettings(settings config.ImageHostSettings) (config.ImageHostSettings, error) {
	result, err := s.SaveSettingsWithOptions(settings, SaveSettingsOptions{StorageDirAction: StorageDirActionKeepOld})
	return result.ImageHostSettings, err
}

func (s *Service) SaveSettingsWithOptions(settings config.ImageHostSettings, options SaveSettingsOptions) (SaveSettingsResult, error) {
	result := SaveSettingsResult{ImageHostSettings: settings}
	oldSettings, err := s.Settings()
	if err != nil {
		return result, err
	}
	if err := config.ValidateImageHostSettings(settings); err != nil {
		return result, err
	}
	settings = config.NormalizeImageHostSettings(settings)
	result.ImageHostSettings = settings

	oldAbs, err := absoluteStorageDir(oldSettings.StorageDir)
	if err != nil {
		return result, err
	}
	newAbs, err := absoluteStorageDir(settings.StorageDir)
	if err != nil {
		return result, err
	}
	migration := SettingsMigrationResult{Changed: !sameStorageDir(oldAbs, newAbs), Action: strings.TrimSpace(options.StorageDirAction), OldStorageDir: oldSettings.StorageDir, NewStorageDir: settings.StorageDir}
	if !migration.Changed {
		if err := os.MkdirAll(settings.StorageDir, 0755); err != nil {
			return result, fmt.Errorf("创建存储目录失败: %w", err)
		}
		migration.Action = ""
		result.Migration = migration
		return result, s.db.SaveImageHostSettings(settings)
	}

	switch migration.Action {
	case StorageDirActionKeepOld:
		if err := os.MkdirAll(settings.StorageDir, 0755); err != nil {
			return result, fmt.Errorf("创建存储目录失败: %w", err)
		}
		migration.Warning = "已保存新存储目录，旧图片文件未迁移，历史图片直链可能失效"
		result.Migration = migration
		return result, s.db.SaveImageHostSettings(settings)
	case StorageDirActionMigrateDeleteOld:
		if err := validateMigrationDirs(oldAbs, newAbs); err != nil {
			return result, err
		}
		if err := os.MkdirAll(settings.StorageDir, 0755); err != nil {
			return result, fmt.Errorf("创建存储目录失败: %w", err)
		}
		migrated, err := s.copyAssetsToNewStorage(oldAbs, newAbs)
		if err != nil {
			return result, err
		}
		migration.MigratedFiles = migrated
		if err := s.db.SaveImageHostSettings(settings); err != nil {
			return result, err
		}
		if err := os.RemoveAll(oldAbs); err != nil {
			migration.Warning = "图片已迁移且配置已保存，但删除旧目录失败: " + err.Error()
		} else {
			migration.DeletedOldDir = true
		}
		result.Migration = migration
		return result, nil
	default:
		return result, fmt.Errorf("storage_dir_action 无效")
	}
}

func (s *Service) Upload(input UploadInput) (*ImageAssetResponse, error) {
	if input.Reader == nil {
		return nil, fmt.Errorf("%w: 缺少图片文件", ErrInvalidInput)
	}
	settings, err := s.Settings()
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(settings.StorageDir, 0755); err != nil {
		return nil, fmt.Errorf("创建存储目录失败: %w", err)
	}
	data, err := readLimited(input.Reader, settings.MaxSize)
	if err != nil {
		return nil, err
	}
	contentType := detectImageContentType(data)
	if !allowedContentType(contentType, settings.AllowedTypes) {
		return nil, fmt.Errorf("%w: 不支持的图片类型", ErrInvalidInput)
	}
	ext, err := extensionByContentType(contentType)
	if err != nil {
		return nil, err
	}
	width, height := imageSize(data)
	digest := sha256.Sum256(data)
	shaText := hex.EncodeToString(digest[:])
	publicID, err := randomPublicID()
	if err != nil {
		return nil, err
	}
	storageKey := publicID + "." + ext
	storagePath, err := resolveStoragePath(settings.StorageDir, storageKey)
	if err != nil {
		return nil, err
	}
	if err = os.WriteFile(storagePath, data, 0644); err != nil {
		return nil, err
	}
	asset, err := s.db.CreateImageAsset(&config.ImageAsset{PublicID: publicID, OriginalName: displayName(input), StorageKey: storageKey, Ext: ext, ContentType: contentType, SizeBytes: int64(len(data)), Width: width, Height: height, SHA256: shaText})
	if err != nil {
		_ = os.Remove(storagePath)
		return nil, err
	}
	return s.assetResponse(asset, input.RequestHost, input.RequestScheme), nil
}

func (s *Service) List(query config.ImageAssetQuery, requestHost, requestScheme string) ([]*ImageAssetResponse, int64, error) {
	items, total, err := s.db.ListImageAssets(query)
	if err != nil {
		return nil, 0, err
	}
	result := make([]*ImageAssetResponse, 0, len(items))
	for _, item := range items {
		result = append(result, s.assetResponse(item, requestHost, requestScheme))
	}
	return result, total, nil
}

func (s *Service) Delete(publicID string) error {
	asset, err := s.db.GetImageAssetByPublicID(publicID)
	if err == sql.ErrNoRows {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	settings, err := s.Settings()
	if err != nil {
		return err
	}
	path, err := resolveStoragePath(settings.StorageDir, asset.StorageKey)
	if err != nil {
		return err
	}
	if err = os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err = s.db.DeleteImageAsset(asset.PublicID); err == sql.ErrNoRows {
		return ErrNotFound
	}
	return err
}

func (s *Service) ResolvePublic(pathValue string) (*PublicAsset, error) {
	publicID, ext, err := parsePublicPath(pathValue)
	if err != nil {
		return nil, err
	}
	asset, err := s.db.GetImageAssetByPublicID(publicID)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(asset.Ext, ext) {
		return nil, ErrNotFound
	}
	settings, err := s.Settings()
	if err != nil {
		return nil, err
	}
	path, err := resolveStoragePath(settings.StorageDir, asset.StorageKey)
	if err != nil {
		return nil, err
	}
	if _, err = os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &PublicAsset{Path: path, ContentType: asset.ContentType, SHA256: asset.SHA256}, nil
}

func (s *Service) PublicURL(asset *config.ImageAsset, requestHost, requestScheme string) string {
	if asset == nil {
		return ""
	}
	settings, err := s.Settings()
	if err != nil {
		settings = config.DefaultImageHostSettings()
	}
	base := settings.PublicBaseURL
	if base == "" && strings.TrimSpace(requestHost) != "" {
		scheme := strings.TrimSpace(requestScheme)
		if scheme == "" {
			scheme = "http"
		}
		base = scheme + "://" + strings.TrimSpace(requestHost)
	}
	path := "/api/open/images/" + asset.PublicID + "." + asset.Ext
	if base == "" {
		return path
	}
	return strings.TrimRight(base, "/") + path
}

func (s *Service) assetResponse(asset *config.ImageAsset, requestHost, requestScheme string) *ImageAssetResponse {
	return &ImageAssetResponse{ImageAsset: asset, URL: s.PublicURL(asset, requestHost, requestScheme)}
}

func (s *Service) copyAssetsToNewStorage(oldAbs, newAbs string) (int, error) {
	assets, err := s.db.ListAllImageAssets()
	if err != nil {
		return 0, err
	}
	migrated := 0
	for _, asset := range assets {
		oldPath, err := resolveStoragePath(oldAbs, asset.StorageKey)
		if err != nil {
			return 0, err
		}
		newPath, err := resolveStoragePath(newAbs, asset.StorageKey)
		if err != nil {
			return 0, err
		}
		oldInfo, err := os.Stat(oldPath)
		if err != nil {
			if os.IsNotExist(err) {
				return 0, fmt.Errorf("旧图片文件不存在: %s", asset.StorageKey)
			}
			return 0, err
		}
		newInfo, err := os.Stat(newPath)
		if err == nil {
			if newInfo.Size() == oldInfo.Size() {
				continue
			}
			return 0, fmt.Errorf("目标图片文件已存在且大小不一致: %s", asset.StorageKey)
		}
		if !os.IsNotExist(err) {
			return 0, err
		}
		if err := copyFile(oldPath, newPath); err != nil {
			return 0, err
		}
		migrated++
	}
	return migrated, nil
}

func copyFile(src, dst string) error {
	input, err := os.Open(src)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(output, input)
	closeErr := output.Close()
	if copyErr != nil {
		_ = os.Remove(dst)
		return copyErr
	}
	if closeErr != nil {
		_ = os.Remove(dst)
		return closeErr
	}
	return nil
}

func absoluteStorageDir(dir string) (string, error) {
	abs, err := filepath.Abs(strings.TrimSpace(dir))
	if err != nil {
		return "", err
	}
	return filepath.Clean(abs), nil
}

func sameStorageDir(a, b string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

func validateMigrationDirs(oldAbs, newAbs string) error {
	if oldAbs == "" || newAbs == "" || sameStorageDir(oldAbs, newAbs) {
		return fmt.Errorf("存储目录迁移路径无效")
	}
	oldVolume := filepath.VolumeName(oldAbs)
	newVolume := filepath.VolumeName(newAbs)
	if oldAbs == oldVolume+string(os.PathSeparator) || newAbs == newVolume+string(os.PathSeparator) {
		return fmt.Errorf("存储目录不能是根目录")
	}
	if isPathInside(oldAbs, newAbs) || isPathInside(newAbs, oldAbs) {
		return fmt.Errorf("新旧存储目录不能互相包含")
	}
	return nil
}

func isPathInside(parent, child string) bool {
	parent = filepath.Clean(parent)
	child = filepath.Clean(child)
	if sameStorageDir(parent, child) {
		return true
	}
	if runtime.GOOS == "windows" {
		parent = strings.ToLower(parent)
		child = strings.ToLower(child)
	}
	return strings.HasPrefix(child, parent+string(os.PathSeparator))
}

func readLimited(reader io.Reader, maxSize int64) ([]byte, error) {
	if maxSize <= 0 {
		maxSize = config.DefaultImageHostSettings().MaxSize
	}
	limited := io.LimitReader(reader, maxSize+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxSize {
		return nil, fmt.Errorf("%w: 图片大小超过限制", ErrInvalidInput)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("%w: 图片内容为空", ErrInvalidInput)
	}
	return data, nil
}

func detectImageContentType(data []byte) string {
	limit := len(data)
	if limit > maxSniffBytes {
		limit = maxSniffBytes
	}
	return strings.ToLower(http.DetectContentType(data[:limit]))
}

func allowedContentType(contentType string, allowed []string) bool {
	contentType = strings.ToLower(strings.TrimSpace(contentType))
	for _, item := range config.NormalizeImageHostSettings(config.ImageHostSettings{AllowedTypes: allowed}).AllowedTypes {
		if item == contentType {
			return true
		}
	}
	return false
}

func extensionByContentType(contentType string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(contentType)) {
	case "image/jpeg":
		return "jpg", nil
	case "image/png":
		return "png", nil
	case "image/gif":
		return "gif", nil
	case "image/webp":
		return "webp", nil
	default:
		return "", fmt.Errorf("%w: 不支持的图片类型", ErrInvalidInput)
	}
}

func imageSize(data []byte) (int, int) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return 0, 0
	}
	return cfg.Width, cfg.Height
}

func randomPublicID() (string, error) {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return hex.EncodeToString(buffer), nil
}

func resolveStoragePath(root, storageKey string) (string, error) {
	root = strings.TrimSpace(root)
	storageKey = strings.Trim(strings.TrimSpace(storageKey), "/\\")
	if root == "" || storageKey == "" || strings.Contains(storageKey, "..") || strings.ContainsAny(storageKey, "/\\") {
		return "", fmt.Errorf("%w: 存储路径无效", ErrInvalidInput)
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	path := filepath.Join(absRoot, storageKey)
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if absPath != absRoot && !strings.HasPrefix(absPath, absRoot+string(os.PathSeparator)) {
		return "", fmt.Errorf("%w: 存储路径越界", ErrInvalidInput)
	}
	return absPath, nil
}

func parsePublicPath(pathValue string) (string, string, error) {
	name := strings.Trim(strings.TrimSpace(pathValue), "/")
	if name == "" || strings.ContainsAny(name, "/\\") {
		return "", "", fmt.Errorf("%w: 图片路径无效", ErrInvalidInput)
	}
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(name)), ".")
	publicID := strings.TrimSuffix(name, "."+ext)
	if publicID == "" || ext == "" || strings.Contains(publicID, ".") {
		return "", "", fmt.Errorf("%w: 图片路径无效", ErrInvalidInput)
	}
	return publicID, ext, nil
}

func displayName(input UploadInput) string {
	name := strings.TrimSpace(input.DisplayName)
	if name == "" {
		name = strings.TrimSpace(input.OriginalName)
	}
	name = filepath.Base(strings.ReplaceAll(name, "\\", "/"))
	if name == "." || name == string(os.PathSeparator) {
		return ""
	}
	return name
}
