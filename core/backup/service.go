package backup

import (
	"archive/zip"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	pathpkg "path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/allbot/allbot/core/config"
	"github.com/allbot/allbot/core/router"
)

const backupFilePrefix = "allbot-backup-"
const maxImportSize int64 = 512 << 20
const maxZipEntries = 20000
const maxZipEntrySize uint64 = 256 << 20
const maxZipTotalSize uint64 = 1024 << 20

type OSSUploader interface {
	Upload(ctx context.Context, file BackupFile, settings config.OSSBackupSettings) error
}

type Service struct {
	database   *config.Database
	pluginDir  string
	openAPIDir string
	uploader   OSSUploader
	now        func() time.Time

	createMu sync.Mutex
	runnerMu sync.Mutex
	stop     chan struct{}
	done     chan struct{}
}

type BackupFile struct {
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	Size      int64     `json:"size"`
	CreatedAt time.Time `json:"created_at"`
	Trigger   string    `json:"trigger,omitempty"`
	Includes  []string  `json:"includes,omitempty"`
}

type Status struct {
	Running   bool       `json:"running"`
	NextRunAt *time.Time `json:"next_run_at,omitempty"`
	LastRunAt *time.Time `json:"last_run_at,omitempty"`
	LastError string     `json:"last_error,omitempty"`
}

type Manifest struct {
	Version   int       `json:"version"`
	CreatedAt time.Time `json:"created_at"`
	Trigger   string    `json:"trigger"`
	Includes  []string  `json:"includes"`
	OSS       string    `json:"oss"`
}

type ImportOptions struct {
	Reader       io.Reader
	OriginalName string
	MaxSize      int64
}

type BackupSummary struct {
	HasData        bool   `json:"has_data"`
	HasPlugins     bool   `json:"has_plugins"`
	HasOpenAPIs    bool   `json:"has_openapis"`
	FileCount      int    `json:"file_count"`
	TotalSize      uint64 `json:"total_size"`
	CompressedSize uint64 `json:"compressed_size"`
	OriginalName   string `json:"original_name,omitempty"`
	ImportedName   string `json:"imported_name,omitempty"`
}

type ImportResult struct {
	File     BackupFile    `json:"file"`
	Manifest Manifest      `json:"manifest"`
	Summary  BackupSummary `json:"summary"`
	Warnings []string      `json:"warnings"`
}

type RestoreOptions struct {
	IncludeData     bool `json:"include_data"`
	IncludePlugins  bool `json:"include_plugins"`
	IncludeOpenAPIs bool `json:"include_openapis"`
	Confirm         bool `json:"confirm"`
}

type RestoreResult struct {
	Restored        []string   `json:"restored"`
	Snapshot        BackupFile `json:"snapshot"`
	RestartRequired bool       `json:"restart_required"`
	Warnings        []string   `json:"warnings"`
}

func NewService(database *config.Database, pluginDir string) *Service {
	return &Service{database: database, pluginDir: pluginDir, openAPIDir: "openapis", now: time.Now}
}

func (s *Service) SetOSSUploader(uploader OSSUploader) {
	if s == nil {
		return
	}
	s.runnerMu.Lock()
	defer s.runnerMu.Unlock()
	s.uploader = uploader
}

func (s *Service) Start() {
	if s == nil || s.database == nil {
		return
	}
	s.runnerMu.Lock()
	defer s.runnerMu.Unlock()
	if s.stop != nil {
		return
	}
	s.stop = make(chan struct{})
	s.done = make(chan struct{})
	go s.loop(s.stop, s.done)
}

func (s *Service) Stop() {
	if s == nil {
		return
	}
	s.runnerMu.Lock()
	stop := s.stop
	done := s.done
	s.stop = nil
	s.done = nil
	s.runnerMu.Unlock()
	if stop == nil {
		return
	}
	close(stop)
	<-done
}

func (s *Service) Reload() {
	if s == nil {
		return
	}
	s.Stop()
	s.Start()
}

