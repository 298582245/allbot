package updater

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const updatePublicKeyEnv = "ALLBOT_UPDATE_ED25519_PUBLIC_KEY"

// defaultUpdatePublicKey 为官方发布的内嵌更新签名公钥（base64），
// 使普通用户无需任何环境变量即可完成升级验签。公钥属于公开信息，
// 内嵌无安全风险；自建者可通过 ALLBOT_UPDATE_ED25519_PUBLIC_KEY 覆盖为自己的公钥。
const defaultUpdatePublicKey = "6jGa1YFHNcF6+DVNaos9FmBOBr15xQcr6ZCTIev6QuM="

func SelectSignatureAsset(assets []ReleaseAsset, checksumAsset ReleaseAsset) (ReleaseAsset, bool) {
	expected := strings.TrimSpace(checksumAsset.Name) + ".sig"
	if expected == ".sig" {
		return ReleaseAsset{}, false
	}
	for _, asset := range assets {
		if strings.EqualFold(strings.TrimSpace(asset.Name), expected) {
			return asset, true
		}
	}
	return ReleaseAsset{}, false
}

func trustedUpdatePublicKey() (ed25519.PublicKey, error) {
	// 优先使用环境变量覆盖（供自建者使用自己的密钥），为空时回退到内嵌官方公钥。
	encoded := strings.TrimSpace(os.Getenv(updatePublicKeyEnv))
	if encoded == "" {
		encoded = strings.TrimSpace(defaultUpdatePublicKey)
	}
	if encoded == "" {
		return nil, fmt.Errorf("未配置独立更新签名公钥 %s", updatePublicKeyEnv)
	}
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(decoded) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("更新签名公钥格式无效")
	}
	return ed25519.PublicKey(decoded), nil
}

func verifyUpdateSignature(publicKey ed25519.PublicKey, encodedSignature []byte, payload []byte) error {
	signature, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(encodedSignature)))
	if err != nil || len(signature) != ed25519.SignatureSize {
		return fmt.Errorf("更新签名格式无效")
	}
	if !ed25519.Verify(publicKey, payload, signature) {
		return fmt.Errorf("更新签名验证失败")
	}
	return nil
}

func downloadSmallReleaseAsset(ctx context.Context, asset ReleaseAsset, maxBytes int64) ([]byte, error) {
	url := strings.TrimSpace(asset.DownloadURL)
	if url == "" {
		return nil, fmt.Errorf("发布资产下载地址不能为空")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("User-Agent", "AllBot-Updater")
	response, err := (&http.Client{Timeout: 2 * time.Minute}).Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("发布资产下载状态码 %d", response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("发布资产超过大小限制")
	}
	return data, nil
}
