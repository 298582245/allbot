package deps

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/klauspost/compress/zstd"
)

const (
	maxRuntimeDownloadSize  = 1200 * 1024 * 1024
	maxRuntimeExtractedSize = 3 * 1024 * 1024 * 1024
	maxRuntimeZipEntries    = 200000
)

type RuntimeDownloadResult struct {
	Runtime      string
	Version      string
	Architecture string
	Executable   string
	RootDir      string
	SourceURL    string
	SHA256       string
}

type RuntimeDownloader interface {
	EnsureRuntime(runtimeName, version, architecture string, force bool, options RuntimeDownloadOptions, progress RuntimeProfileInitProgressFunc) (RuntimeDownloadResult, error)
}

type HTTPRuntimeDownloader struct {
	rootDir string
	mu      sync.Mutex
}

type runtimeDownloadSpec struct {
	Runtime          string
	Version          string
	Architecture     string
	URL              string
	SHA256URL        string
	NuGetIndexURL    string
	ArchiveName      string
	RootDir          string
	Executable       string
	TrustedHosts     []string
	HashTrustedHosts []string
	AllowMissingHash bool
}

func NewHTTPRuntimeDownloader(rootDir string) *HTTPRuntimeDownloader {
	return &HTTPRuntimeDownloader{rootDir: rootDir}
}

