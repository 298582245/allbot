package web

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/allbot/allbot/core/updater"
)

type updateInfoResponse struct {
	CurrentVersion   string                `json:"currentVersion"`
	DisplayVersion   string                `json:"displayVersion"`
	Commit           string                `json:"commit"`
	BuildTime        string                `json:"buildTime"`
	BuildChannel     string                `json:"buildChannel"`
	GoVersion        string                `json:"goVersion"`
	LatestVersion    string                `json:"latestVersion"`
	HasUpdate        bool                  `json:"hasUpdate"`
	ReleaseName      string                `json:"releaseName"`
	ReleaseBody      string                `json:"releaseBody"`
	ReleaseURL       string                `json:"releaseUrl"`
	Assets           []updateAssetResponse `json:"assets"`
	MatchedAsset     updateAssetResponse   `json:"matchedAsset"`
	ChecksumAsset    updateAssetResponse   `json:"checksumAsset"`
	Error            string                `json:"error"`
	Message          string                `json:"message"`
	UpgradeSupported bool                  `json:"upgradeSupported"`
	UpgradeMessage   string                `json:"upgradeMessage"`
}

type updateAssetResponse struct {
	Name        string `json:"name"`
	DownloadURL string `json:"downloadUrl"`
	Size        int64  `json:"size"`
}

func (s *Server) SetReleaseClient(client updater.ReleaseClient) {
	service := s.ensureUpdateService()
	service.SetReleaseClient(client)
	if client == nil {
		s.releaseClient = updater.NewGitHubClient()
		return
	}
	s.releaseClient = client
}

func (s *Server) SetUpgradeRunner(runner func(updater.ApplyUpdateRequest) error) {
	service := s.ensureUpdateService()
	service.SetUpgradeRunner(runner)
	if runner == nil {
		s.upgradeRunner = updater.DefaultUpgradeRunner
		return
	}
	s.upgradeRunner = runner
}

func (s *Server) SetUpgradeExitHandler(handler func()) {
	s.upgradeExit = handler
	s.ensureUpdateService().SetExitHandler(handler)
}

func (s *Server) UpdateService() *updater.Service {
	return s.ensureUpdateService()
}

func (s *Server) handleSystemUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	s.jsonResponse(w, toUpdateInfoResponse(s.ensureUpdateService().Check(ctx)))
}

func (s *Server) handleSystemUpdateStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.jsonResponse(w, s.ensureUpdateService().CurrentState())
}

func (s *Server) handleSystemUpgrade(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	state, err := s.ensureUpdateService().StartUpgrade(ctx)
	if err != nil {
		s.jsonError(w, err.Error(), statusForUpgradeState(state))
		return
	}
	s.jsonResponse(w, state)
}

func (s *Server) ensureUpdateService() *updater.Service {
	if s.updateService != nil {
		return s.updateService
	}
	s.updateService = updater.NewService(s.releaseClient, s.upgradeRunner)
	s.updateService.SetExitHandler(s.upgradeExit)
	return s.updateService
}

func statusForUpgradeState(state updater.UpgradeState) int {
	if state.Status == updater.UpgradeStatusDownloading || state.Status == updater.UpgradeStatusRestarting {
		return http.StatusConflict
	}
	return http.StatusBadRequest
}

func toUpdateInfoResponse(result updater.UpdateCheckResult) updateInfoResponse {
	response := updateInfoResponse{
		CurrentVersion:   result.CurrentVersion,
		DisplayVersion:   result.DisplayVersion,
		Commit:           result.Commit,
		BuildTime:        result.BuildTime,
		BuildChannel:     result.BuildChannel,
		GoVersion:        result.GoVersion,
		LatestVersion:    result.LatestVersion,
		HasUpdate:        result.HasUpdate,
		ReleaseName:      result.ReleaseName,
		ReleaseBody:      result.ReleaseBody,
		ReleaseURL:       result.ReleaseURL,
		Assets:           make([]updateAssetResponse, 0, len(result.Assets)),
		MatchedAsset:     toUpdateAssetResponse(result.MatchedAsset),
		ChecksumAsset:    toUpdateAssetResponse(result.ChecksumAsset),
		Error:            result.Error,
		Message:          result.Message,
		UpgradeSupported: result.UpgradeSupported,
		UpgradeMessage:   result.UpgradeMessage,
	}
	for _, asset := range result.Assets {
		response.Assets = append(response.Assets, toUpdateAssetResponse(asset))
	}
	return response
}

func toUpdateAssetResponse(asset updater.ReleaseAsset) updateAssetResponse {
	return updateAssetResponse{Name: asset.Name, DownloadURL: asset.DownloadURL, Size: asset.Size}
}

func (s *Server) failUpgrade(w http.ResponseWriter, message string) {
	s.jsonError(w, message, http.StatusBadRequest)
}

func unsupportedUpgradeMessage(version string) string {
	return fmt.Sprintf("未找到 checksums-%s.txt 校验文件", version)
}
