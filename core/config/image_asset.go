package config

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const imageHostConfigKey = "imagehost.config"

const MaxImageHostUploadSize int64 = 100 * 1024 * 1024

type ImageAsset struct {
	ID           int64     `json:"id"`
	PublicID     string    `json:"public_id"`
	OriginalName string    `json:"original_name"`
	StorageKey   string    `json:"storage_key"`
	Ext          string    `json:"ext"`
	ContentType  string    `json:"content_type"`
	SizeBytes    int64     `json:"size_bytes"`
	Width        int       `json:"width"`
	Height       int       `json:"height"`
	SHA256       string    `json:"sha256"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type ImageAssetQuery struct {
	Keyword     string
	ContentType string
	Limit       int
	Offset      int
}

type ImageHostSettings struct {
	StorageDir    string   `json:"storage_dir"`
	PublicBaseURL string   `json:"public_base_url"`
	MaxSize       int64    `json:"max_size"`
	AllowedTypes  []string `json:"allowed_types"`
}

type ImageAssetStatsSummary struct {
	TotalAssets    int64                       `json:"total_assets"`
	TotalSizeBytes int64                       `json:"total_size_bytes"`
	ByContentType  []ImageAssetContentTypeStat `json:"by_content_type"`
}

type ImageAssetContentTypeStat struct {
	Name      string `json:"name"`
	Count     int64  `json:"count"`
	SizeBytes int64  `json:"size_bytes"`
}

func DefaultImageHostSettings() ImageHostSettings {
	return ImageHostSettings{StorageDir: "./runtime/image_assets", PublicBaseURL: "", MaxSize: 5 * 1024 * 1024, AllowedTypes: []string{"image/jpeg", "image/png", "image/gif", "image/webp"}}
}

func defaultImageHostConfigJSON() string {
	data, _ := json.Marshal(DefaultImageHostSettings())
	return string(data)
}

func NormalizeImageHostSettings(settings ImageHostSettings) ImageHostSettings {
	defaults := DefaultImageHostSettings()
	settings.StorageDir = strings.TrimSpace(settings.StorageDir)
	if settings.StorageDir == "" {
		settings.StorageDir = defaults.StorageDir
	}
	settings.PublicBaseURL = strings.TrimRight(strings.TrimSpace(settings.PublicBaseURL), "/")
	if settings.MaxSize <= 0 {
		settings.MaxSize = defaults.MaxSize
	}
	if settings.MaxSize > MaxImageHostUploadSize {
		settings.MaxSize = MaxImageHostUploadSize
	}
	settings.AllowedTypes = normalizeImageContentTypes(settings.AllowedTypes)
	if len(settings.AllowedTypes) == 0 {
		settings.AllowedTypes = append([]string{}, defaults.AllowedTypes...)
	}
	return settings
}

func ValidateImageHostSettings(settings ImageHostSettings) error {
	settings.StorageDir = strings.TrimSpace(settings.StorageDir)
	if settings.StorageDir == "" {
		return fmt.Errorf("存储目录不能为空")
	}
	if settings.MaxSize <= 0 {
		return fmt.Errorf("最大上传大小必须大于 0")
	}
	if settings.MaxSize > MaxImageHostUploadSize {
		return fmt.Errorf("最大上传大小不能超过 %d 字节", MaxImageHostUploadSize)
	}
	if len(normalizeImageContentTypes(settings.AllowedTypes)) == 0 {
		return fmt.Errorf("允许的图片类型不能为空")
	}
	return nil
}

func (d *Database) GetImageHostSettings() (ImageHostSettings, error) {
	settings := DefaultImageHostSettings()
	value, err := d.GetSetting(imageHostConfigKey)
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
		return DefaultImageHostSettings(), nil
	}
	return NormalizeImageHostSettings(settings), nil
}

func (d *Database) SaveImageHostSettings(settings ImageHostSettings) error {
	settings.StorageDir = strings.TrimSpace(settings.StorageDir)
	settings.PublicBaseURL = strings.TrimRight(strings.TrimSpace(settings.PublicBaseURL), "/")
	settings.AllowedTypes = normalizeImageContentTypes(settings.AllowedTypes)
	if err := ValidateImageHostSettings(settings); err != nil {
		return err
	}
	data, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	return d.SetSetting(imageHostConfigKey, string(data), "图床配置")
}

func (d *Database) CreateImageAsset(asset *ImageAsset) (*ImageAsset, error) {
	if asset == nil {
		return nil, fmt.Errorf("图片资产不能为空")
	}
	asset.PublicID = strings.TrimSpace(asset.PublicID)
	asset.OriginalName = strings.TrimSpace(asset.OriginalName)
	asset.StorageKey = strings.TrimSpace(asset.StorageKey)
	asset.Ext = strings.Trim(strings.ToLower(strings.TrimSpace(asset.Ext)), ".")
	asset.ContentType = strings.ToLower(strings.TrimSpace(asset.ContentType))
	asset.SHA256 = strings.TrimSpace(asset.SHA256)
	if asset.PublicID == "" || asset.StorageKey == "" || asset.Ext == "" || asset.ContentType == "" || asset.SizeBytes <= 0 {
		return nil, fmt.Errorf("图片资产字段不完整")
	}
	_, err := d.db.Exec(`
		INSERT INTO image_assets (public_id, original_name, storage_key, ext, content_type, size_bytes, width, height, sha256, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, asset.PublicID, asset.OriginalName, asset.StorageKey, asset.Ext, asset.ContentType, asset.SizeBytes, asset.Width, asset.Height, asset.SHA256)
	if err != nil {
		return nil, err
	}
	return d.GetImageAssetByPublicID(asset.PublicID)
}