func (d *HTTPRuntimeDownloader) EnsureRuntime(runtimeName, version, architecture string, force bool, options RuntimeDownloadOptions, progress RuntimeProfileInitProgressFunc) (RuntimeDownloadResult, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	runtimeName = normalizeRuntimeName(runtimeName)
	version = strings.TrimSpace(version)
	architecture = strings.ToLower(strings.TrimSpace(architecture))
	if runtimeName != "nodejs" && runtimeName != "python" {
		return RuntimeDownloadResult{}, fmt.Errorf("自动下载只支持 nodejs/python: %s", runtimeName)
	}
	if !isSafeRuntimeVersion(version) {
		return RuntimeDownloadResult{}, fmt.Errorf("运行环境版本号不合法: %s", version)
	}
	if !isSupportedRuntimeArchitecture(architecture) {
		return RuntimeDownloadResult{}, fmt.Errorf("运行环境架构不支持: %s", architecture)
	}
	client, err := runtimeHTTPClient(options)
	if err != nil {
		return RuntimeDownloadResult{}, err
	}
	spec, err := d.downloadSpec(client, runtimeName, version, architecture, options)
	if err != nil {
		return RuntimeDownloadResult{}, err
	}
	if fileExists(spec.Executable) && !force {
		reportRuntimeInitProgress(progress, "download", "托管解释器已存在，跳过下载", 60)
		return RuntimeDownloadResult{Runtime: runtimeName, Version: version, Architecture: architecture, Executable: spec.Executable, RootDir: spec.RootDir, SourceURL: spec.URL}, nil
	}
	if _, err := os.Stat(spec.RootDir); err == nil && !force {
		return RuntimeDownloadResult{}, fmt.Errorf("解释器目录已存在但不可用，请使用 force 重新下载: %s", spec.RootDir)
	} else if err != nil && !os.IsNotExist(err) {
		return RuntimeDownloadResult{}, err
	}
	if err := os.MkdirAll(filepath.Dir(spec.RootDir), 0755); err != nil {
		return RuntimeDownloadResult{}, err
	}
	stagingDir := fmt.Sprintf("%s.downloading.%d", spec.RootDir, time.Now().UnixNano())
	if err := os.RemoveAll(stagingDir); err != nil {
		return RuntimeDownloadResult{}, err
	}
	if err := os.MkdirAll(stagingDir, 0755); err != nil {
		return RuntimeDownloadResult{}, err
	}
	defer os.RemoveAll(stagingDir)

	archivePath := filepath.Join(stagingDir, spec.ArchiveName)
	if err := downloadFile(client, spec.URL, archivePath, spec.TrustedHosts, progress); err != nil {
		return RuntimeDownloadResult{}, err
	}
	reportRuntimeInitProgress(progress, "hash", "正在校验下载文件", 55)
	expectedHash, err := d.fetchExpectedHash(client, spec)
	if err != nil && !spec.AllowMissingHash {
		return RuntimeDownloadResult{}, err
	}
	actualHash, err := hashFile(archivePath, expectedHashAlgorithm(spec))
	if err != nil {
		return RuntimeDownloadResult{}, err
	}
	if expectedHash == "" && !spec.AllowMissingHash {
		return RuntimeDownloadResult{}, fmt.Errorf("未获取到可信哈希，停止自动下载")
	}
	if expectedHash != "" && !strings.EqualFold(expectedHash, actualHash) {
		return RuntimeDownloadResult{}, fmt.Errorf("下载文件哈希校验失败")
	}

	reportRuntimeInitProgress(progress, "extract", "正在解压解释器", 60)
	extractDir := filepath.Join(stagingDir, "extract")
	if err := extractRuntimeArchive(archivePath, extractDir); err != nil {
		return RuntimeDownloadResult{}, err
	}
	payloadRoot, err := singleChildDirOrSelf(extractDir)
	if err != nil {
		return RuntimeDownloadResult{}, err
	}
	backupDir := ""
	if force {
		if _, err := os.Stat(spec.RootDir); err == nil {
			backupDir = fmt.Sprintf("%s.backup.%d", spec.RootDir, time.Now().UnixNano())
			if err := os.Rename(spec.RootDir, backupDir); err != nil {
				return RuntimeDownloadResult{}, err
			}
		} else if err != nil && !os.IsNotExist(err) {
			return RuntimeDownloadResult{}, err
		}
	}
	moved := false
	if err := os.Rename(payloadRoot, spec.RootDir); err != nil {
		if copyErr := copyDir(payloadRoot, spec.RootDir); copyErr != nil {
			if backupDir != "" {
				_ = os.Rename(backupDir, spec.RootDir)
			}
			return RuntimeDownloadResult{}, fmt.Errorf("移动解释器目录失败: %w", copyErr)
		}
		moved = true
	} else {
		moved = true
	}
	if moved && backupDir != "" {
		_ = os.RemoveAll(backupDir)
	}
	if !fileExists(spec.Executable) {
		return RuntimeDownloadResult{}, fmt.Errorf("下载完成后未找到解释器: %s", spec.Executable)
	}
	if _, err := runRuntimeVersion(spec.Executable); err != nil {
		return RuntimeDownloadResult{}, fmt.Errorf("解释器完整性校验失败: %w", err)
	}
	return RuntimeDownloadResult{Runtime: runtimeName, Version: version, Architecture: architecture, Executable: spec.Executable, RootDir: spec.RootDir, SourceURL: spec.URL, SHA256: actualHash}, nil
}

func (d *HTTPRuntimeDownloader) downloadSpec(client *http.Client, runtimeName, version, architecture string, options RuntimeDownloadOptions) (runtimeDownloadSpec, error) {
	if runtimeName == "nodejs" {
		return d.nodeDownloadSpec(version, architecture, options)
	}
	return d.pythonDownloadSpec(client, version, architecture, options)
}

func runtimeHTTPClient(options RuntimeDownloadOptions) (*http.Client, error) {
	client := &http.Client{Timeout: 15 * time.Minute}
	proxyURL := strings.TrimSpace(options.ProxyURL)
	if proxyURL == "" {
		return client, nil
	}
	parsed, err := url.Parse(proxyURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, fmt.Errorf("代理地址必须是 HTTP/HTTPS URL 且包含 host")
	}
	client.Transport = &http.Transport{Proxy: http.ProxyURL(parsed)}
	return client, nil
}

func runtimeOptionOrDefault(value, fallback string) string {
	value = strings.TrimRight(strings.TrimSpace(value), "/")
	if value == "" {
		return fallback
	}
	return value
}

