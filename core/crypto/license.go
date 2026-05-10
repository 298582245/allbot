package crypto

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"os"
	"time"
)

// License 授权证书
type License struct {
	PluginID  string    `json:"plugin_id"`
	UserID    string    `json:"user_id"`
	DeviceID  string    `json:"device_id"`
	Key       string    `json:"license_key"`
	Type      string    `json:"type"`       // one-time | subscription
	ExpiresAt time.Time `json:"expires_at"` // 过期时间
	Signature string    `json:"signature"`  // RSA 签名
}

// LicenseManager 授权管理器
type LicenseManager struct {
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
	deviceID   string
}

// NewLicenseManager 创建授权管理器
func NewLicenseManager() (*LicenseManager, error) {
	// 生成 RSA 密钥对
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}

	return &LicenseManager{
		privateKey: privateKey,
		publicKey:  &privateKey.PublicKey,
		deviceID:   GetDeviceID(),
	}, nil
}

// GenerateLicense 生成授权证书
func (lm *LicenseManager) GenerateLicense(pluginID, userID string, licenseType string, duration time.Duration) (*License, error) {
	license := &License{
		PluginID:  pluginID,
		UserID:    userID,
		DeviceID:  lm.deviceID,
		Key:       generateRandomString(32),
		Type:      licenseType,
		ExpiresAt: time.Now().Add(duration),
	}

	// 签名
	signature, err := lm.signLicense(license)
	if err != nil {
		return nil, err
	}
	license.Signature = signature

	return license, nil
}

// VerifyLicense 验证授权证书
func (lm *LicenseManager) VerifyLicense(license *License) error {
	// 1. 验证签名
	if err := lm.verifySignature(license); err != nil {
		return fmt.Errorf("invalid signature: %w", err)
	}

	// 2. 验证设备绑定
	if license.DeviceID != lm.deviceID {
		return fmt.Errorf("device mismatch: expected %s, got %s", lm.deviceID, license.DeviceID)
	}

	// 3. 验证过期时间
	if time.Now().After(license.ExpiresAt) {
		return fmt.Errorf("license expired at %s", license.ExpiresAt)
	}

	return nil
}

// signLicense 签名授权证书
func (lm *LicenseManager) signLicense(license *License) (string, error) {
	// 序列化授权信息（不包括签名）
	data := fmt.Sprintf("%s:%s:%s:%s:%s:%s",
		license.PluginID,
		license.UserID,
		license.DeviceID,
		license.Key,
		license.Type,
		license.ExpiresAt.Format(time.RFC3339),
	)

	// 计算哈希
	hash := sha256.Sum256([]byte(data))

	// RSA 签名
	signature, err := rsa.SignPKCS1v15(rand.Reader, lm.privateKey, 0, hash[:])
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", signature), nil
}

// verifySignature 验证签名
func (lm *LicenseManager) verifySignature(license *License) error {
	// 重新计算数据哈希
	data := fmt.Sprintf("%s:%s:%s:%s:%s:%s",
		license.PluginID,
		license.UserID,
		license.DeviceID,
		license.Key,
		license.Type,
		license.ExpiresAt.Format(time.RFC3339),
	)

	hash := sha256.Sum256([]byte(data))

	// 解析签名
	var signature []byte
	fmt.Sscanf(license.Signature, "%x", &signature)

	// 验证签名
	return rsa.VerifyPKCS1v15(lm.publicKey, 0, hash[:], signature)
}

// SaveLicense 保存授权证书
func (lm *LicenseManager) SaveLicense(license *License, path string) error {
	data, err := json.MarshalIndent(license, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// LoadLicense 加载授权证书
func (lm *LicenseManager) LoadLicense(path string) (*License, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var license License
	if err := json.Unmarshal(data, &license); err != nil {
		return nil, err
	}

	return &license, nil
}

// ExportPublicKey 导出公钥（用于市场服务器）
func (lm *LicenseManager) ExportPublicKey() (string, error) {
	pubKeyBytes, err := x509.MarshalPKIXPublicKey(lm.publicKey)
	if err != nil {
		return "", err
	}

	pubKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubKeyBytes,
	})

	return string(pubKeyPEM), nil
}
