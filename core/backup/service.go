package backup

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/allbot/allbot/core/config"
	"github.com/allbot/allbot/core/router"
)

const backupFilePrefix = "allbot-backup-"

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
	s.createMu.Lock()
	defer s.createMu.Unlock()

	settings, err := s.database.GetBackupSettings()
	if err != nil {
		return BackupFile{}, err
	}
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
	if err := s.Cleanup(settings.Retention); err != nil {
		log.Printf("[SYSTEM] 清理旧备份失败: %v", err)
	}
	return file, nil
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