func runtimeURLHost(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return strings.ToLower(parsed.Hostname())
}

func appendTrustedHost(hosts []string, rawURL string) []string {
	host := runtimeURLHost(rawURL)
	if host == "" {
		return hosts
	}
	for _, item := range hosts {
		if strings.EqualFold(item, host) {
			return hosts
		}
	}
	return append(hosts, host)
}

func downloadFile(client *http.Client, sourceURL, targetPath string, trustedHosts []string, progress RuntimeProfileInitProgressFunc) error {
	if err := validateTrustedURL(sourceURL, trustedHosts); err != nil {
		return err
	}
	request, err := http.NewRequest(http.MethodGet, sourceURL, nil)
	if err != nil {
		return err
	}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if err := validateTrustedURL(response.Request.URL.String(), trustedHosts); err != nil {
		return err
	}
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("下载失败: HTTP %d", response.StatusCode)
	}
	if response.ContentLength > maxRuntimeDownloadSize {
		return fmt.Errorf("下载文件超过大小限制")
	}
	file, err := os.Create(targetPath)
	if err != nil {
		return err
	}
	defer file.Close()
	if err := copyDownloadWithProgress(file, response.Body, response.ContentLength, progress); err != nil {
		return err
	}
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if info.Size() > maxRuntimeDownloadSize {
		return fmt.Errorf("下载文件超过大小限制")
	}
	return nil
}