func (d *Database) GetImageAssetByPublicID(publicID string) (*ImageAsset, error) {
	publicID = strings.TrimSpace(publicID)
	if publicID == "" {
		return nil, sql.ErrNoRows
	}
	return scanImageAsset(d.db.QueryRow(imageAssetSelectSQL()+` WHERE public_id = ?`, publicID))
}

func (d *Database) ListImageAssets(query ImageAssetQuery) ([]*ImageAsset, int64, error) {
	where, args := imageAssetWhere(query)
	var total int64
	if err := d.db.QueryRow(`SELECT COUNT(*) FROM image_assets`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	limit, offset := normalizeLimitOffset(query.Limit, query.Offset)
	rows, err := d.db.Query(imageAssetSelectSQL()+where+` ORDER BY created_at DESC, id DESC LIMIT ? OFFSET ?`, append(args, limit, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items, err := scanImageAssetRows(rows)
	return items, total, err
}

func (d *Database) ListAllImageAssets() ([]*ImageAsset, error) {
	rows, err := d.db.Query(imageAssetSelectSQL() + ` ORDER BY created_at ASC, id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanImageAssetRows(rows)
}

func (d *Database) GetImageAssetStatsSummary() (ImageAssetStatsSummary, error) {
	var summary ImageAssetStatsSummary
	if err := d.db.QueryRow(`SELECT COUNT(*), COALESCE(SUM(size_bytes), 0) FROM image_assets`).Scan(&summary.TotalAssets, &summary.TotalSizeBytes); err != nil {
		return summary, err
	}
	rows, err := d.db.Query(`
		SELECT content_type, COUNT(*), COALESCE(SUM(size_bytes), 0)
		FROM image_assets
		GROUP BY content_type
		ORDER BY COUNT(*) DESC, content_type ASC
	`)
	if err != nil {
		return summary, err
	}
	defer rows.Close()
	for rows.Next() {
		var item ImageAssetContentTypeStat
		if err := rows.Scan(&item.Name, &item.Count, &item.SizeBytes); err != nil {
			return summary, err
		}
		summary.ByContentType = append(summary.ByContentType, item)
	}
	return summary, rows.Err()
}

func (d *Database) DeleteImageAsset(publicID string) error {
	publicID = strings.TrimSpace(publicID)
	if publicID == "" {
		return sql.ErrNoRows
	}
	result, err := d.db.Exec(`DELETE FROM image_assets WHERE public_id = ?`, publicID)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func normalizeImageContentTypes(items []string) []string {
	result := make([]string, 0, len(items))
	seen := map[string]bool{}
	for _, item := range items {
		value := strings.ToLower(strings.TrimSpace(item))
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}

func imageAssetSelectSQL() string {
	return `SELECT id, public_id, original_name, storage_key, ext, content_type, size_bytes, width, height, sha256, created_at, updated_at FROM image_assets`
}

func scanImageAsset(row interface{ Scan(...interface{}) error }) (*ImageAsset, error) {
	var item ImageAsset
	if err := row.Scan(&item.ID, &item.PublicID, &item.OriginalName, &item.StorageKey, &item.Ext, &item.ContentType, &item.SizeBytes, &item.Width, &item.Height, &item.SHA256, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return nil, err
	}
	return &item, nil
}

func scanImageAssetRows(rows *sql.Rows) ([]*ImageAsset, error) {
	items := []*ImageAsset{}
	for rows.Next() {
		item, err := scanImageAsset(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func imageAssetWhere(query ImageAssetQuery) (string, []interface{}) {
	clauses := []string{}
	args := []interface{}{}
	keyword := strings.TrimSpace(query.Keyword)
	if keyword != "" {
		clauses = append(clauses, `(public_id LIKE ? OR original_name LIKE ?)`)
		like := "%" + keyword + "%"
		args = append(args, like, like)
	}
	contentType := strings.ToLower(strings.TrimSpace(query.ContentType))
	if contentType != "" {
		clauses = append(clauses, `content_type = ?`)
		args = append(args, contentType)
	}
	if len(clauses) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(clauses, " AND "), args
}