func (s *Service) Status() Status {
	if s == nil || s.database == nil {
		return Status{}
	}
	settings, err := s.database.GetBackupSettings()
	if err != nil || !settings.Enabled {
		return Status{Running: s.isRunning(), LastError: errorString(err)}
	}
	next, err := router.NextCronTime(settings.Cron, s.now())
	if err != nil {
		return Status{Running: s.isRunning(), LastError: err.Error()}
	}
	return Status{Running: s.isRunning(), NextRunAt: &next}
}

func (s *Service) Create(ctx context.Context, trigger string) (BackupFile, error) {
	if s == nil || s.database == nil {
		return BackupFile{}, fmt.Errorf("备份服务未初始化")
	}
	settings, err := s.database.GetBackupSettings()
	if err != nil {
		return BackupFile{}, err
	}
	return s.createWithSettings(ctx, trigger, config.NormalizeBackupSettings(settings), true)
}

func (s *Service) createWithSettings(ctx context.Context, trigger string, settings config.BackupSettings, cleanup bool) (BackupFile, error) {
	if s == nil || s.database == nil {
		return BackupFile{}, fmt.Errorf("备份服务未初始化")
	}
	s.createMu.Lock()
	defer s.createMu.Unlock()

	settings = config.NormalizeBackupSettings(settings)
	if !settings.IncludePlugins && !settings.IncludeData {
		return BackupFile{}, fmt.Errorf("至少需要选择插件或数据中的一项")
	}
	backupDir, err := normalizePath(settings.BackupDir)
	if err != nil {
		return BackupFile{}, err
	}
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return BackupFile{}, err
	}

	createdAt := s.now()
	fileName := fmt.Sprintf("%s%s.zip", backupFilePrefix, createdAt.Format("20060102-150405"))
	backupPath := filepath.Join(backupDir, fileName)
	if _, err := os.Stat(backupPath); err == nil {
		fileName = fmt.Sprintf("%s%s-%d.zip", backupFilePrefix, createdAt.Format("20060102-150405"), createdAt.UnixNano())
		backupPath = filepath.Join(backupDir, fileName)
	} else if !os.IsNotExist(err) {
		return BackupFile{}, err
	}

	stagingDir, err := os.MkdirTemp(backupDir, ".allbot-backup-")
	if err != nil {
		return BackupFile{}, err
	}
	defer os.RemoveAll(stagingDir)

	includes := make([]string, 0, 3)
	if settings.IncludeData {
		snapshotPath := filepath.Join(stagingDir, "config.db")
		if err := s.database.SnapshotTo(snapshotPath); err != nil {
			return BackupFile{}, fmt.Errorf("生成数据库快照失败: %w", err)
		}
		includes = append(includes, "data")
	}
	if settings.IncludePlugins {
		includes = append(includes, "plugins")
	}

	manifest := Manifest{Version: 1, CreatedAt: createdAt, Trigger: strings.TrimSpace(trigger), Includes: includes, OSS: "reserved"}
	tmpZipPath := backupPath + ".tmp"
	if err := s.writeZip(tmpZipPath, stagingDir, backupDir, settings, manifest); err != nil {
		_ = os.Remove(tmpZipPath)
		return BackupFile{}, err
	}
	if err := os.Rename(tmpZipPath, backupPath); err != nil {
		_ = os.Remove(tmpZipPath)
		return BackupFile{}, err
	}

	info, err := os.Stat(backupPath)
	if err != nil {
		return BackupFile{}, err
	}
	file := BackupFile{Name: fileName, Path: backupPath, Size: info.Size(), CreatedAt: createdAt, Trigger: trigger, Includes: includes}
	if settings.OSS.Enabled {
		if s.uploader == nil {
			log.Printf("[SYSTEM] OSS 备份接口已预留，当前未配置上传实现: %s", fileName)
		} else if err := s.uploader.Upload(ctx, file, settings.OSS); err != nil {
			log.Printf("[SYSTEM] OSS 备份上传失败: %v", err)
		}
	}
	if cleanup {
		if err := s.Cleanup(settings.Retention); err != nil {
			log.Printf("[SYSTEM] 清理旧备份失败: %v", err)
		}
	}
	return file, nil
}