func copyDownloadWithProgress(writer io.Writer, reader io.Reader, totalBytes int64, progress RuntimeProfileInitProgressFunc) error {
	buffer := make([]byte, 1024*256)
	downloader := io.LimitReader(reader, maxRuntimeDownloadSize+1)
	var downloaded int64
	lastProgress := -1
	lastReport := time.Now()
	for {
		readBytes, readErr := downloader.Read(buffer)
		if readBytes > 0 {
			writtenBytes, writeErr := writer.Write(buffer[:readBytes])
			if writeErr != nil {
				return writeErr
			}
			if writtenBytes != readBytes {
				return io.ErrShortWrite
			}
			downloaded += int64(readBytes)
			if downloaded > maxRuntimeDownloadSize {
				return fmt.Errorf("下载文件超过大小限制")
			}
			percent := 15
			if totalBytes > 0 {
				percent = 15 + int(downloaded*40/totalBytes)
				if percent > 55 {
					percent = 55
				}
			}
			if progress != nil && (percent != lastProgress || time.Since(lastReport) >= time.Second) {
				progress(RuntimeProfileInitProgress{Stage: "download", Message: "正在下载托管解释器", Progress: percent, DownloadedBytes: downloaded, TotalBytes: totalBytes})
				lastProgress = percent
				lastReport = time.Now()
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return readErr
		}
	}
	return nil
}

func (d *HTTPRuntimeDownloader) fetchExpectedHash(client *http.Client, spec runtimeDownloadSpec) (string, error) {
	if spec.SHA256URL != "" {
		if err := validateTrustedURL(spec.SHA256URL, spec.HashTrustedHosts); err != nil {
			return "", err
		}
		request, err := http.NewRequest(http.MethodGet, spec.SHA256URL, nil)
		if err != nil {
			return "", err
		}
		response, err := client.Do(request)
		if err != nil {
			return "", err
		}
		defer response.Body.Close()
		if err := validateTrustedURL(response.Request.URL.String(), spec.HashTrustedHosts); err != nil {
			return "", err
		}
		if response.StatusCode != http.StatusOK {
			return "", fmt.Errorf("读取哈希失败: HTTP %d", response.StatusCode)
		}
		data, err := io.ReadAll(io.LimitReader(response.Body, 2*1024*1024))
		if err != nil {
			return "", err
		}
		return extractSHA256(string(data), spec.ArchiveName), nil
	}
	if spec.NuGetIndexURL != "" {
		return d.fetchNuGetPackageHash(client, spec)
	}
	return "", nil
}

func (d *HTTPRuntimeDownloader) fetchNuGetPackageHash(client *http.Client, spec runtimeDownloadSpec) (string, error) {
	data, err := d.fetchTrustedText(client, spec.NuGetIndexURL, spec.HashTrustedHosts, "读取 NuGet 元数据失败")
	if err != nil {
		return "", err
	}
	catalogURL, err := d.findNuGetCatalogEntryURL(client, data, spec)
	if err != nil {
		return "", err
	}
	catalogData, err := d.fetchTrustedText(client, catalogURL, spec.HashTrustedHosts, "读取 NuGet Catalog 元数据失败")
	if err != nil {
		return "", err
	}
	packageHash, packageHashAlgorithm, err := extractNuGetPackageHash(catalogData)
	if err != nil {
		return "", err
	}
	if strings.ToUpper(packageHashAlgorithm) != "SHA512" {
		return "", fmt.Errorf("NuGet 元数据哈希算法不支持: %s", packageHashAlgorithm)
	}
	decoded, err := base64.StdEncoding.DecodeString(packageHash)
	if err != nil {
		return "", err
	}
	if len(decoded) != sha512.Size {
		return "", fmt.Errorf("NuGet SHA512 哈希长度无效")
	}
	return hex.EncodeToString(decoded), nil
}

func (d *HTTPRuntimeDownloader) fetchTrustedText(client *http.Client, sourceURL string, trustedHosts []string, message string) ([]byte, error) {
	if err := validateTrustedURL(sourceURL, trustedHosts); err != nil {
		return nil, err
	}
	request, err := http.NewRequest(http.MethodGet, sourceURL, nil)
	if err != nil {
		return nil, err
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if err := validateTrustedURL(response.Request.URL.String(), trustedHosts); err != nil {
		return nil, err
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: HTTP %d", message, response.StatusCode)
	}
	return io.ReadAll(io.LimitReader(response.Body, 4*1024*1024))
}

func (d *HTTPRuntimeDownloader) findNuGetCatalogEntryURL(client *http.Client, data []byte, spec runtimeDownloadSpec) (string, error) {
	var root struct {
		Items []struct {
			Items []nugetRegistrationItem `json:"items"`
			URL   string                  `json:"@id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(data, &root); err != nil {
		return "", err
	}
	for _, page := range root.Items {
		if url := findNuGetCatalogEntryURLInItems(page.Items, spec.Version); url != "" {
			return url, nil
		}
		if page.URL == "" {
			continue
		}
		pageData, err := d.fetchTrustedText(client, page.URL, spec.HashTrustedHosts, "读取 NuGet 版本页失败")
		if err != nil {
			return "", err
		}
		var expanded struct {
			Items []nugetRegistrationItem `json:"items"`
		}
		if err := json.Unmarshal(pageData, &expanded); err != nil {
			return "", err
		}
		if url := findNuGetCatalogEntryURLInItems(expanded.Items, spec.Version); url != "" {
			return url, nil
		}
	}
	return "", fmt.Errorf("NuGet 元数据中未找到版本: %s", spec.Version)
}

type nugetRegistrationItem struct {
	CatalogEntry struct {
		URL     string `json:"@id"`
		Version string `json:"version"`
	} `json:"catalogEntry"`
}

func findNuGetCatalogEntryURLInItems(items []nugetRegistrationItem, version string) string {
	for _, item := range items {
		if strings.EqualFold(item.CatalogEntry.Version, version) && item.CatalogEntry.URL != "" {
			return item.CatalogEntry.URL
		}
	}
	return ""
}

func extractNuGetPackageHash(data []byte) (string, string, error) {
	var catalog struct {
		PackageHash          string `json:"packageHash"`
		PackageHashAlgorithm string `json:"packageHashAlgorithm"`
	}
	if err := json.Unmarshal(data, &catalog); err != nil {
		return "", "", err
	}
	if catalog.PackageHash == "" || catalog.PackageHashAlgorithm == "" {
		return "", "", fmt.Errorf("NuGet Catalog 元数据缺少包哈希")
	}
	return catalog.PackageHash, catalog.PackageHashAlgorithm, nil
}

func extractSHA256(content, archiveName string) string {
	for _, line := range strings.Split(content, "\n") {
		if !strings.Contains(line, archiveName) {
			continue
		}
		fields := strings.Fields(line)
		for _, field := range fields {
			if len(field) == 64 && isHex(field) {
				return field
			}
		}
	}
	return ""
}

func isHex(value string) bool {
	for _, char := range value {
		if (char >= '0' && char <= '9') || (char >= 'a' && char <= 'f') || (char >= 'A' && char <= 'F') {
			continue
		}
		return false
	}
	return true
}

func validateTrustedURL(rawURL string, trustedHosts []string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return fmt.Errorf("下载地址必须是 HTTPS 可信来源")
	}
	host := strings.ToLower(parsed.Hostname())
	for _, trusted := range trustedHosts {
		if host == strings.ToLower(trusted) {
			return nil
		}
	}
	return fmt.Errorf("下载来源不受信任: %s", host)
}

func expectedHashAlgorithm(spec runtimeDownloadSpec) string {
	if spec.NuGetIndexURL != "" {
		return "sha512"
	}
	return "sha256"
}

func hashFile(path, algorithm string) (string, error) {
	if strings.EqualFold(algorithm, "sha512") {
		return sha512File(path)
	}
	return sha256File(path)
}

func sha512File(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha512.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func sha256File(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func extractRuntimeArchive(archivePath, targetDir string) error {
	lowerPath := strings.ToLower(archivePath)
	if strings.HasSuffix(lowerPath, ".tar.gz") || strings.HasSuffix(lowerPath, ".tgz") {
		return untarGzipSafe(archivePath, targetDir)
	}
	if strings.HasSuffix(lowerPath, ".tar.zst") || strings.HasSuffix(lowerPath, ".tar.zstd") {
		return untarZstdSafe(archivePath, targetDir)
	}
	return unzipSafe(archivePath, targetDir)
}

func unzipSafe(zipPath, targetDir string) error {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer reader.Close()
	absTarget, err := filepath.Abs(targetDir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(absTarget, 0755); err != nil {
		return err
	}
	if len(reader.File) > maxRuntimeZipEntries {
		return fmt.Errorf("压缩包文件数量超过限制")
	}
	totalSize := uint64(0)
	for _, file := range reader.File {
		if !file.FileInfo().IsDir() {
			totalSize += file.UncompressedSize64
			if totalSize > maxRuntimeExtractedSize {
				return fmt.Errorf("压缩包解压体积超过限制")
			}
		}
		cleanName := filepath.Clean(file.Name)
		if filepath.IsAbs(cleanName) || strings.HasPrefix(cleanName, "..") {
			return fmt.Errorf("压缩包包含非法路径: %s", file.Name)
		}
		path := filepath.Join(absTarget, cleanName)
		absPath, err := filepath.Abs(path)
		if err != nil {
			return err
		}
		if absPath != absTarget && !strings.HasPrefix(absPath, absTarget+string(filepath.Separator)) {
			return fmt.Errorf("压缩包路径越界: %s", file.Name)
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(absPath, 0755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(absPath), 0755); err != nil {
			return err
		}
		input, err := file.Open()
		if err != nil {
			return err
		}
		output, err := os.OpenFile(absPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
		if err != nil {
			input.Close()
			return err
		}
		_, copyErr := io.Copy(output, input)
		closeInputErr := input.Close()
		closeOutputErr := output.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeInputErr != nil {
			return closeInputErr
		}
		if closeOutputErr != nil {
			return closeOutputErr
		}
	}
	return nil
}

func untarGzipSafe(archivePath, targetDir string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzipReader.Close()
	return untarSafe(gzipReader, targetDir)
}

func untarZstdSafe(archivePath, targetDir string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()
	zstdReader, err := zstd.NewReader(file)
	if err != nil {
		return err
	}
	defer zstdReader.Close()
	return untarSafe(zstdReader, targetDir)
}

func untarSafe(source io.Reader, targetDir string) error {
	reader := tar.NewReader(source)
	absTarget, err := filepath.Abs(targetDir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(absTarget, 0755); err != nil {
		return err
	}
	var totalSize int64
	var entries int
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		entries++
		if entries > maxRuntimeZipEntries {
			return fmt.Errorf("压缩包文件数量超过限制")
		}
		cleanName := filepath.Clean(header.Name)
		if filepath.IsAbs(cleanName) || strings.HasPrefix(cleanName, "..") {
			return fmt.Errorf("压缩包包含非法路径: %s", header.Name)
		}
		path := filepath.Join(absTarget, cleanName)
		absPath, err := filepath.Abs(path)
		if err != nil {
			return err
		}
		if absPath != absTarget && !strings.HasPrefix(absPath, absTarget+string(filepath.Separator)) {
			return fmt.Errorf("压缩包路径越界: %s", header.Name)
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(absPath, os.FileMode(header.Mode)&0755); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			totalSize += header.Size
			if totalSize > maxRuntimeExtractedSize {
				return fmt.Errorf("压缩包解压体积超过限制")
			}
			if err := os.MkdirAll(filepath.Dir(absPath), 0755); err != nil {
				return err
			}
			output, err := os.OpenFile(absPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.FileMode(header.Mode)&0755)
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(output, io.LimitReader(reader, header.Size))
			closeErr := output.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
		case tar.TypeSymlink:
			linkName := filepath.Clean(filepath.FromSlash(header.Linkname))
			if filepath.IsAbs(linkName) {
				return fmt.Errorf("压缩包包含非法符号链接: %s", header.Name)
			}
			linkTarget := filepath.Join(filepath.Dir(absPath), linkName)
			absLinkTarget, err := filepath.Abs(linkTarget)
			if err != nil {
				return err
			}
			if absLinkTarget != absTarget && !strings.HasPrefix(absLinkTarget, absTarget+string(filepath.Separator)) {
				return fmt.Errorf("压缩包符号链接路径越界: %s", header.Name)
			}
			if err := os.MkdirAll(filepath.Dir(absPath), 0755); err != nil {
				return err
			}
			if err := os.Symlink(header.Linkname, absPath); err != nil {
				data, readErr := os.ReadFile(absLinkTarget)
				if readErr != nil {
					return err
				}
				if writeErr := os.WriteFile(absPath, data, os.FileMode(header.Mode)&0755); writeErr != nil {
					return err
				}
			}
		}
	}
	return nil
}

func singleChildDirOrSelf(path string) (string, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return "", err
	}
	if len(entries) == 1 && entries[0].IsDir() {
		return filepath.Join(path, entries[0].Name()), nil
	}
	return path, nil
}

func copyDir(sourceDir, targetDir string) error {
	return filepath.WalkDir(sourceDir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}
		targetPath := filepath.Join(targetDir, relPath)
		if entry.IsDir() {
			return os.MkdirAll(targetPath, 0755)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		input, err := os.Open(path)
		if err != nil {
			return err
		}
		defer input.Close()
		output, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode())
		if err != nil {
			return err
		}
		defer output.Close()
		_, err = io.Copy(output, input)
		return err
	})
}

func managedRuntimeExecutableInRoot(runtimeName, architecture, root string) string {
	if runtimeName == "python" {
		if isWindowsRuntimeArchitecture(architecture) {
			return filepath.Join(root, "tools", "python.exe")
		}
		return filepath.Join(root, "bin", "python3")
	}
	if isWindowsRuntimeArchitecture(architecture) {
		return filepath.Join(root, "node.exe")
	}
	return filepath.Join(root, "bin", "node")
}
