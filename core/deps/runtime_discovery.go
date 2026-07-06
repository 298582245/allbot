package deps

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path"
	"sort"
	"strconv"
	"strings"
)

const (
	defaultRuntimeDownloadCandidateLimit = 50
	maxRuntimeDownloadCandidateLimit     = 100
	pythonStandaloneReleasePageSize      = 30
)

var pythonStandaloneReleasesURL = "https://api.github.com/repos/astral-sh/python-build-standalone/releases"

type RuntimeDownloadCandidateQuery struct {
	Runtime      string
	Architecture string
	Q            string
	Limit        int
}

type RuntimeDownloadCandidate struct {
	Runtime      string   `json:"runtime,omitempty"`
	Architecture string   `json:"architecture,omitempty"`
	Version      string   `json:"version"`
	Label        string   `json:"label"`
	AssetName    string   `json:"asset_name,omitempty"`
	FileName     string   `json:"file_name,omitempty"`
	Source       string   `json:"source"`
	Preferred    bool     `json:"preferred,omitempty"`
	Warnings     []string `json:"warnings,omitempty"`
}

type RuntimeDownloadCandidateResult struct {
	Runtime      string                     `json:"runtime"`
	Architecture string                     `json:"architecture"`
	Source       string                     `json:"source"`
	Candidates   []RuntimeDownloadCandidate `json:"candidates"`
	Warnings     []string                   `json:"warnings"`
}

type RuntimeDownloadDiscoverer interface {
	ListRuntimeDownloadCandidates(query RuntimeDownloadCandidateQuery, options RuntimeDownloadOptions) (RuntimeDownloadCandidateResult, error)
}

func (m *Manager) ListRuntimeDownloadCandidates(query RuntimeDownloadCandidateQuery, options RuntimeDownloadOptions) (RuntimeDownloadCandidateResult, error) {
	query.Runtime = normalizeRuntimeName(query.Runtime)
	query.Architecture = strings.ToLower(strings.TrimSpace(query.Architecture))
	query.Q = strings.TrimSpace(strings.TrimPrefix(query.Q, "v"))
	query.Limit = normalizeRuntimeCandidateLimit(query.Limit)
	if query.Runtime != "nodejs" && query.Runtime != "python" {
		return RuntimeDownloadCandidateResult{}, fmt.Errorf("运行环境只支持 nodejs/python: %s", query.Runtime)
	}
	if query.Architecture == "" {
		query.Architecture = defaultRuntimeArchitecture()
	}
	if !isSupportedRuntimeArchitecture(query.Architecture) {
		return RuntimeDownloadCandidateResult{}, fmt.Errorf("运行环境架构不支持: %s", query.Architecture)
	}
	m.mu.RLock()
	downloader := m.downloader
	m.mu.RUnlock()
	discoverer, ok := downloader.(RuntimeDownloadDiscoverer)
	if !ok || discoverer == nil {
		return RuntimeDownloadCandidateResult{}, fmt.Errorf("运行环境下载器不支持候选发现")
	}
	return discoverer.ListRuntimeDownloadCandidates(query, options)
}

func (d *HTTPRuntimeDownloader) ListRuntimeDownloadCandidates(query RuntimeDownloadCandidateQuery, options RuntimeDownloadOptions) (RuntimeDownloadCandidateResult, error) {
	query.Runtime = normalizeRuntimeName(query.Runtime)
	query.Architecture = strings.ToLower(strings.TrimSpace(query.Architecture))
	query.Q = strings.TrimSpace(strings.TrimPrefix(query.Q, "v"))
	query.Limit = normalizeRuntimeCandidateLimit(query.Limit)
	if query.Architecture == "" {
		query.Architecture = defaultRuntimeArchitecture()
	}
	client, err := runtimeHTTPClient(options)
	if err != nil {
		return RuntimeDownloadCandidateResult{}, err
	}
	if query.Runtime == "nodejs" {
		return d.listNodeDownloadCandidates(client, query, options)
	}
	if query.Runtime == "python" {
		if isWindowsRuntimeArchitecture(query.Architecture) {
			return d.listWindowsPythonDownloadCandidates(client, query, options)
		}
		return d.listLinuxPythonDownloadCandidates(client, query)
	}
	return RuntimeDownloadCandidateResult{}, fmt.Errorf("运行环境只支持 nodejs/python: %s", query.Runtime)
}

