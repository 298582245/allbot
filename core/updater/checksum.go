package updater

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func SelectChecksumAsset(assets []ReleaseAsset, version string) (ReleaseAsset, bool) {
	version = strings.TrimSpace(version)
	if version == "" {
		return ReleaseAsset{}, false
	}
	version = strings.TrimPrefix(strings.TrimPrefix(version, "v"), "V")
	expected := "checksums-v" + version + ".txt"
	for _, asset := range assets {
		if strings.EqualFold(strings.TrimSpace(asset.Name), expected) {
			return asset, true
		}
	}
	return ReleaseAsset{}, false
}

type ChecksumFile struct {
	Items map[string]string
}

func DownloadChecksumFile(ctx context.Context, asset ReleaseAsset) (ChecksumFile, error) {
	url := strings.TrimSpace(asset.DownloadURL)
	if url == "" {
		return ChecksumFile{}, fmt.Errorf("checksum 下载地址不能为空")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return ChecksumFile{}, err
	}
	request.Header.Set("User-Agent", "AllBot-Updater")
	client := &http.Client{Timeout: 2 * time.Minute}
	response, err := client.Do(request)
	if err != nil {
		return ChecksumFile{}, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return ChecksumFile{}, fmt.Errorf("checksum 下载状态码 %d", response.StatusCode)
	}
	return ParseChecksumFile(response.Body)
}

func ParseChecksumFile(reader io.Reader) (ChecksumFile, error) {
	result := ChecksumFile{Items: map[string]string{}}
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			return result, fmt.Errorf("checksum 行格式无效: %s", line)
		}
		sum := strings.ToLower(strings.TrimSpace(parts[0]))
		if len(sum) != sha256.Size*2 {
			return result, fmt.Errorf("checksum 长度无效: %s", sum)
		}
		if _, err := hex.DecodeString(sum); err != nil {
			return result, fmt.Errorf("checksum 不是有效十六进制: %w", err)
		}
		name := strings.TrimPrefix(strings.Join(parts[1:], " "), "*")
		name = strings.TrimSpace(name)
		if name == "" {
			return result, fmt.Errorf("checksum 文件名不能为空")
		}
		result.Items[filepath.Base(name)] = sum
	}
	if err := scanner.Err(); err != nil {
		return result, err
	}
	if len(result.Items) == 0 {
		return result, fmt.Errorf("checksum 文件为空")
	}
	return result, nil
}

func (c ChecksumFile) ExpectedSHA256(assetName string) (string, bool) {
	if c.Items == nil {
		return "", false
	}
	sum, ok := c.Items[filepath.Base(strings.TrimSpace(assetName))]
	return sum, ok
}

func VerifyFileSHA256(path string, expected string) error {
	expected = strings.ToLower(strings.TrimSpace(expected))
	if len(expected) != sha256.Size*2 {
		return fmt.Errorf("期望 checksum 长度无效")
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return err
	}
	actual := hex.EncodeToString(hash.Sum(nil))
	if actual != expected {
		return fmt.Errorf("SHA256 校验失败: got %s want %s", actual, expected)
	}
	return nil
}
