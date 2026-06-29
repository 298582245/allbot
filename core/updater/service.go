package updater

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/allbot/allbot/core/version"
)

type UpdateCheckResult struct {
	CurrentVersion   string
	DisplayVersion   string
	Commit           string
	BuildTime        string
	BuildChannel     string
	GoVersion        string
	LatestVersion    string
	HasUpdate        bool
	ReleaseName      string
	ReleaseBody      string
	ReleaseURL       string
	Assets           []ReleaseAsset
	MatchedAsset     ReleaseAsset
	ChecksumAsset    ReleaseAsset
	Error            string
	Message          string
	UpgradeSupported bool
	UpgradeMessage   string
}

type Service struct {
	client      ReleaseClient
	runner      func(ApplyUpdateRequest) error
	exitHandler func()
	mu          sync.Mutex
	state       UpgradeState
}

func NewService(client ReleaseClient, runner func(ApplyUpdateRequest) error) *Service {
	if client == nil {
		client = NewGitHubClient()
	}
	if runner == nil {
		runner = DefaultUpgradeRunner
	}
	return &Service{client: client, runner: runner, state: UpgradeState{Status: UpgradeStatusIdle, Message: "暂无升级任务"}}
}

func (s *Service) SetReleaseClient(client ReleaseClient) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if client == nil {
		client = NewGitHubClient()
	}
	s.client = client
}

func (s *Service) SetUpgradeRunner(runner func(ApplyUpdateRequest) error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if runner == nil {
		runner = DefaultUpgradeRunner
	}
	s.runner = runner
}

func (s *Service) SetExitHandler(handler func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.exitHandler = handler
}

func (s *Service) Check(ctx context.Context) UpdateCheckResult {
	response := currentCheckResult()
	release, err := s.latestRelease(ctx)
	if err != nil {
		response.Error = err.Error()
		response.Message = "检查更新失败: " + err.Error()
		return response
	}
	if release == nil {
		response.Error = "未检测到 GitHub Release"
		response.Message = "未检测到 GitHub Release。"
		return response
	}
	fillCheckReleaseInfo(&response, release)
	compare, err := CompareVersion(response.CurrentVersion, response.LatestVersion)
	if err != nil {
		response.Error = err.Error()
		response.Message = "版本比较失败: " + err.Error()
		return response
	}
	if compare < 0 {
		response.HasUpdate = true
		asset, found := SelectAssetForCurrentPlatform(release.Assets)
		checksumAsset, checksumFound := SelectChecksumAsset(release.Assets, response.LatestVersion)
		if found {
			response.MatchedAsset = asset
		}
		if checksumFound {
			response.ChecksumAsset = checksumAsset
		}
		if found && checksumFound {
			response.UpgradeSupported = true
			response.UpgradeMessage = fmt.Sprintf("可一键升级到 %s，匹配资产：%s，校验文件：%s。", response.LatestVersion, asset.Name, checksumAsset.Name)
			response.Message = "发现新版本，可一键升级。"
		} else if !found {
			response.UpgradeMessage = fmt.Sprintf("发现新版本，但未找到当前平台 %s/%s 的发布资产，请手动下载。", runtime.GOOS, runtime.GOARCH)
			response.Message = "发现新版本，请前往 Release 手动更新。"
		} else {
			response.UpgradeMessage = fmt.Sprintf("发现新版本，但未找到 checksums-%s.txt 校验文件，请补充后再使用一键升级。", response.LatestVersion)
			response.Message = "发现新版本，请前往 Release 手动更新。"
		}
	} else {
		response.Message = "当前已是最新版本。"
	}
	return response
}

func (s *Service) StartUpgrade(ctx context.Context) (UpgradeState, error) {
	if !s.beginUpgrade() {
		state := s.CurrentState()
		return state, fmt.Errorf("已有升级任务正在执行")
	}
	check := s.Check(ctx)
	if check.Error != "" {
		state := UpgradeState{Status: UpgradeStatusFailed, Message: check.Message, Error: check.Error}
		s.setState(state)
		return state, errors.New(check.Message)
	}
	if !check.HasUpdate {
		state := UpgradeState{Status: UpgradeStatusIdle, Message: "当前已是最新版本。"}
		s.setState(state)
		return state, errors.New(state.Message)
	}
	if strings.TrimSpace(check.MatchedAsset.Name) == "" {
		message := fmt.Sprintf("未找到当前平台 %s/%s 的发布资产", runtime.GOOS, runtime.GOARCH)
		state := UpgradeState{Status: UpgradeStatusFailed, Message: message, Error: message}
		s.setState(state)
		return state, errors.New(message)
	}
	if strings.TrimSpace(check.ChecksumAsset.Name) == "" {
		message := fmt.Sprintf("未找到 checksums-%s.txt 校验文件", check.LatestVersion)
		state := UpgradeState{Status: UpgradeStatusFailed, Message: message, Error: message}
		s.setState(state)
		return state, errors.New(message)
	}
	state := UpgradeState{Status: UpgradeStatusDownloading, Message: "正在下载升级包", Version: check.LatestVersion, AssetName: check.MatchedAsset.Name}
	s.setState(state)
	go s.runDownload(ctx, check.CurrentVersion, check.LatestVersion, check.MatchedAsset, check.ChecksumAsset)
	return state, nil
}