func normalizeRuntimeCandidateLimit(limit int) int {
	if limit <= 0 {
		return defaultRuntimeDownloadCandidateLimit
	}
	if limit > maxRuntimeDownloadCandidateLimit {
		return maxRuntimeDownloadCandidateLimit
	}
	return limit
}

type nodeDistIndexItem struct {
	Version string   `json:"version"`
	Files   []string `json:"files"`
	LTS     any      `json:"lts"`
}

func (d *HTTPRuntimeDownloader) listNodeDownloadCandidates(client *http.Client, query RuntimeDownloadCandidateQuery, options RuntimeDownloadOptions) (RuntimeDownloadCandidateResult, error) {
	if !isSupportedRuntimeArchitecture(query.Architecture) {
		return RuntimeDownloadCandidateResult{}, fmt.Errorf("Node.js 架构不支持: %s", query.Architecture)
	}
	fileKey, err := nodeDistFileKey(query.Architecture)
	if err != nil {
		return RuntimeDownloadCandidateResult{}, err
	}
	mirrorURL := runtimeOptionOrDefault(options.NodeMirrorURL, "https://nodejs.org/dist")
	trustedHosts := appendTrustedHost([]string{"nodejs.org"}, mirrorURL)
	data, err := d.fetchTrustedText(client, mirrorURL+"/index.json", trustedHosts, "读取 Node.js 版本索引失败")
	if err != nil {
		return RuntimeDownloadCandidateResult{}, err
	}
	var index []nodeDistIndexItem
	if err := json.Unmarshal(data, &index); err != nil {
		return RuntimeDownloadCandidateResult{}, err
	}
	candidates := make([]RuntimeDownloadCandidate, 0, len(index))
	for _, item := range index {
		version := strings.TrimPrefix(strings.TrimSpace(item.Version), "v")
		if version == "" || !versionMatchesQuery(version, query.Q) || !hasString(item.Files, fileKey) {
			continue
		}
		archiveName := nodeArchiveName(version, query.Architecture)
		candidate := RuntimeDownloadCandidate{Runtime: "nodejs", Architecture: query.Architecture, Version: version, Label: "Node.js " + version, AssetName: archiveName, FileName: archiveName, Source: "nodejs-dist", Preferred: true}
		if item.LTS != nil && item.LTS != false {
			candidate.Label += " LTS"
		}
		candidates = append(candidates, candidate)
		if len(candidates) >= query.Limit {
			break
		}
	}
	return RuntimeDownloadCandidateResult{Runtime: "nodejs", Architecture: query.Architecture, Source: "nodejs-dist", Candidates: candidates, Warnings: []string{}}, nil
}

func hasString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func nodeDistFileKey(architecture string) (string, error) {
	switch architecture {
	case "linux-x64", "linux-arm64":
		return architecture, nil
	case "win-x64", "win-arm64":
		return architecture + "-zip", nil
	default:
		return "", fmt.Errorf("Node.js 架构不支持: %s", architecture)
	}
}

func nodeArchiveName(version, architecture string) string {
	archiveExt := ".zip"
	if !isWindowsRuntimeArchitecture(architecture) {
		archiveExt = ".tar.gz"
	}
	return fmt.Sprintf("node-v%s-%s%s", version, architecture, archiveExt)
}