func (s *Service) Import(ctx context.Context, options ImportOptions) (ImportResult, error) {
	if s == nil || s.database == nil {
		return ImportResult{}, fmt.Errorf("备份服务未初始化")
	}
	if options.Reader == nil {
		return ImportResult{}, fmt.Errorf("备份文件不能为空")
	}
	limit := options.MaxSize
	if limit <= 0 {
		limit = maxImportSize
	}
	settings, err := s.database.GetBackupSettings()
	if err != nil {
		return ImportResult{}, err
	}
	backupDir, err := normalizePath(settings.BackupDir)
	if err != nil {
		return ImportResult{}, err
	}
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return ImportResult{}, err
	}

	tmpFile, err := os.CreateTemp(backupDir, ".allbot-import-*.zip")
	if err != nil {
		return ImportResult{}, err
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)
	written, copyErr := io.Copy(tmpFile, io.LimitReader(options.Reader, limit+1))
	closeErr := tmpFile.Close()
	if copyErr != nil {
		return ImportResult{}, copyErr
	}
	if closeErr != nil {
		return ImportResult{}, closeErr
	}
	if written == 0 {
		return ImportResult{}, fmt.Errorf("备份文件不能为空")
	}
	if written > limit {
		return ImportResult{}, fmt.Errorf("备份文件超过大小限制")
	}

	manifest, summary, warnings, err := s.Inspect(tmpPath)
	if err != nil {
		return ImportResult{}, err
	}
	createdAt := s.now()
	fileName := fmt.Sprintf("%simport-%s.zip", backupFilePrefix, createdAt.Format("20060102-150405"))
	backupPath := filepath.Join(backupDir, fileName)
	if _, err := os.Stat(backupPath); err == nil {
		fileName = fmt.Sprintf("%simport-%s-%d.zip", backupFilePrefix, createdAt.Format("20060102-150405"), createdAt.UnixNano())
		backupPath = filepath.Join(backupDir, fileName)
	} else if !os.IsNotExist(err) {
		return ImportResult{}, err
	}
	if err := os.Rename(tmpPath, backupPath); err != nil {
		return ImportResult{}, err
	}
	info, err := os.Stat(backupPath)
	if err != nil {
		return ImportResult{}, err
	}
	summary.OriginalName = filepath.Base(options.OriginalName)
	summary.ImportedName = fileName
	file := BackupFile{Name: fileName, Path: backupPath, Size: info.Size(), CreatedAt: info.ModTime(), Trigger: "import", Includes: manifest.Includes}
	select {
	case <-ctx.Done():
		return ImportResult{}, ctx.Err()
	default:
	}
	return ImportResult{File: file, Manifest: manifest, Summary: summary, Warnings: warnings}, nil
}

func (s *Service) Inspect(path string) (Manifest, BackupSummary, []string, error) {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return Manifest{}, BackupSummary{}, nil, fmt.Errorf("备份包无法读取: %w", err)
	}
	defer reader.Close()
	return inspectZipFiles(reader.File)
}

