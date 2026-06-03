package config

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/allbot/allbot/core/types"
)

type PlatformAdmin struct {
	Platform string `json:"platform"`
	UserID   string `json:"user_id"`
}

const (
	adminPasswordKey                  = "admin.password"
	adminPasswordDescription          = "管理员密码哈希"
	adminGeneratedPasswordKey         = "admin.generated_password"
	adminGeneratedPasswordDescription = "首次自动生成的管理员默认密码"
	adminPasswordHashPrefix           = "pbkdf2_sha256"
	adminPasswordIterations           = 200000
	adminPasswordSaltBytes            = 16
	adminPasswordKeyBytes             = 32
)

type AdminPasswordInitResult struct {
	Username          string
	GeneratedPassword string
	Generated         bool
	Migrated          bool
}

type SystemSettings struct {
	AdminUsername   string                    `json:"admin_username"`
	AdminPassword   string                    `json:"admin_password,omitempty"`
	PlatformAdmins  []PlatformAdmin           `json:"platform_admins"`
	AutoRefresh     bool                      `json:"auto_refresh"`
	RefreshInterval int                       `json:"refresh_interval"`
	PluginDir       string                    `json:"plugin_dir"`
	AutoLoadPlugins bool                      `json:"auto_load_plugins"`
	PointsUnit      string                    `json:"points_unit"`
	AccessControl   types.AccessControlConfig `json:"access_control"`
}

func (d *Database) GetSetting(key string) (string, error) {
	var value string
	err := d.db.QueryRow(`SELECT value FROM system_settings WHERE key = ?`, key).Scan(&value)
	return value, err
}

func (d *Database) SetSetting(key, value, description string) error {
	_, err := d.db.Exec(`
		INSERT INTO system_settings (key, value, description, created_at, updated_at)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, description = excluded.description, updated_at = CURRENT_TIMESTAMP
	`, key, value, description)
	return err
}

func (d *Database) GetSystemSettings() (*SystemSettings, error) {
	items, err := d.getSettingsMap()
	if err != nil {
		return nil, err
	}
	platformAdmins := parsePlatformAdmins(items["admin.platform_users"])
	return &SystemSettings{
		AdminUsername:   valueOrDefault(items, "admin.username", "admin"),
		PlatformAdmins:  platformAdmins,
		AutoRefresh:     valueOrDefault(items, "web.auto_refresh", "true") == "true",
		RefreshInterval: intValueOrDefault(items, "web.refresh_interval", 5),
		PluginDir:       valueOrDefault(items, "plugin.dir", "./plugins"),
		AutoLoadPlugins: valueOrDefault(items, "plugin.auto_load", "true") == "true",
		PointsUnit:      valueOrDefault(items, "user.points_unit", "积分"),
		AccessControl:   ParseAccessControlConfig(items["access_control"]),
	}, nil
}

func (d *Database) SaveSystemSettings(settings *SystemSettings) error {
	if settings.AdminUsername == "" {
		return fmt.Errorf("管理员用户名不能为空")
	}
	if settings.RefreshInterval <= 0 {
		settings.RefreshInterval = 5
	}
	if settings.PluginDir == "" {
		settings.PluginDir = "./plugins"
	}
	if settings.PointsUnit == "" {
		settings.PointsUnit = "积分"
	}

	items := map[string]struct {
		value       string
		description string
	}{
		"admin.username":       {settings.AdminUsername, "管理员用户名"},
		"admin.platform_users": {marshalPlatformAdmins(settings.PlatformAdmins), "平台管理员用户列表"},
		"web.auto_refresh":     {boolString(settings.AutoRefresh), "是否自动刷新"},
		"web.refresh_interval": {fmt.Sprintf("%d", settings.RefreshInterval), "刷新间隔秒数"},
		"plugin.dir":           {settings.PluginDir, "插件目录"},
		"plugin.auto_load":     {boolString(settings.AutoLoadPlugins), "启动时自动加载插件"},
		"user.points_unit":     {settings.PointsUnit, "用户积分单位"},
		"access_control":       {MarshalAccessControlConfig(settings.AccessControl), "系统访问控制配置"},
	}
	for key, item := range items {
		if err := d.SetSetting(key, item.value, item.description); err != nil {
			return err
		}
	}
	return nil
}

func ParseAccessControlConfig(value string) types.AccessControlConfig {
	if value == "" {
		return types.AccessControlConfig{}
	}
	var config types.AccessControlConfig
	if err := json.Unmarshal([]byte(value), &config); err != nil {
		return types.AccessControlConfig{}
	}
	return NormalizeAccessControlConfig(config)
}

func MarshalAccessControlConfig(config types.AccessControlConfig) string {
	config = NormalizeAccessControlConfig(config)
	data, _ := json.Marshal(config)
	return string(data)
}

func NormalizeAccessControlConfig(config types.AccessControlConfig) types.AccessControlConfig {
	config.WhitelistGroups = normalizeStringList(config.WhitelistGroups)
	config.BlockedGroups = normalizeStringList(config.BlockedGroups)
	config.WhitelistUserIDs = normalizeStringList(config.WhitelistUserIDs)
	config.BlockedUserIDs = normalizeStringList(config.BlockedUserIDs)
	return config
}

func normalizeStringList(items []string) []string {
	result := make([]string, 0, len(items))
	seen := make(map[string]bool)
	for _, item := range items {
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		result = append(result, item)
	}
	return result
}