func (d *HTTPRuntimeDownloader) listWindowsPythonDownloadCandidates(client *http.Client, query RuntimeDownloadCandidateQuery, options RuntimeDownloadOptions) (RuntimeDownloadCandidateResult, error) {
	if query.Architecture != "win-x64" {
		return RuntimeDownloadCandidateResult{}, fmt.Errorf("Python Windows 嵌入包自动下载暂只支持 win-x64")
	}
	metadataURL := runtimeOptionOrDefault(options.PythonMetadataURL, "https://api.nuget.org/v3/registration5-gz-semver2/python/index.json")
	trustedHosts := appendTrustedHost([]string{"api.nuget.org", "nuget.azure.cn"}, metadataURL)
	versions, err := d.listNuGetPythonVersions(client, metadataURL, trustedHosts)
	if err != nil {
		return RuntimeDownloadCandidateResult{}, err
	}
	sort.SliceStable(versions, func(i, j int) bool { return compareVersionDesc(versions[i], versions[j]) })
	candidates := make([]RuntimeDownloadCandidate, 0, len(versions))
	for _, version := range versions {
		if !versionMatchesQuery(version, query.Q) {
			continue
		}
		archiveName := fmt.Sprintf("python.%s.nupkg", version)
		candidates = append(candidates, RuntimeDownloadCandidate{Runtime: "python", Architecture: query.Architecture, Version: version, Label: "Python " + version, AssetName: archiveName, FileName: archiveName, Source: "nuget", Preferred: true})
		if len(candidates) >= query.Limit {
			break
		}
	}
	return RuntimeDownloadCandidateResult{Runtime: "python", Architecture: query.Architecture, Source: "nuget", Candidates: candidates, Warnings: []string{}}, nil
}

func (d *HTTPRuntimeDownloader) listNuGetPythonVersions(client *http.Client, metadataURL string, trustedHosts []string) ([]string, error) {
	data, err := d.fetchTrustedText(client, metadataURL, trustedHosts, "读取 Python NuGet 元数据失败")
	if err != nil {
		return nil, err
	}
	versions, err := d.extractNuGetVersionsFromRegistration(client, data, trustedHosts)
	if err != nil {
		return nil, err
	}
	return versions, nil
}