func (s *Service) CurrentState() UpgradeState {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state.Status == "" {
		return UpgradeState{Status: UpgradeStatusIdle, Message: "暂无升级任务"}
	}
	return s.state
}

func (s *Service) latestRelease(ctx context.Context) (*ReleaseInfo, error) {
	s.mu.Lock()
	client := s.client
	s.mu.Unlock()
	if client == nil {
		client = NewGitHubClient()
	}
	return client.LatestRelease(ctx)
}

func (s *Service) beginUpgrade() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state.Status == UpgradeStatusDownloading || s.state.Status == UpgradeStatusRestarting {
		return false
	}
	s.state = UpgradeState{Status: UpgradeStatusDownloading, Message: "准备升级"}
	return true
}

func (s *Service) setState(state UpgradeState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state = state
}

func (s *Service) runDownload(parent context.Context, currentVersion string, latestVersion string, asset ReleaseAsset, checksumAsset ReleaseAsset) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(parent), 15*time.Minute)
	defer cancel()
	currentPath, err := os.Executable()
	if err != nil {
		s.setState(UpgradeState{Status: UpgradeStatusFailed, Message: "获取当前程序路径失败", Error: err.Error(), Version: latestVersion, AssetName: asset.Name})
		return
	}
	workDir, err := os.Getwd()
	if err != nil {
		s.setState(UpgradeState{Status: UpgradeStatusFailed, Message: "获取工作目录失败", Error: err.Error(), Version: latestVersion, AssetName: asset.Name})
		return
	}
	updateDir := filepath.Join("runtime", "update")
	newPath := filepath.Join(updateDir, downloadedBinaryName())
	if err := (Downloader{}).Download(ctx, asset, newPath); err != nil {
		s.setState(UpgradeState{Status: UpgradeStatusFailed, Message: "下载升级包失败", Error: err.Error(), Version: latestVersion, AssetName: asset.Name})
		return
	}
	checksumFile, err := DownloadChecksumFile(ctx, checksumAsset)
	if err != nil {
		s.setState(UpgradeState{Status: UpgradeStatusFailed, Message: "下载校验文件失败", Error: err.Error(), Version: latestVersion, AssetName: asset.Name})
		return
	}
	expectedSHA256, ok := checksumFile.ExpectedSHA256(asset.Name)
	if !ok {
		s.setState(UpgradeState{Status: UpgradeStatusFailed, Message: "校验文件缺少当前平台资产", Error: "校验文件缺少 " + asset.Name, Version: latestVersion, AssetName: asset.Name})
		return
	}
	if err := VerifyFileSHA256(newPath, expectedSHA256); err != nil {
		s.setState(UpgradeState{Status: UpgradeStatusFailed, Message: "升级包 SHA256 校验失败", Error: err.Error(), Version: latestVersion, AssetName: asset.Name})
		return
	}
	downLoadedAt := time.Now().Format(time.RFC3339)
	s.setState(UpgradeState{Status: UpgradeStatusRestarting, Message: "下载完成，正在重启并应用更新", Version: latestVersion, AssetName: asset.Name, DownloadedAt: downLoadedAt})
	s.mu.Lock()
	runner := s.runner
	exitHandler := s.exitHandler
	s.mu.Unlock()
	if runner == nil {
		runner = DefaultUpgradeRunner
	}
	request := ApplyUpdateRequest{ParentPID: os.Getpid(), CurrentPath: currentPath, NewPath: newPath, BackupPath: filepath.Join(updateDir, "backup", filepath.Base(currentPath)+".bak"), WorkDir: workDir, Args: os.Args[1:], FromVersion: currentVersion, ToVersion: latestVersion, RestartDelay: "2000", RestartedFlag: "1"}
	if err := runner(request); err != nil {
		s.setState(UpgradeState{Status: UpgradeStatusFailed, Message: "启动更新器失败", Error: err.Error(), Version: latestVersion, AssetName: asset.Name, DownloadedAt: downLoadedAt})
		return
	}
	if exitHandler != nil {
		go func() {
			time.Sleep(500 * time.Millisecond)
			exitHandler()
		}()
	}
}

func currentCheckResult() UpdateCheckResult {
	return UpdateCheckResult{CurrentVersion: normalizeVersion(version.Version), DisplayVersion: version.DisplayVersion(), Commit: fallbackUnknown(version.Commit), BuildTime: fallbackUnknown(version.BuildTime), BuildChannel: fallbackUnknown(version.NormalizedBuildChannel()), GoVersion: runtime.Version(), Assets: []ReleaseAsset{}, Message: "点击检查更新获取最新 Release 信息。", UpgradeSupported: false, UpgradeMessage: "检查到可用版本且存在当前平台发布资产后，可一键升级。"}
}

func fillCheckReleaseInfo(response *UpdateCheckResult, release *ReleaseInfo) {
	response.LatestVersion = strings.TrimSpace(release.Version)
	response.ReleaseName = strings.TrimSpace(release.Name)
	response.ReleaseBody = release.Body
	response.ReleaseURL = strings.TrimSpace(release.URL)
	response.Assets = append([]ReleaseAsset(nil), release.Assets...)
}

func downloadedBinaryName() string {
	if runtime.GOOS == "windows" {
		return "allbot-new.exe"
	}
	return "allbot-new"
}

func normalizeVersion(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown"
	}
	return value
}

func fallbackUnknown(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown"
	}
	return value
}
