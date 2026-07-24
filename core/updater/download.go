package updater

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Downloader struct {
	HTTPClient *http.Client
}

func (d Downloader) Download(ctx context.Context, asset ReleaseAsset, targetPath string) error {
	url := strings.TrimSpace(asset.DownloadURL)
	if url == "" {
		return fmt.Errorf("下载地址不能为空")
	}
	if strings.TrimSpace(targetPath) == "" {
		return fmt.Errorf("目标文件不能为空")
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0700); err != nil {
		return err
	}
	if info, err := os.Lstat(targetPath); err == nil {
		return fmt.Errorf("目标文件已存在: %s", info.Name())
	} else if !os.IsNotExist(err) {
		return err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	request.Header.Set("User-Agent", "AllBot-Updater")

	httpClient := d.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Minute}
	}
	response, err := httpClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("下载状态码 %d", response.StatusCode)
	}

	tempFile, err := os.CreateTemp(filepath.Dir(targetPath), ".download-*")
	if err != nil {
		return err
	}
	tempPath := tempFile.Name()
	if err := tempFile.Chmod(0700); err != nil {
		_ = tempFile.Close()
		_ = os.Remove(tempPath)
		return err
	}
	_, copyErr := io.Copy(tempFile, response.Body)
	closeErr := tempFile.Close()
	if copyErr != nil {
		_ = os.Remove(tempPath)
		return copyErr
	}
	if closeErr != nil {
		_ = os.Remove(tempPath)
		return closeErr
	}
	if asset.Size > 0 {
		info, err := os.Stat(tempPath)
		if err != nil {
			_ = os.Remove(tempPath)
			return err
		}
		if info.Size() != asset.Size {
			_ = os.Remove(tempPath)
			return fmt.Errorf("下载文件大小不一致: got %d want %d", info.Size(), asset.Size)
		}
	}
	return os.Rename(tempPath, targetPath)
}