func (d *HTTPRuntimeDownloader) extractNuGetVersionsFromRegistration(client *http.Client, data []byte, trustedHosts []string) ([]string, error) {
	var root struct {
		Items []struct {
			Items []nugetRegistrationItem `json:"items"`
			URL   string                  `json:"@id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	versions := []string{}
	appendItems := func(items []nugetRegistrationItem) {
		for _, item := range items {
			version := strings.TrimSpace(item.CatalogEntry.Version)
			if version == "" || seen[strings.ToLower(version)] {
				continue
			}
			seen[strings.ToLower(version)] = true
			versions = append(versions, version)
		}
	}
	for _, page := range root.Items {
		appendItems(page.Items)
		if len(page.Items) > 0 || page.URL == "" {
			continue
		}
		pageData, err := d.fetchTrustedText(client, page.URL, trustedHosts, "读取 NuGet 版本页失败")
		if err != nil {
			return nil, err
		}
		var expanded struct {
			Items []nugetRegistrationItem `json:"items"`
		}
		if err := json.Unmarshal(pageData, &expanded); err != nil {
			return nil, err
		}
		appendItems(expanded.Items)
	}
	return versions, nil
}

func (d *HTTPRuntimeDownloader) listLinuxPythonDownloadCandidates(client *http.Client, query RuntimeDownloadCandidateQuery) (RuntimeDownloadCandidateResult, error) {
	assets, err := listPythonStandaloneCandidateAssets(client, query.Architecture, pythonStandaloneReleasePageSize)
	if err != nil {
		return RuntimeDownloadCandidateResult{}, err
	}
	sort.SliceStable(assets, func(i, j int) bool { return compareVersionDesc(assets[i].Version, assets[j].Version) })
	warnings := []string{fmt.Sprintf("仅展示最近 %d 条 python-build-standalone release 中可发现的 install_only 资产", pythonStandaloneReleasePageSize)}
	candidates := make([]RuntimeDownloadCandidate, 0, len(assets))
	for _, item := range assets {
		if !versionMatchesQuery(item.Version, query.Q) {
			continue
		}
		fileName := item.Asset.Name
		if fileName == "" {
			fileName = path.Base(item.Asset.URL)
		}
		candidates = append(candidates, RuntimeDownloadCandidate{Runtime: "python", Architecture: query.Architecture, Version: item.Version, Label: "Python " + item.Version, AssetName: item.Asset.Name, FileName: fileName, Source: "python-build-standalone", Preferred: item.Preferred, Warnings: warnings})
		if len(candidates) >= query.Limit {
			break
		}
	}
	return RuntimeDownloadCandidateResult{Runtime: "python", Architecture: query.Architecture, Source: "python-build-standalone", Candidates: candidates, Warnings: warnings}, nil
}

type pythonStandaloneCandidateAsset struct {
	Version   string
	Asset     pythonStandaloneAsset
	Preferred bool
}

func listPythonStandaloneCandidateAssets(client *http.Client, architecture string, perPage int) ([]pythonStandaloneCandidateAsset, error) {
	platform, err := pythonStandalonePlatform(architecture)
	if err != nil {
		return nil, err
	}
	requestURL := fmt.Sprintf("%s?per_page=%d", strings.TrimRight(pythonStandaloneReleasesURL, "/"), perPage)
	request, err := http.NewRequest(http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("读取 Python standalone 发布列表失败: HTTP %d", response.StatusCode)
	}
	var releases []pythonStandaloneRelease
	if err := json.NewDecoder(response.Body).Decode(&releases); err != nil {
		return nil, err
	}
	byVersion := map[string]pythonStandaloneCandidateAsset{}
	for _, release := range releases {
		for _, asset := range release.Assets {
			version, ok := pythonStandaloneAssetVersion(asset.Name, platform)
			if !ok {
				continue
			}
			candidate := pythonStandaloneCandidateAsset{Version: version, Asset: asset, Preferred: isPreferredPythonStandaloneAsset(asset.Name)}
			current, exists := byVersion[version]
			if !exists || (!current.Preferred && candidate.Preferred) {
				byVersion[version] = candidate
			}
		}
	}
	items := make([]pythonStandaloneCandidateAsset, 0, len(byVersion))
	for _, item := range byVersion {
		items = append(items, item)
	}
	return items, nil
}

func pythonStandaloneAssetVersion(name, platform string) (string, bool) {
	if !strings.HasPrefix(name, "cpython-") || !strings.Contains(name, platform) || !strings.Contains(name, "install_only") {
		return "", false
	}
	if !isPythonStandaloneArchive(name) {
		return "", false
	}
	withoutPrefix := strings.TrimPrefix(name, "cpython-")
	plusIndex := strings.Index(withoutPrefix, "+")
	if plusIndex <= 0 {
		return "", false
	}
	return withoutPrefix[:plusIndex], true
}

func isPythonStandaloneArchive(name string) bool {
	return strings.HasSuffix(name, ".tar.gz") || strings.HasSuffix(name, ".tgz") || strings.HasSuffix(name, ".tar.zst") || strings.HasSuffix(name, ".tar.zstd")
}

func isPreferredPythonStandaloneAsset(name string) bool {
	return strings.HasSuffix(name, ".tar.gz") || strings.HasSuffix(name, ".tgz")
}

func versionMatchesQuery(version, q string) bool {
	q = strings.TrimSpace(strings.TrimPrefix(q, "v"))
	if q == "" {
		return true
	}
	return strings.HasPrefix(strings.ToLower(version), strings.ToLower(q))
}

func compareVersionDesc(left, right string) bool {
	lp := splitVersionParts(left)
	rp := splitVersionParts(right)
	maxLen := len(lp)
	if len(rp) > maxLen {
		maxLen = len(rp)
	}
	for i := 0; i < maxLen; i++ {
		lv, rv := 0, 0
		if i < len(lp) {
			lv = lp[i]
		}
		if i < len(rp) {
			rv = rp[i]
		}
		if lv != rv {
			return lv > rv
		}
	}
	return left > right
}

func splitVersionParts(version string) []int {
	version = strings.TrimPrefix(version, "v")
	parts := strings.FieldsFunc(version, func(r rune) bool { return r == '.' || r == '-' || r == '_' || r == '+' })
	values := make([]int, 0, len(parts))
	for _, part := range parts {
		value, err := strconv.Atoi(part)
		if err != nil {
			break
		}
		values = append(values, value)
	}
	return values
}