func (s *Service) Restore(ctx context.Context, name string, options RestoreOptions) (RestoreResult, error) {
	if s == nil || s.database == nil {
		return RestoreResult{}, fmt.Errorf("备份服务未初始化")
	}
	if !options.Confirm {
		return RestoreResult{}, fmt.Errorf("请先确认恢复会覆盖当前数据")
	}
	if !options.IncludeData && !options.IncludePlugins && !options.IncludeOpenAPIs {
		return RestoreResult{}, fmt.Errorf("至少需要选择一项恢复内容")
	}
	file, err := s.Resolve(name)
	if err != nil {
		return RestoreResult{}, err
	}
	_, summary, warnings, err := s.Inspect(file.Path)
	if err != nil {
		return RestoreResult{}, err
	}
	if options.IncludeData && !summary.HasData {
		return RestoreResult{}, fmt.Errorf("备份包不包含数据库")
	}
	if options.IncludePlugins && !summary.HasPlugins {
		return RestoreResult{}, fmt.Errorf("备份包不包含插件目录")
	}
	if options.IncludeOpenAPIs && !summary.HasOpenAPIs {
		return RestoreResult{}, fmt.Errorf("备份包不包含 OpenAPI 文件")
	}

	snapshot, err := s.CreateFullSnapshot(ctx, "pre-restore")
	if err != nil {
		return RestoreResult{}, fmt.Errorf("创建恢复前快照失败: %w", err)
	}
	settings, err := s.database.GetBackupSettings()
	if err != nil {
		return RestoreResult{}, err
	}
	backupDir, err := normalizePath(settings.BackupDir)
	if err != nil {
		return RestoreResult{}, err
	}
	stagingDir, err := os.MkdirTemp(backupDir, ".allbot-restore-")
	if err != nil {
		return RestoreResult{}, err
	}
	defer os.RemoveAll(stagingDir)
	if err := extractBackup(file.Path, stagingDir); err != nil {
		return RestoreResult{}, err
	}

	restored := make([]string, 0, 3)
	if options.IncludeData {
		dbPath := filepath.Join(stagingDir, "data", "config.db")
		if err := validateSQLiteDatabase(dbPath); err != nil {
			return RestoreResult{}, fmt.Errorf("数据库校验失败: %w", err)
		}
		if err := s.database.ReplaceWith(dbPath); err != nil {
			return RestoreResult{}, fmt.Errorf("恢复数据库失败: %w", err)
		}
		restored = append(restored, "data")
	}
	if options.IncludePlugins {
		if err := replaceDirectory(filepath.Join(stagingDir, "plugins"), s.pluginDir); err != nil {
			return RestoreResult{}, fmt.Errorf("恢复插件目录失败: %w", err)
		}
		restored = append(restored, "plugins")
	}
	if options.IncludeOpenAPIs {
		if err := replaceDirectory(filepath.Join(stagingDir, "openapis"), s.openAPIDir); err != nil {
			return RestoreResult{}, fmt.Errorf("恢复 OpenAPI 文件失败: %w", err)
		}
		restored = append(restored, "openapis")
	}
	return RestoreResult{Restored: restored, Snapshot: snapshot, RestartRequired: true, Warnings: warnings}, nil
}

func (s *Service) CreateFullSnapshot(ctx context.Context, trigger string) (BackupFile, error) {
	settings, err := s.database.GetBackupSettings()
	if err != nil {
		return BackupFile{}, err
	}
	settings = config.NormalizeBackupSettings(settings)
	settings.IncludeData = true
	settings.IncludePlugins = true
	return s.createWithSettings(ctx, trigger, settings, false)
}

func (s *Service) List() ([]BackupFile, error) {
	if s == nil || s.database == nil {
		return nil, fmt.Errorf("备份服务未初始化")
	}
	settings, err := s.database.GetBackupSettings()
	if err != nil {
		return nil, err
	}
	backupDir, err := normalizePath(settings.BackupDir)
	if err != nil {
		return nil, err
	}
	items, err := listBackupFiles(backupDir)
	if os.IsNotExist(err) {
		return []BackupFile{}, nil
	}
	return items, err
}