func (d *Database) IsPlatformAdmin(platform, userID string) bool {
	if platform == "" || userID == "" {
		return false
	}
	value, err := d.GetSetting("admin.platform_users")
	if err != nil {
		return false
	}
	for _, item := range parsePlatformAdmins(value) {
		if item.Platform == platform && item.UserID == userID {
			return true
		}
	}
	return false
}

func (d *Database) EnsureAdminPassword() (*AdminPasswordInitResult, error) {
	username, err := d.GetSetting("admin.username")
	if err != nil || username == "" {
		username = "admin"
	}

	storedGeneratedPassword, err := d.GetSetting(adminGeneratedPasswordKey)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	storedGeneratedPassword = strings.TrimSpace(storedGeneratedPassword)

	current, err := d.GetSetting(adminPasswordKey)
	if err == sql.ErrNoRows || strings.TrimSpace(current) == "" {
		password, err := GenerateAdminPassword()
		if err != nil {
			return nil, err
		}
		hash, err := HashAdminPassword(password)
		if err != nil {
			return nil, err
		}
		if err := d.SetSetting(adminPasswordKey, hash, adminPasswordDescription); err != nil {
			return nil, err
		}
		if err := d.SetSetting(adminGeneratedPasswordKey, password, adminGeneratedPasswordDescription); err != nil {
			return nil, err
		}
		return &AdminPasswordInitResult{Username: username, GeneratedPassword: password, Generated: true}, nil
	}
	if err != nil {
		return nil, err
	}
	if !IsAdminPasswordHash(current) {
		hash, err := HashAdminPassword(current)
		if err != nil {
			return nil, err
		}
		if err := d.SetSetting(adminPasswordKey, hash, adminPasswordDescription); err != nil {
			return nil, err
		}
		if storedGeneratedPassword == "" {
			if err := d.SetSetting(adminGeneratedPasswordKey, current, adminGeneratedPasswordDescription); err != nil {
				return nil, err
			}
			storedGeneratedPassword = current
		}
		return &AdminPasswordInitResult{Username: username, GeneratedPassword: storedGeneratedPassword, Migrated: true}, nil
	}
	return &AdminPasswordInitResult{Username: username, GeneratedPassword: storedGeneratedPassword}, nil
}

func (d *Database) VerifyAdminPassword(password string) (bool, error) {
	current, err := d.GetSetting(adminPasswordKey)
	if err != nil {
		return false, err
	}
	if IsAdminPasswordHash(current) {
		return VerifyAdminPasswordHash(password, current), nil
	}
	if subtle.ConstantTimeCompare([]byte(password), []byte(current)) != 1 {
		return false, nil
	}
	hash, err := HashAdminPassword(current)
	if err != nil {
		return false, err
	}
	if err := d.SetSetting(adminPasswordKey, hash, adminPasswordDescription); err != nil {
		return false, err
	}
	return true, nil
}

func (d *Database) ChangeAdminPassword(oldPassword, newPassword string) error {
	ok, err := d.VerifyAdminPassword(oldPassword)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("当前密码不正确")
	}
	if strings.TrimSpace(newPassword) == "" {
		return fmt.Errorf("新密码不能为空")
	}
	hash, err := HashAdminPassword(newPassword)
	if err != nil {
		return err
	}
	return d.SetSetting(adminPasswordKey, hash, adminPasswordDescription)
}

func GenerateAdminPassword() (string, error) {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789!@#$%*-_=+"
	buffer := make([]byte, 24)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	password := make([]byte, len(buffer))
	for i, value := range buffer {
		password[i] = alphabet[int(value)%len(alphabet)]
	}
	return string(password), nil
}

func HashAdminPassword(password string) (string, error) {
	salt := make([]byte, adminPasswordSaltBytes)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key, err := pbkdf2.Key(sha256.New, password, salt, adminPasswordIterations, adminPasswordKeyBytes)
	if err != nil {
		return "", err
	}
	return strings.Join([]string{
		adminPasswordHashPrefix,
		strconv.Itoa(adminPasswordIterations),
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	}, "$"), nil
}

func IsAdminPasswordHash(value string) bool {
	return strings.HasPrefix(value, adminPasswordHashPrefix+"$")
}

func VerifyAdminPasswordHash(password, encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 4 || parts[0] != adminPasswordHashPrefix {
		return false
	}
	iterations, err := strconv.Atoi(parts[1])
	if err != nil || iterations <= 0 {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[2])
	if err != nil {
		return false
	}
	expected, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil || len(expected) == 0 {
		return false
	}
	actual, err := pbkdf2.Key(sha256.New, password, salt, iterations, len(expected))
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(actual, expected) == 1
}

func (d *Database) getSettingsMap() (map[string]string, error) {
	rows, err := d.db.Query(`SELECT key, value FROM system_settings`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]string)
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}
		result[key] = value
	}
	return result, rows.Err()
}

func valueOrDefault(items map[string]string, key, fallback string) string {
	if value := items[key]; value != "" {
		return value
	}
	return fallback
}

func intValueOrDefault(items map[string]string, key string, fallback int) int {
	var value int
	if _, err := fmt.Sscanf(items[key], "%d", &value); err == nil && value > 0 {
		return value
	}
	return fallback
}

func boolString(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func parsePlatformAdmins(value string) []PlatformAdmin {
	if value == "" {
		return []PlatformAdmin{}
	}
	var admins []PlatformAdmin
	if err := json.Unmarshal([]byte(value), &admins); err != nil {
		return []PlatformAdmin{}
	}
	return admins
}

func marshalPlatformAdmins(admins []PlatformAdmin) string {
	if admins == nil {
		admins = []PlatformAdmin{}
	}
	data, _ := json.Marshal(admins)
	return string(data)
}