func (s *Service) Cleanup(retention int) error {
	if s == nil || s.database == nil || retention <= 0 {
		return nil
	}
	settings, err := s.database.GetBackupSettings()
	if err != nil {
		return err
	}
	backupDir, err := normalizePath(settings.BackupDir)
	if err != nil {
		return err
	}
	items, err := listBackupFiles(backupDir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for index, item := range items {
		if index < retention {
			continue
		}
		if err := os.Remove(item.Path); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func (s *Service) Resolve(name string) (BackupFile, error) {
	if s == nil || s.database == nil {
		return BackupFile{}, fmt.Errorf("备份服务未初始化")
	}
	name = cleanBackupName(name)
	if name == "" {
		return BackupFile{}, fmt.Errorf("备份文件名无效")
	}
	settings, err := s.database.GetBackupSettings()
	if err != nil {
		return BackupFile{}, err
	}
	backupDir, err := normalizePath(settings.BackupDir)
	if err != nil {
		return BackupFile{}, err
	}
	path := filepath.Join(backupDir, name)
	info, err := os.Stat(path)
	if err != nil {
		return BackupFile{}, err
	}
	if info.IsDir() {
		return BackupFile{}, fmt.Errorf("备份文件名无效")
	}
	return BackupFile{Name: name, Path: path, Size: info.Size(), CreatedAt: info.ModTime()}, nil
}

func (s *Service) Delete(name string) error {
	file, err := s.Resolve(name)
	if err != nil {
		return err
	}
	return os.Remove(file.Path)
}

func (s *Service) loop(stop <-chan struct{}, done chan<- struct{}) {
	defer close(done)
	for {
		settings, err := s.database.GetBackupSettings()
		if err != nil {
			log.Printf("[SYSTEM] 加载备份配置失败: %v", err)
			if waitOrStop(time.Minute, stop) {
				return
			}
			continue
		}
		settings = config.NormalizeBackupSettings(settings)
		if !settings.Enabled {
			if waitOrStop(time.Minute, stop) {
				return
			}
			continue
		}
		next, err := router.NextCronTime(settings.Cron, s.now())
		if err != nil {
			log.Printf("[SYSTEM] 备份定时表达式无效: %v", err)
			if waitOrStop(time.Minute, stop) {
				return
			}
			continue
		}
		if waitOrStop(time.Until(next), stop) {
			return
		}
		if _, err := s.Create(context.Background(), "scheduled"); err != nil {
			log.Printf("[SYSTEM] 定时备份失败: %v", err)
		} else {
			log.Printf("[SYSTEM] 定时备份完成")
		}
	}
}

func (s *Service) isRunning() bool {
	s.runnerMu.Lock()
	defer s.runnerMu.Unlock()
	return s.stop != nil
}

func (s *Service) writeZip(zipPath, stagingDir, backupDir string, settings config.BackupSettings, manifest Manifest) error {
	zipFile, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	defer zipFile.Close()
	writer := zip.NewWriter(zipFile)
	defer writer.Close()

	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	if err := addBytes(writer, "manifest.json", manifestData); err != nil {
		return err
	}
	if settings.IncludeData {
		if err := addFile(writer, filepath.Join(stagingDir, "config.db"), "data/config.db"); err != nil {
			return err
		}
		if err := addDirIfExists(writer, s.openAPIDir, "openapis", backupDir); err != nil {
			return err
		}
	}
	if settings.IncludePlugins {
		if err := addDirIfExists(writer, s.pluginDir, "plugins", backupDir); err != nil {
			return err
		}
	}
	return nil
}

func inspectZipFiles(files []*zip.File) (Manifest, BackupSummary, []string, error) {
	if len(files) == 0 {
		return Manifest{}, BackupSummary{}, nil, fmt.Errorf("备份包为空")
	}
	if len(files) > maxZipEntries {
		return Manifest{}, BackupSummary{}, nil, fmt.Errorf("备份包文件数量超过限制")
	}
	var manifest Manifest
	manifestFound := false
	summary := BackupSummary{}
	warnings := make([]string, 0)
	for _, file := range files {
		cleanName, isDir, err := validateZipEntry(file)
		if err != nil {
			return Manifest{}, BackupSummary{}, nil, err
		}
		if isDir {
			continue
		}
		if file.UncompressedSize64 > maxZipEntrySize {
			return Manifest{}, BackupSummary{}, nil, fmt.Errorf("备份包单文件超过限制: %s", cleanName)
		}
		summary.FileCount++
		summary.TotalSize += file.UncompressedSize64
		summary.CompressedSize += file.CompressedSize64
		if summary.TotalSize > maxZipTotalSize {
			return Manifest{}, BackupSummary{}, nil, fmt.Errorf("备份包解压后大小超过限制")
		}
		switch {
		case cleanName == "manifest.json":
			if manifestFound {
				return Manifest{}, BackupSummary{}, nil, fmt.Errorf("备份包包含重复 manifest.json")
			}
			data, err := readZipFileLimited(file, maxZipEntrySize)
			if err != nil {
				return Manifest{}, BackupSummary{}, nil, err
			}
			if err := json.Unmarshal(data, &manifest); err != nil {
				return Manifest{}, BackupSummary{}, nil, fmt.Errorf("manifest.json 无法解析: %w", err)
			}
			manifestFound = true
		case cleanName == "data/config.db":
			summary.HasData = true
		case strings.HasPrefix(cleanName, "plugins/"):
			summary.HasPlugins = true
		case strings.HasPrefix(cleanName, "openapis/"):
			summary.HasOpenAPIs = true
		}
	}
	if !manifestFound {
		return Manifest{}, BackupSummary{}, nil, fmt.Errorf("备份包缺少 manifest.json")
	}
	if manifest.Version != 1 {
		return Manifest{}, BackupSummary{}, nil, fmt.Errorf("备份版本不支持: %d", manifest.Version)
	}
	if !summary.HasData {
		warnings = append(warnings, "备份包不包含数据库")
	}
	return manifest, summary, warnings, nil
}

func validateZipEntry(file *zip.File) (string, bool, error) {
	name := strings.TrimSpace(file.Name)
	if name == "" {
		return "", false, fmt.Errorf("备份包包含空路径")
	}
	if strings.Contains(name, "\\") {
		return "", false, fmt.Errorf("备份包包含非法路径: %s", name)
	}
	if strings.HasPrefix(name, "/") || filepath.IsAbs(name) || hasWindowsDrive(name) {
		return "", false, fmt.Errorf("备份包包含绝对路径: %s", name)
	}
	cleanName := pathpkg.Clean(name)
	if cleanName == "." || strings.HasPrefix(cleanName, "../") || cleanName == ".." {
		return "", false, fmt.Errorf("备份包包含路径穿越: %s", name)
	}
	mode := file.FileInfo().Mode()
	if mode&os.ModeSymlink != 0 || mode&os.ModeType != 0 && !mode.IsRegular() && !mode.IsDir() {
		return "", false, fmt.Errorf("备份包包含不支持的文件类型: %s", cleanName)
	}
	if cleanName == "manifest.json" {
		return cleanName, false, nil
	}
	if cleanName == "data/config.db" {
		return cleanName, false, nil
	}
	if file.FileInfo().IsDir() && (cleanName == "data" || cleanName == "plugins" || cleanName == "openapis") {
		return cleanName, true, nil
	}
	if strings.HasPrefix(cleanName, "plugins/") || strings.HasPrefix(cleanName, "openapis/") {
		return cleanName, file.FileInfo().IsDir(), nil
	}
	return "", false, fmt.Errorf("备份包包含未知路径: %s", cleanName)
}

func readZipFileLimited(file *zip.File, limit uint64) ([]byte, error) {
	reader, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return io.ReadAll(io.LimitReader(reader, int64(limit)+1))
}

func extractBackup(zipPath, stagingDir string) error {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer reader.Close()
	if _, _, _, err := inspectZipFiles(reader.File); err != nil {
		return err
	}
	for _, file := range reader.File {
		cleanName, isDir, err := validateZipEntry(file)
		if err != nil {
			return err
		}
		if cleanName == "manifest.json" {
			continue
		}
		targetPath, err := safeJoin(stagingDir, cleanName)
		if err != nil {
			return err
		}
		if isDir {
			if err := os.MkdirAll(targetPath, 0755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return err
		}
		reader, err := file.Open()
		if err != nil {
			return err
		}
		writer, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, file.FileInfo().Mode().Perm())
		if err != nil {
			reader.Close()
			return err
		}
		_, copyErr := io.Copy(writer, io.LimitReader(reader, int64(maxZipEntrySize)+1))
		closeReaderErr := reader.Close()
		closeWriterErr := writer.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeReaderErr != nil {
			return closeReaderErr
		}
		if closeWriterErr != nil {
			return closeWriterErr
		}
	}
	return nil
}

func safeJoin(root, name string) (string, error) {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	path := filepath.Join(rootAbs, filepath.FromSlash(name))
	pathAbs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if !isPathInside(pathAbs, rootAbs) {
		return "", fmt.Errorf("解压路径越界: %s", name)
	}
	return pathAbs, nil
}

func replaceDirectory(sourceDir, targetDir string) error {
	info, err := os.Stat(sourceDir)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("恢复来源不是目录: %s", sourceDir)
	}
	targetAbs, err := filepath.Abs(filepath.Clean(targetDir))
	if err != nil {
		return err
	}
	if targetAbs == string(filepath.Separator) || strings.TrimSpace(targetDir) == "" {
		return fmt.Errorf("恢复目标目录无效")
	}
	if err := os.MkdirAll(filepath.Dir(targetAbs), 0755); err != nil {
		return err
	}
	backupOld := targetAbs + ".restore-old-" + time.Now().Format("20060102150405")
	oldExists := false
	if _, err := os.Stat(targetAbs); err == nil {
		oldExists = true
		if err := os.Rename(targetAbs, backupOld); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.Rename(sourceDir, targetAbs); err != nil {
		if oldExists {
			_ = os.Rename(backupOld, targetAbs)
		}
		return err
	}
	if oldExists {
		_ = os.RemoveAll(backupOld)
	}
	return nil
}

func validateSQLiteDatabase(path string) error {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return err
	}
	defer db.Close()
	var result string
	if err := db.QueryRow(`PRAGMA integrity_check`).Scan(&result); err != nil {
		return err
	}
	if result != "ok" {
		return fmt.Errorf("integrity_check: %s", result)
	}
	var tableName string
	if err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='system_settings'`).Scan(&tableName); err != nil {
		return fmt.Errorf("缺少 system_settings 表: %w", err)
	}
	return nil
}

func hasWindowsDrive(name string) bool {
	return len(name) >= 2 && name[1] == ':' && ((name[0] >= 'A' && name[0] <= 'Z') || (name[0] >= 'a' && name[0] <= 'z'))
}

func addDirIfExists(writer *zip.Writer, sourceDir, zipRoot, excludedDir string) error {
	if strings.TrimSpace(sourceDir) == "" {
		return nil
	}
	info, err := os.Stat(sourceDir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return nil
	}
	sourceAbs, err := filepath.Abs(sourceDir)
	if err != nil {
		return err
	}
	excludedAbs := ""
	if candidate, err := filepath.Abs(excludedDir); err == nil && isPathInside(candidate, sourceAbs) {
		excludedAbs = candidate
	}
	return filepath.WalkDir(sourceAbs, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if shouldSkipPath(path, excludedAbs) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		relPath, err := filepath.Rel(sourceAbs, path)
		if err != nil {
			return err
		}
		return addFile(writer, path, filepath.ToSlash(filepath.Join(zipRoot, relPath)))
	})
}

func addFile(writer *zip.Writer, sourcePath, zipName string) error {
	info, err := os.Stat(sourcePath)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return nil
	}
	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	header.Name = filepath.ToSlash(zipName)
	header.Method = zip.Deflate
	fileWriter, err := writer.CreateHeader(header)
	if err != nil {
		return err
	}
	file, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(fileWriter, file)
	return err
}

func addBytes(writer *zip.Writer, zipName string, data []byte) error {
	header := &zip.FileHeader{Name: filepath.ToSlash(zipName), Method: zip.Deflate}
	header.SetModTime(time.Now())
	fileWriter, err := writer.CreateHeader(header)
	if err != nil {
		return err
	}
	_, err = fileWriter.Write(data)
	return err
}

func listBackupFiles(backupDir string) ([]BackupFile, error) {
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		return nil, err
	}
	items := make([]BackupFile, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if cleanBackupName(name) == "" {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return nil, err
		}
		items = append(items, BackupFile{Name: name, Path: filepath.Join(backupDir, name), Size: info.Size(), CreatedAt: info.ModTime()})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.After(items[j].CreatedAt) })
	return items, nil
}

func cleanBackupName(name string) string {
	name = filepath.Base(filepath.Clean(strings.TrimSpace(name)))
	if !strings.HasPrefix(name, backupFilePrefix) || !strings.HasSuffix(strings.ToLower(name), ".zip") {
		return ""
	}
	return name
}

func normalizePath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		path = "./backups"
	}
	return filepath.Abs(filepath.Clean(path))
}

func shouldSkipPath(path, excludedDir string) bool {
	if excludedDir == "" {
		return false
	}
	pathAbs, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	return isPathInside(pathAbs, excludedDir)
}

func isPathInside(path, parent string) bool {
	rel, err := filepath.Rel(parent, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func waitOrStop(duration time.Duration, stop <-chan struct{}) bool {
	if duration < 0 {
		duration = 0
	}
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-timer.C:
		return false
	case <-stop:
		return true
	}
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
