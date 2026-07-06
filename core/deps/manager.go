package deps

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

// Manager 全局依赖管理器。
type Manager struct {
	pythonVenv          string // Python 虚拟环境路径
	nodeModules         string // Node.js 全局 node_modules 路径
	pythonDepsFile      string // Python 依赖清单文件
	nodeDepsFile        string // Node.js 依赖清单文件
	runtimeProfilesFile string // 运行环境 Profile 配置文件
	downloader          RuntimeDownloader
	mu                  sync.RWMutex
}

type RuntimeProfile struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Runtime          string `json:"runtime"`
	Version          string `json:"version,omitempty"`
	Executable       string `json:"executable"`
	Enabled          bool   `json:"enabled"`
	Default          bool   `json:"default"`
	Description      string `json:"description,omitempty"`
	Source           string `json:"source,omitempty"`
	RequestedVersion string `json:"requested_version,omitempty"`
	Architecture     string `json:"architecture,omitempty"`
}

type RuntimeProfileConfig struct {
	Profiles []RuntimeProfile `json:"profiles"`
}

type RuntimeDownloadOptions struct {
	ProxyURL               string
	NodeMirrorURL          string
	PythonPackageMirrorURL string
	PythonMetadataURL      string
}

type RuntimeProfileInitOptions struct {
	ProfileID       string
	Force           bool
	AutoDownload    bool
	DownloadOptions RuntimeDownloadOptions
	Progress        RuntimeProfileInitProgressFunc `json:"-"`
}

type RuntimeProfileInitProgressFunc func(progress RuntimeProfileInitProgress)

type RuntimeProfileInitProgress struct {
	Stage           string `json:"stage"`
	Message         string `json:"message"`
	Progress        int    `json:"progress"`
	DownloadedBytes int64  `json:"downloaded_bytes,omitempty"`
	TotalBytes      int64  `json:"total_bytes,omitempty"`
}

type RuntimeProfileInitResult struct {
	ProfileID     string `json:"profile_id"`
	Runtime       string `json:"runtime"`
	Source        string `json:"source"`
	Status        string `json:"status"`
	Message       string `json:"message"`
	Executable    string `json:"executable"`
	EnvRoot       string `json:"env_root"`
	NodePath      string `json:"node_path,omitempty"`
	VersionOutput string `json:"version_output"`
}

type RuntimeProfileStatus struct {
	ProfileID     string `json:"profile_id"`
	Runtime       string `json:"runtime"`
	Source        string `json:"source"`
	Initialized   bool   `json:"initialized"`
	Executable    string `json:"executable"`
	EnvRoot       string `json:"env_root"`
	DepsFile      string `json:"deps_file"`
	NodePath      string `json:"node_path,omitempty"`
	VersionOutput string `json:"version_output,omitempty"`
	Error         string `json:"error,omitempty"`
}

type ResolvedRuntime struct {
	Profile    RuntimeProfile
	Executable string
	NodePath   string
}

type profileDependencyPaths struct {
	pythonVenv     string
	pythonDepsFile string
	nodeRuntimeDir string
	nodeModules    string
	nodeDepsFile   string
}

// PythonDeps Python 依赖清单。
type PythonDeps struct {
	Packages map[string]string `json:"packages"`
}

// NodeDeps Node.js 依赖清单。
type NodeDeps struct {
	Dependencies map[string]string `json:"dependencies"`
}

// NewManager 创建依赖管理器。
func NewManager(runtimeDir string) *Manager {
	manager := &Manager{
		pythonVenv:          filepath.Join(runtimeDir, ".venv"),
		nodeModules:         filepath.Join(runtimeDir, "node_modules"),
		pythonDepsFile:      filepath.Join(runtimeDir, "python_deps.json"),
		nodeDepsFile:        filepath.Join(runtimeDir, "package.json"),
		runtimeProfilesFile: filepath.Join(runtimeDir, "runtime_profiles.json"),
	}
	manager.downloader = NewHTTPRuntimeDownloader(filepath.Join(runtimeDir, "interpreters"))
	return manager
}

// InitPythonEnv 初始化 Python 虚拟环境。
func (m *Manager) InitPythonEnv() error {
	if _, err := os.Stat(m.pythonVenv); os.IsNotExist(err) {
		fmt.Println("正在创建 Python 虚拟环境...")
		cmd := exec.Command("python", "-m", "venv", m.pythonVenv)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("创建虚拟环境失败: %w", err)
		}
		fmt.Println("Python 虚拟环境创建成功")
	}

	if _, err := os.Stat(m.pythonDepsFile); os.IsNotExist(err) {
		deps := PythonDeps{Packages: make(map[string]string)}
		data, _ := json.MarshalIndent(deps, "", "  ")
		if err := os.WriteFile(m.pythonDepsFile, data, 0644); err != nil {
			return err
		}
	}

	return nil
}

// InitNodeEnv 初始化 Node.js 环境。
func (m *Manager) InitNodeEnv() error {
	runtimeDir := filepath.Dir(m.nodeModules)
	if err := os.MkdirAll(runtimeDir, 0755); err != nil {
		return err
	}

	if _, err := os.Stat(m.nodeDepsFile); os.IsNotExist(err) {
		fmt.Println("正在初始化 Node.js 环境...")
		deps := NodeDeps{Dependencies: make(map[string]string)}
		data, _ := json.MarshalIndent(deps, "", "  ")
		if err := os.WriteFile(m.nodeDepsFile, data, 0644); err != nil {
			return err
		}
		fmt.Println("Node.js 环境初始化成功")
	}

	return nil
}

// InstallPythonDeps 安装默认 Python 依赖，版本为空或 latest 时安装最新版并记录实际版本号。
func (m *Manager) InstallPythonDeps(deps map[string]string) error {
	return m.InstallPythonDepsForProfile("", deps)
}

func (m *Manager) InstallPythonDepsForProfile(profileID string, deps map[string]string) error {
	return m.installPythonDepsForProfile(profileID, deps, dependencyInstallOptions{forceLatest: true})
}

// EnsurePythonDeps 幂等确保默认 Python 依赖存在，已满足时不主动升级。
func (m *Manager) EnsurePythonDeps(deps map[string]string) error {
	return m.EnsurePythonDepsForProfile("", deps)
}

func (m *Manager) EnsurePythonDepsForProfile(profileID string, deps map[string]string) error {
	return m.installPythonDepsForProfile(profileID, deps, dependencyInstallOptions{quietSkip: true})
}

type dependencyInstallOptions struct {
	forceLatest bool
	quietSkip   bool
}

var (
	pythonPackageNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*(\[[A-Za-z0-9][A-Za-z0-9_.-]*(,[A-Za-z0-9][A-Za-z0-9_.-]*)*\])?$`)
	nodePackageNamePattern   = regexp.MustCompile(`^(?:@[A-Za-z0-9][A-Za-z0-9_.-]*/)?[A-Za-z0-9][A-Za-z0-9_.-]*$`)
	dependencyVersionPattern = regexp.MustCompile(`^[A-Za-z0-9^~=][A-Za-z0-9_.+~^=-]*$`)
)

func validatePythonDependencyName(name string) error {
	name = strings.TrimSpace(name)
	if !pythonPackageNamePattern.MatchString(name) {
		return fmt.Errorf("Python 依赖包名无效")
	}
	return nil
}

func pythonPackageLookupName(name string) string {
	name = strings.TrimSpace(name)
	if index := strings.Index(name, "["); index >= 0 {
		return strings.TrimSpace(name[:index])
	}
	return name
}

func validateNodeDependencyName(name string) error {
	name = strings.TrimSpace(name)
	if !nodePackageNamePattern.MatchString(name) {
		return fmt.Errorf("Node.js 依赖包名无效")
	}
	return nil
}

func validateDependencyVersion(version string) error {
	version = strings.TrimSpace(version)
	if version == "" || version == "latest" {
		return nil
	}
	if !dependencyVersionPattern.MatchString(version) {
		return fmt.Errorf("依赖版本号无效")
	}
	return nil
}

func validateDependencyRequest(deps map[string]string, validateName func(string) error) error {
	for name, version := range deps {
		if err := validateName(name); err != nil {
			return err
		}
		if err := validateDependencyVersion(version); err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) installPythonDepsForProfile(profileID string, deps map[string]string, options dependencyInstallOptions) error {
	if err := validateDependencyRequest(deps, validatePythonDependencyName); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	profile, err := m.resolveProfileLocked("python", profileID)
	if err != nil {
		return err
	}
	paths := m.profileDependencyPaths(profile)
	pythonEnvExisted := dirExists(paths.pythonVenv)
	if err := m.ensurePythonProfileEnv(profile, paths.pythonVenv); err != nil {
		return err
	}
	installed, err := m.loadPythonDepsFile(paths.pythonDepsFile)
	if err != nil {
		return err
	}

	changed := false
	pipPath := profilePipPath(paths.pythonVenv)
	pythonPath := profilePythonPath(paths.pythonVenv)
	for pkg, version := range deps {
		pkg = strings.TrimSpace(pkg)
		if err := validatePythonDependencyName(pkg); err != nil {
			return err
		}
		requestedVersion := strings.TrimSpace(version)
		if err := validateDependencyVersion(requestedVersion); err != nil {
			return err
		}
		latestRequested := requestedVersion == "" || requestedVersion == "latest"
		packageSpec := pkg
		forceUpgrade := false

		if latestRequested {
			if options.forceLatest {
				forceUpgrade = true
				fmt.Printf("正在安装或更新 Python 包: %s（%s，最新版）\n", pkg, profile.ID)
			} else {
				if pythonEnvExisted && isConcreteVersion(installed.Packages[pkg]) {
					continue
				}
				if actualVersion, err := m.getPythonPackageVersionWithPython(pythonPath, pkg); err == nil && actualVersion != "" {
					if installed.Packages[pkg] != actualVersion {
						installed.Packages[pkg] = actualVersion
						changed = true
					}
					continue
				}
				fmt.Printf("正在安装 Python 包: %s（%s，最新版）\n", pkg, profile.ID)
			}
		} else {
			if installedVersion, exists := installed.Packages[pkg]; exists && isVersionSatisfied(installedVersion, requestedVersion) && (options.forceLatest || pythonEnvExisted) {
				if !options.quietSkip {
					fmt.Printf("Python 包 %s==%s 已安装（%s）\n", pkg, requestedVersion, profile.ID)
				}
				continue
			}
			if !options.forceLatest {
				if actualVersion, err := m.getPythonPackageVersionWithPython(pythonPath, pkg); err == nil && isVersionSatisfied(actualVersion, requestedVersion) {
					if installed.Packages[pkg] != actualVersion {
						installed.Packages[pkg] = actualVersion
						changed = true
					}
					continue
				}
			}
			fmt.Printf("正在安装 Python 包: %s==%s（%s）\n", pkg, requestedVersion, profile.ID)
			packageSpec = fmt.Sprintf("%s==%s", pkg, requestedVersion)
		}

		args := []string{"install"}
		if forceUpgrade {
			args = append(args, "--upgrade")
		}
		args = append(args, packageSpec)
		cmd := exec.Command(pipPath, args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("安装 %s 失败: %w", pkg, err)
		}

		actualVersion, err := m.getPythonPackageVersionWithPython(pythonPath, pkg)
		if err != nil {
			return err
		}
		if installed.Packages[pkg] != actualVersion {
			installed.Packages[pkg] = actualVersion
			changed = true
		}
	}

	if changed {
		return m.savePythonDepsFile(paths.pythonDepsFile, installed)
	}
	return nil
}

// InstallNodeDeps 安装默认 Node.js 依赖，版本为空或 latest 时安装最新版并记录实际版本号。
func (m *Manager) InstallNodeDeps(deps map[string]string) error {
	return m.InstallNodeDepsForProfile("", deps)
}

func (m *Manager) InstallNodeDepsForProfile(profileID string, deps map[string]string) error {
	return m.installNodeDepsForProfile(profileID, deps, dependencyInstallOptions{forceLatest: true})
}

// EnsureNodeDeps 幂等确保默认 Node.js 依赖存在，已满足时不主动升级。
func (m *Manager) EnsureNodeDeps(deps map[string]string) error {
	return m.EnsureNodeDepsForProfile("", deps)
}

func (m *Manager) EnsureNodeDepsForProfile(profileID string, deps map[string]string) error {
	return m.installNodeDepsForProfile(profileID, deps, dependencyInstallOptions{quietSkip: true})
}

func (m *Manager) installNodeDepsForProfile(profileID string, deps map[string]string, options dependencyInstallOptions) error {
	if err := validateDependencyRequest(deps, validateNodeDependencyName); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	profile, err := m.resolveProfileLocked("nodejs", profileID)
	if err != nil {
		return err
	}
	paths := m.profileDependencyPaths(profile)
	nodeDeps, err := m.loadNodeDepsFile(paths.nodeDepsFile)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(paths.nodeRuntimeDir, 0755); err != nil {
		return err
	}
	manifestExists := fileExists(paths.nodeDepsFile)
	changed := false
	npmExecutable := ""
	resolveNpm := func() (string, error) {
		if npmExecutable != "" {
			return npmExecutable, nil
		}
		resolved, err := m.npmExecutableForProfile(profile)
		if err != nil {
			return "", err
		}
		npmExecutable = resolved
		return npmExecutable, nil
	}

	for pkg, version := range deps {
		pkg = strings.TrimSpace(pkg)
		if err := validateNodeDependencyName(pkg); err != nil {
			return err
		}
		requestedVersion := strings.TrimSpace(version)
		if err := validateDependencyVersion(requestedVersion); err != nil {
			return err
		}
		latestRequested := requestedVersion == "" || requestedVersion == "latest"
		packageSpec := pkg
		actualVersion, actualErr := getNodePackageVersionAt(paths.nodeModules, pkg)
		if actualErr == nil && actualVersion != "" {
			if latestRequested || isNodeVersionSatisfied(actualVersion, requestedVersion) {
				if nodeDeps.Dependencies[pkg] != actualVersion {
					nodeDeps.Dependencies[pkg] = actualVersion
					changed = true
				}
				if !options.forceLatest {
					continue
				}
				if !latestRequested {
					if !options.quietSkip {
						fmt.Printf("Node.js 包 %s@%s 已安装（%s）\n", pkg, actualVersion, profile.ID)
					}
					continue
				}
			}
		}
		if options.forceLatest && !latestRequested {
			if installedVersion := nodeDeps.Dependencies[pkg]; isNodeVersionSatisfied(installedVersion, requestedVersion) {
				if !options.quietSkip {
					fmt.Printf("Node.js 包 %s@%s 已安装（%s）\n", pkg, installedVersion, profile.ID)
				}
				continue
			}
		}

		if latestRequested {
			if options.forceLatest {
				fmt.Printf("正在安装或更新 Node.js 包: %s（%s，最新版）\n", pkg, profile.ID)
			} else {
				fmt.Printf("正在安装 Node.js 包: %s（%s，最新版）\n", pkg, profile.ID)
			}
		} else {
			fmt.Printf("正在安装 Node.js 包: %s@%s（%s）\n", pkg, requestedVersion, profile.ID)
			packageSpec = fmt.Sprintf("%s@%s", pkg, requestedVersion)
		}

		resolvedNpm, err := resolveNpm()
		if err != nil {
			return err
		}
		cmd := exec.Command(resolvedNpm, "install", packageSpec, "--save-exact")
		cmd.Dir = paths.nodeRuntimeDir
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("npm install 失败: %w", err)
		}

		installedVersion, err := getNodePackageVersionAt(paths.nodeModules, pkg)
		if err != nil {
			return err
		}
		if nodeDeps.Dependencies[pkg] != installedVersion {
			nodeDeps.Dependencies[pkg] = installedVersion
			changed = true
		}
	}

	if changed || !manifestExists {
		return m.saveNodeDepsFile(paths.nodeDepsFile, nodeDeps)
	}
	return nil
}

func isVersionSatisfied(installedVersion, requestedVersion string) bool {
	installedVersion = strings.TrimSpace(installedVersion)
	requestedVersion = strings.TrimSpace(requestedVersion)
	if installedVersion == "" || requestedVersion == "" || requestedVersion == "latest" {
		return false
	}
	if installedVersion == requestedVersion {
		return true
	}
	trimmedRequested := strings.TrimLeft(requestedVersion, "^~=")
	trimmedInstalled := strings.TrimLeft(installedVersion, "^~=")
	return trimmedInstalled == trimmedRequested
}

func isConcreteVersion(version string) bool {
	version = strings.TrimSpace(version)
	return version != "" && version != "latest"
}

func isNodeVersionSatisfied(installedVersion, requestedVersion string) bool {
	installedVersion = normalizeNodeVersion(installedVersion)
	requestedVersion = strings.TrimSpace(requestedVersion)
	if installedVersion == "" || requestedVersion == "" || requestedVersion == "latest" {
		return false
	}
	requestedVersion = strings.TrimSpace(requestedVersion)
	if strings.HasPrefix(requestedVersion, "^") {
		return isNodeCaretVersionSatisfied(installedVersion, strings.TrimSpace(strings.TrimPrefix(requestedVersion, "^")))
	}
	if strings.HasPrefix(requestedVersion, "~") {
		return isNodeTildeVersionSatisfied(installedVersion, strings.TrimSpace(strings.TrimPrefix(requestedVersion, "~")))
	}
	requestedVersion = normalizeNodeVersion(requestedVersion)
	return installedVersion == requestedVersion
}

type nodeSemver struct {
	major int
	minor int
	patch int
}

func isNodeCaretVersionSatisfied(installedVersion, requestedVersion string) bool {
	installed, ok := parseNodeSemver(installedVersion)
	if !ok {
		return false
	}
	requested, ok := parseNodeSemver(requestedVersion)
	if !ok || compareNodeSemver(installed, requested) < 0 {
		return false
	}
	if requested.major > 0 {
		return installed.major == requested.major
	}
	if requested.minor > 0 {
		return installed.major == 0 && installed.minor == requested.minor
	}
	return installed.major == 0 && installed.minor == 0 && installed.patch == requested.patch
}

func isNodeTildeVersionSatisfied(installedVersion, requestedVersion string) bool {
	installed, ok := parseNodeSemver(installedVersion)
	if !ok {
		return false
	}
	requested, ok := parseNodeSemver(requestedVersion)
	if !ok || compareNodeSemver(installed, requested) < 0 {
		return false
	}
	return installed.major == requested.major && installed.minor == requested.minor
}

func normalizeNodeVersion(version string) string {
	version = strings.TrimSpace(version)
	for version != "" {
		first := version[0]
		if first != 'v' && first != 'V' && first != '=' {
			break
		}
		version = strings.TrimSpace(version[1:])
	}
	if index := strings.IndexAny(version, "+-"); index >= 0 {
		version = version[:index]
	}
	return strings.TrimSpace(version)
}

func parseNodeSemver(version string) (nodeSemver, bool) {
	version = normalizeNodeVersion(version)
	if version == "" {
		return nodeSemver{}, false
	}
	parts := strings.Split(version, ".")
	if len(parts) > 3 {
		return nodeSemver{}, false
	}
	values := [3]int{}
	for i, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return nodeSemver{}, false
		}
		value, err := strconv.Atoi(part)
		if err != nil || value < 0 {
			return nodeSemver{}, false
		}
		values[i] = value
	}
	return nodeSemver{major: values[0], minor: values[1], patch: values[2]}, true
}

func compareNodeSemver(left, right nodeSemver) int {
	if left.major != right.major {
		return compareInt(left.major, right.major)
	}
	if left.minor != right.minor {
		return compareInt(left.minor, right.minor)
	}
	return compareInt(left.patch, right.patch)
}

func compareInt(left, right int) int {
	if left < right {
		return -1
	}
	if left > right {
		return 1
	}
	return 0
}

// GetPythonPath 获取 Python 解释器路径。
func (m *Manager) GetPythonPath() string {
	pythonPath := ""
	if os.PathSeparator == '\\' {
		pythonPath = filepath.Join(m.pythonVenv, "Scripts", "python.exe")
	} else {
		pythonPath = filepath.Join(m.pythonVenv, "bin", "python")
	}
	if absPath, err := filepath.Abs(pythonPath); err == nil {
		return absPath
	}
	return pythonPath
}

// GetNodePath 获取默认 Node.js NODE_PATH 环境变量。
func (m *Manager) GetNodePath() string {
	return m.GetNodePathForProfile("")
}

func (m *Manager) GetNodePathForProfile(profileID string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	profile, err := m.resolveProfileLocked("nodejs", profileID)
	if err != nil {
		if absPath, absErr := filepath.Abs(m.nodeModules); absErr == nil {
			return absPath
		}
		return m.nodeModules
	}
	return m.absPath(m.profileDependencyPaths(profile).nodeModules)
}

func (m *Manager) GetPythonPathForProfile(profileID string) (string, error) {
	m.mu.Lock()
	profile, err := m.resolveProfileLocked("python", profileID)
	if err != nil {
		m.mu.Unlock()
		return "", err
	}
	paths := m.profileDependencyPaths(profile)
	m.mu.Unlock()
	if isDefaultProfile(profile) {
		return m.GetPythonPath(), nil
	}
	return m.absPath(profilePythonPath(paths.pythonVenv)), nil
}

func (m *Manager) ListRuntimeProfiles() ([]RuntimeProfile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.loadRuntimeProfilesLocked()
}

func (m *Manager) SaveRuntimeProfiles(profiles []RuntimeProfile) ([]RuntimeProfile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.saveRuntimeProfilesLocked(profiles)
}

func (m *Manager) saveRuntimeProfilesLocked(profiles []RuntimeProfile) ([]RuntimeProfile, error) {
	normalized, err := m.normalizeRuntimeProfiles(profiles)
	if err != nil {
		return nil, err
	}
	data, err := json.MarshalIndent(RuntimeProfileConfig{Profiles: normalized}, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(m.runtimeProfilesFile), 0755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(m.runtimeProfilesFile, data, 0644); err != nil {
		return nil, err
	}
	return normalized, nil
}

func (m *Manager) ResolveRuntime(runtimeName, profileID string) (ResolvedRuntime, error) {
	runtimeName = normalizeRuntimeName(runtimeName)
	profiles, err := m.ListRuntimeProfiles()
	if err != nil {
		return ResolvedRuntime{}, err
	}
	profileID = strings.TrimSpace(profileID)
	for _, profile := range profiles {
		if profile.Runtime != runtimeName || !profile.Enabled {
			continue
		}
		if profileID != "" && profile.ID == profileID {
			return m.resolvedRuntime(profile)
		}
		if profileID == "" && profile.Default {
			return m.resolvedRuntime(profile)
		}
	}
	if profileID != "" {
		return ResolvedRuntime{}, fmt.Errorf("运行环境 Profile 不存在或未启用: %s", profileID)
	}
	return ResolvedRuntime{}, fmt.Errorf("未找到 %s 默认运行环境", runtimeName)
}

func (m *Manager) TestRuntimeProfile(profile RuntimeProfile) (map[string]string, error) {
	profiles, err := m.normalizeRuntimeProfiles([]RuntimeProfile{profile})
	if err != nil {
		return nil, err
	}
	profile = profiles[0]
	resolved, err := m.resolvedRuntime(profile)
	if err != nil {
		return nil, err
	}
	versionOutput, err := runRuntimeVersion(resolved.Executable)
	if err != nil {
		return nil, fmt.Errorf("测试运行环境失败: %w", err)
	}
	return map[string]string{"version_output": versionOutput}, nil
}

func (m *Manager) InitializeRuntimeProfile(profileID string, options RuntimeProfileInitOptions) (RuntimeProfileInitResult, error) {
	reportRuntimeInitProgress(options.Progress, "prepare", "正在读取运行环境配置", 5)
	profileID = strings.TrimSpace(profileID)
	if profileID == "" {
		profileID = strings.TrimSpace(options.ProfileID)
	}
	if profileID == "" {
		return RuntimeProfileInitResult{}, fmt.Errorf("运行环境 Profile ID 不能为空")
	}

	m.mu.Lock()
	profiles, err := m.loadRuntimeProfilesLocked()
	if err != nil {
		m.mu.Unlock()
		return RuntimeProfileInitResult{}, err
	}
	profileIndex := -1
	var profile RuntimeProfile
	for i, item := range profiles {
		if item.ID == profileID {
			profileIndex = i
			profile = item
			break
		}
	}
	paths := m.profileDependencyPaths(profile)
	downloader := m.downloader
	m.mu.Unlock()
	if profileIndex < 0 {
		return RuntimeProfileInitResult{}, fmt.Errorf("运行环境 Profile 不存在: %s", profileID)
	}

	result := RuntimeProfileInitResult{ProfileID: profile.ID, Runtime: profile.Runtime, Source: profile.Source, Status: "failed", EnvRoot: m.absPath(m.profileEnvRoot(profile, paths)), NodePath: m.absPath(paths.nodeModules)}
	baseExecutable := profile.Executable
	if profile.Source == "managed" {
		if !options.AutoDownload {
			return result, fmt.Errorf("自动下载运行环境需要开启 auto_download")
		}
		if downloader == nil {
			return result, fmt.Errorf("运行环境下载器不可用")
		}
		reportRuntimeInitProgress(options.Progress, "download", "正在下载托管解释器", 10)
		downloaded, err := downloader.EnsureRuntime(profile.Runtime, profile.RequestedVersion, profile.Architecture, options.Force, options.DownloadOptions, options.Progress)
		if err != nil {
			return result, err
		}
		baseExecutable = downloaded.Executable
		profile.Executable = downloaded.Executable
		result.Executable = m.absPath(downloaded.Executable)
	}

	if profile.Runtime == "python" {
		result.NodePath = ""
		reportRuntimeInitProgress(options.Progress, "venv", "正在初始化 Python 虚拟环境", 70)
		versionOutput, executable, err := m.initializePythonRuntimeProfile(profile, paths, baseExecutable, options.Force)
		result.Executable = m.absPath(executable)
		result.VersionOutput = versionOutput
		if err != nil {
			return result, err
		}
	} else {
		reportRuntimeInitProgress(options.Progress, "node", "正在初始化 Node.js 环境", 70)
		versionOutput, err := m.initializeNodeRuntimeProfile(profile, paths, baseExecutable)
		result.Executable = m.absPath(baseExecutable)
		result.VersionOutput = versionOutput
		if err != nil {
			return result, err
		}
	}

	reportRuntimeInitProgress(options.Progress, "verify", "正在校验运行环境", 90)
	if profile.Source == "managed" {
		m.mu.Lock()
		latest, err := m.loadRuntimeProfilesLocked()
		if err == nil {
			for i := range latest {
				if latest[i].ID == profile.ID {
					latest[i].Executable = profile.Executable
					_, err = m.saveRuntimeProfilesLocked(latest)
					break
				}
			}
		}
		m.mu.Unlock()
		if err != nil {
			return result, err
		}
	}

	result.Status = "initialized"
	result.Message = "运行环境初始化成功"
	reportRuntimeInitProgress(options.Progress, "completed", result.Message, 100)
	return result, nil
}

func reportRuntimeInitProgress(callback RuntimeProfileInitProgressFunc, stage, message string, progress int) {
	if callback == nil {
		return
	}
	callback(RuntimeProfileInitProgress{Stage: stage, Message: message, Progress: progress})
}

func (m *Manager) GetRuntimeProfileStatus(profileID string) (RuntimeProfileStatus, error) {
	profileID = strings.TrimSpace(profileID)
	m.mu.Lock()
	profiles, err := m.loadRuntimeProfilesLocked()
	if err != nil {
		m.mu.Unlock()
		return RuntimeProfileStatus{}, err
	}
	var profile RuntimeProfile
	found := false
	for _, item := range profiles {
		if item.ID == profileID {
			profile = item
			found = true
			break
		}
	}
	paths := m.profileDependencyPaths(profile)
	m.mu.Unlock()
	if !found {
		return RuntimeProfileStatus{}, fmt.Errorf("运行环境 Profile 不存在: %s", profileID)
	}
	return m.runtimeProfileStatus(profile, paths), nil
}

func (m *Manager) ListRuntimeProfileStatuses() ([]RuntimeProfileStatus, error) {
	m.mu.Lock()
	profiles, err := m.loadRuntimeProfilesLocked()
	if err != nil {
		m.mu.Unlock()
		return nil, err
	}
	items := make([]struct {
		profile RuntimeProfile
		paths   profileDependencyPaths
	}, 0, len(profiles))
	for _, profile := range profiles {
		items = append(items, struct {
			profile RuntimeProfile
			paths   profileDependencyPaths
		}{profile: profile, paths: m.profileDependencyPaths(profile)})
	}
	m.mu.Unlock()

	statuses := make([]RuntimeProfileStatus, 0, len(items))
	for _, item := range items {
		statuses = append(statuses, m.runtimeProfileStatus(item.profile, item.paths))
	}
	return statuses, nil
}

func (m *Manager) loadRuntimeProfilesLocked() ([]RuntimeProfile, error) {
	data, err := os.ReadFile(m.runtimeProfilesFile)
	if os.IsNotExist(err) {
		return m.defaultRuntimeProfiles(), nil
	}
	if err != nil {
		return nil, err
	}
	var config RuntimeProfileConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	return m.normalizeRuntimeProfiles(config.Profiles)
}

func (m *Manager) defaultRuntimeProfiles() []RuntimeProfile {
	return []RuntimeProfile{
		{ID: "node-default", Name: "默认 Node.js", Runtime: "nodejs", Executable: "node", Enabled: true, Default: true, Description: "使用系统 PATH 中的 node", Source: "manual", Architecture: defaultRuntimeArchitecture()},
		{ID: "python-default", Name: "默认 Python", Runtime: "python", Executable: m.GetPythonPath(), Enabled: true, Default: true, Description: "使用当前 runtime/.venv 虚拟环境", Source: "manual", Architecture: defaultRuntimeArchitecture()},
	}
}

func (m *Manager) normalizeRuntimeProfiles(profiles []RuntimeProfile) ([]RuntimeProfile, error) {
	if len(profiles) == 0 {
		profiles = m.defaultRuntimeProfiles()
	}
	seen := map[string]bool{}
	defaultByRuntime := map[string]bool{}
	for i := range profiles {
		profile := &profiles[i]
		profile.ID = strings.TrimSpace(profile.ID)
		if profile.ID == "" {
			return nil, fmt.Errorf("运行环境 ID 不能为空")
		}
		for _, char := range profile.ID {
			if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || char == '-' || char == '_' {
				continue
			}
			return nil, fmt.Errorf("运行环境 ID 只能包含字母、数字、横线和下划线")
		}
		if seen[profile.ID] {
			return nil, fmt.Errorf("运行环境 ID 重复: %s", profile.ID)
		}
		seen[profile.ID] = true
		profile.Runtime = normalizeRuntimeName(profile.Runtime)
		if profile.Runtime != "nodejs" && profile.Runtime != "python" {
			return nil, fmt.Errorf("运行环境只支持 nodejs/python: %s", profile.Runtime)
		}
		profile.Name = strings.TrimSpace(profile.Name)
		if profile.Name == "" {
			profile.Name = profile.ID
		}
		profile.Version = strings.TrimSpace(profile.Version)
		profile.Description = strings.TrimSpace(profile.Description)
		profile.Source = strings.ToLower(strings.TrimSpace(profile.Source))
		if profile.Source == "" {
			profile.Source = "manual"
		}
		if profile.Source != "manual" && profile.Source != "managed" {
			return nil, fmt.Errorf("运行环境来源只支持 manual/managed: %s", profile.Source)
		}
		profile.Architecture = strings.ToLower(strings.TrimSpace(profile.Architecture))
		if profile.Architecture == "" || shouldRefreshDefaultRuntimeArchitecture(profile) {
			profile.Architecture = defaultRuntimeArchitecture()
		}
		if !isSupportedRuntimeArchitecture(profile.Architecture) {
			return nil, fmt.Errorf("运行环境架构不支持: %s", profile.Architecture)
		}
		profile.RequestedVersion = strings.TrimSpace(profile.RequestedVersion)
		if profile.Source == "managed" && profile.RequestedVersion == "" && profile.Version != "" {
			profile.RequestedVersion = profile.Version
		}
		if profile.RequestedVersion != "" && !isSafeRuntimeVersion(profile.RequestedVersion) {
			return nil, fmt.Errorf("运行环境版本号只能包含字母、数字、点、横线和下划线")
		}
		if profile.Source == "managed" && profile.Runtime == "python" {
			profile.RequestedVersion = normalizePythonNuGetVersion(profile.RequestedVersion)
		}
		profile.Executable = strings.TrimSpace(profile.Executable)
		if profile.Source == "managed" {
			if profile.RequestedVersion == "" {
				return nil, fmt.Errorf("自动下载运行环境必须填写目标版本")
			}
			profile.Executable = m.managedRuntimeExecutable(*profile)
		} else if profile.Executable == "" {
			return nil, fmt.Errorf("手动运行环境必须填写解释器路径")
		}
		if profile.Default && !profile.Enabled {
			profile.Default = false
		}
		if profile.Default {
			if defaultByRuntime[profile.Runtime] {
				return nil, fmt.Errorf("%s 只能有一个默认运行环境", profile.Runtime)
			}
			defaultByRuntime[profile.Runtime] = true
		}
	}
	for runtimeName := range map[string]bool{"nodejs": true, "python": true} {
		if defaultByRuntime[runtimeName] {
			continue
		}
		for i := range profiles {
			if profiles[i].Runtime == runtimeName && profiles[i].Enabled {
				profiles[i].Default = true
				defaultByRuntime[runtimeName] = true
				break
			}
		}
	}
	return profiles, nil
}

func (m *Manager) resolveProfileLocked(runtimeName, profileID string) (RuntimeProfile, error) {
	runtimeName = normalizeRuntimeName(runtimeName)
	profiles, err := m.loadRuntimeProfilesLocked()
	if err != nil {
		return RuntimeProfile{}, err
	}
	profileID = strings.TrimSpace(profileID)
	for _, profile := range profiles {
		if profile.Runtime != runtimeName || !profile.Enabled {
			continue
		}
		if profileID != "" && profile.ID == profileID {
			return profile, nil
		}
		if profileID == "" && profile.Default {
			return profile, nil
		}
	}
	if profileID != "" {
		return RuntimeProfile{}, fmt.Errorf("运行环境 Profile 不存在或未启用: %s", profileID)
	}
	return RuntimeProfile{}, fmt.Errorf("未找到 %s 默认运行环境", runtimeName)
}

func (m *Manager) profileDependencyPaths(profile RuntimeProfile) profileDependencyPaths {
	if isDefaultProfile(profile) {
		return profileDependencyPaths{
			pythonVenv:     m.pythonVenv,
			pythonDepsFile: m.pythonDepsFile,
			nodeRuntimeDir: filepath.Dir(m.nodeModules),
			nodeModules:    m.nodeModules,
			nodeDepsFile:   m.nodeDepsFile,
		}
	}
	root := filepath.Join(filepath.Dir(m.runtimeProfilesFile), "envs", profile.ID)
	return profileDependencyPaths{
		pythonVenv:     filepath.Join(root, ".venv"),
		pythonDepsFile: filepath.Join(root, "python_deps.json"),
		nodeRuntimeDir: root,
		nodeModules:    filepath.Join(root, "node_modules"),
		nodeDepsFile:   filepath.Join(root, "package.json"),
	}
}

func (m *Manager) runtimeRoot() string {
	return filepath.Dir(m.runtimeProfilesFile)
}

func (m *Manager) profileEnvRoot(profile RuntimeProfile, paths profileDependencyPaths) string {
	if profile.Runtime == "python" {
		return filepath.Dir(paths.pythonVenv)
	}
	return paths.nodeRuntimeDir
}

func (m *Manager) managedRuntimeRoot(runtimeName, version, architecture string) string {
	return filepath.Join(m.runtimeRoot(), "interpreters", runtimeName, fmt.Sprintf("%s-%s", version, architecture))
}

func (m *Manager) managedRuntimeExecutable(profile RuntimeProfile) string {
	return managedRuntimeExecutableInRoot(profile.Runtime, profile.Architecture, m.managedRuntimeRoot(profile.Runtime, profile.RequestedVersion, profile.Architecture))
}

func (m *Manager) setRuntimeDownloader(downloader RuntimeDownloader) {
	m.downloader = downloader
}

func (m *Manager) ensurePythonProfileEnv(profile RuntimeProfile, venvPath string) error {
	if _, err := os.Stat(venvPath); os.IsNotExist(err) {
		if isDefaultProfile(profile) {
			return m.InitPythonEnv()
		}
		fmt.Printf("正在创建 Python 运行环境 %s 虚拟环境...\n", profile.ID)
		cmd := exec.Command(profile.Executable, "-m", "venv", venvPath)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("创建运行环境 %s 虚拟环境失败: %w", profile.ID, err)
		}
	}
	return nil
}

func (m *Manager) absPath(path string) string {
	if isCommandName(path) {
		return path
	}
	if absPath, err := filepath.Abs(path); err == nil {
		return absPath
	}
	return path
}

func isDefaultProfile(profile RuntimeProfile) bool {
	return profile.ID == "node-default" || profile.ID == "python-default"
}

func profilePythonPath(venvPath string) string {
	if os.PathSeparator == '\\' {
		return filepath.Join(venvPath, "Scripts", "python.exe")
	}
	return filepath.Join(venvPath, "bin", "python")
}

func profilePipPath(venvPath string) string {
	if os.PathSeparator == '\\' {
		return filepath.Join(venvPath, "Scripts", "pip.exe")
	}
	return filepath.Join(venvPath, "bin", "pip")
}

func (m *Manager) resolvedRuntime(profile RuntimeProfile) (ResolvedRuntime, error) {
	if !profile.Enabled {
		return ResolvedRuntime{}, fmt.Errorf("运行环境未启用: %s", profile.ID)
	}
	paths := m.profileDependencyPaths(profile)
	executable := profile.Executable
	if profile.Runtime == "python" && !isDefaultProfile(profile) {
		if err := m.ensurePythonProfileEnv(profile, paths.pythonVenv); err != nil {
			return ResolvedRuntime{}, err
		}
		executable = profilePythonPath(paths.pythonVenv)
	}
	if !isCommandName(executable) {
		absPath, err := filepath.Abs(executable)
		if err != nil {
			return ResolvedRuntime{}, err
		}
		executable = absPath
		if info, err := os.Stat(executable); err != nil || info.IsDir() {
			return ResolvedRuntime{}, fmt.Errorf("解释器不存在或不是文件: %s", executable)
		}
	}
	return ResolvedRuntime{Profile: profile, Executable: executable, NodePath: m.absPath(paths.nodeModules)}, nil
}

func normalizeRuntimeName(runtimeName string) string {
	runtimeName = strings.ToLower(strings.TrimSpace(runtimeName))
	if runtimeName == "node" {
		return "nodejs"
	}
	if runtimeName == "py" || runtimeName == "python3" {
		return "python"
	}
	return runtimeName
}

func isCommandName(value string) bool {
	return value != "" && !strings.ContainsAny(value, `/\\`)
}

func defaultRuntimeArchitecture() string {
	arch := "x64"
	if runtime.GOARCH == "arm64" {
		arch = "arm64"
	}
	switch runtime.GOOS {
	case "windows":
		return "win-" + arch
	case "linux":
		return "linux-" + arch
	default:
		return "linux-" + arch
	}
}

func isSupportedRuntimeArchitecture(architecture string) bool {
	switch architecture {
	case "win-x64", "win-arm64", "linux-x64", "linux-arm64":
		return true
	default:
		return false
	}
}

func isWindowsRuntimeArchitecture(architecture string) bool {
	return strings.HasPrefix(architecture, "win-")
}

func shouldRefreshDefaultRuntimeArchitecture(profile *RuntimeProfile) bool {
	return profile != nil && isDefaultProfile(*profile) && runtime.GOOS == "linux" && isWindowsRuntimeArchitecture(profile.Architecture)
}

func isSafeRuntimeVersion(version string) bool {
	if version == "" || len(version) > 40 {
		return false
	}
	first := version[0]
	if first < '0' || first > '9' {
		return false
	}
	for _, char := range version {
		if (char >= '0' && char <= '9') || (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || char == '.' || char == '-' || char == '_' {
			continue
		}
		return false
	}
	return true
}

func runRuntimeVersion(executable string) (string, error) {
	cmd := exec.Command(executable, "--version")
	output, err := cmd.CombinedOutput()
	trimmed := strings.TrimSpace(string(output))
	if err != nil {
		if trimmed != "" {
			return "", fmt.Errorf("%w: %s", err, trimmed)
		}
		return "", err
	}
	return trimmed, nil
}

func runRuntimeModule(executable string, args ...string) (string, error) {
	cmd := exec.Command(executable, args...)
	output, err := cmd.CombinedOutput()
	trimmed := strings.TrimSpace(string(output))
	if err != nil {
		if trimmed != "" {
			return "", fmt.Errorf("%w: %s", err, trimmed)
		}
		return "", err
	}
	return trimmed, nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func (m *Manager) initializeNodeRuntimeProfile(profile RuntimeProfile, paths profileDependencyPaths, executable string) (string, error) {
	if err := ensureExecutableAvailable(executable); err != nil {
		return "", err
	}
	if _, err := m.npmExecutableForProfile(profile); err != nil {
		return "", err
	}
	if err := os.MkdirAll(paths.nodeRuntimeDir, 0755); err != nil {
		return "", err
	}
	if _, err := os.Stat(paths.nodeDepsFile); os.IsNotExist(err) {
		if err := m.saveNodeDepsFile(paths.nodeDepsFile, &NodeDeps{Dependencies: map[string]string{}}); err != nil {
			return "", err
		}
	} else if err != nil {
		return "", err
	}
	return runRuntimeVersion(executable)
}

func (m *Manager) initializePythonRuntimeProfile(profile RuntimeProfile, paths profileDependencyPaths, baseExecutable string, force bool) (string, string, error) {
	if isDefaultProfile(profile) {
		if err := m.InitPythonEnv(); err != nil {
			return "", m.GetPythonPath(), err
		}
		executable := m.GetPythonPath()
		versionOutput, err := runRuntimeVersion(executable)
		if err != nil {
			return "", executable, err
		}
		if _, err := runRuntimeModule(executable, "-m", "pip", "--version"); err != nil {
			return versionOutput, executable, fmt.Errorf("校验 pip 失败: %w", err)
		}
		return versionOutput, executable, nil
	}
	if err := ensureExecutableAvailable(baseExecutable); err != nil {
		return "", profilePythonPath(paths.pythonVenv), err
	}
	if force {
		if err := os.RemoveAll(paths.pythonVenv); err != nil {
			return "", profilePythonPath(paths.pythonVenv), err
		}
	}
	if _, err := os.Stat(paths.pythonVenv); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(paths.pythonVenv), 0755); err != nil {
			return "", profilePythonPath(paths.pythonVenv), err
		}
		fmt.Printf("正在创建 Python 运行环境 %s 虚拟环境...\n", profile.ID)
		cmd := exec.Command(baseExecutable, "-m", "venv", paths.pythonVenv)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return "", profilePythonPath(paths.pythonVenv), fmt.Errorf("创建运行环境 %s 虚拟环境失败: %w", profile.ID, err)
		}
	} else if err != nil {
		return "", profilePythonPath(paths.pythonVenv), err
	}
	if _, err := os.Stat(paths.pythonDepsFile); os.IsNotExist(err) {
		if err := m.savePythonDepsFile(paths.pythonDepsFile, &PythonDeps{Packages: map[string]string{}}); err != nil {
			return "", profilePythonPath(paths.pythonVenv), err
		}
	} else if err != nil {
		return "", profilePythonPath(paths.pythonVenv), err
	}
	executable := profilePythonPath(paths.pythonVenv)
	versionOutput, err := runRuntimeVersion(executable)
	if err != nil {
		return "", executable, err
	}
	if _, err := runRuntimeModule(executable, "-m", "pip", "--version"); err != nil {
		return versionOutput, executable, fmt.Errorf("校验 pip 失败: %w", err)
	}
	return versionOutput, executable, nil
}

func ensureExecutableAvailable(executable string) error {
	if isCommandName(executable) {
		return nil
	}
	info, err := os.Stat(executable)
	if err != nil || info.IsDir() {
		return fmt.Errorf("解释器不存在或不是文件: %s", executable)
	}
	return nil
}

func (m *Manager) runtimeProfileStatus(profile RuntimeProfile, paths profileDependencyPaths) RuntimeProfileStatus {
	status := RuntimeProfileStatus{ProfileID: profile.ID, Runtime: profile.Runtime, Source: profile.Source, EnvRoot: m.absPath(m.profileEnvRoot(profile, paths))}
	if profile.Runtime == "nodejs" {
		status.Executable = m.absPath(profile.Executable)
		status.DepsFile = m.absPath(paths.nodeDepsFile)
		status.NodePath = m.absPath(paths.nodeModules)
		status.Initialized = fileExists(paths.nodeDepsFile)
		versionOutput, err := runRuntimeVersion(profile.Executable)
		if err != nil {
			status.Error = "解释器不可用: " + err.Error()
			status.Initialized = false
			return status
		}
		if _, err := m.npmExecutableForProfile(profile); err != nil {
			status.Error = "npm 不可用: " + err.Error()
			status.Initialized = false
			return status
		}
		status.VersionOutput = versionOutput
		return status
	}

	status.DepsFile = m.absPath(paths.pythonDepsFile)
	pythonExecutable := profilePythonPath(paths.pythonVenv)
	if isDefaultProfile(profile) {
		pythonExecutable = m.GetPythonPath()
	}
	status.Executable = m.absPath(pythonExecutable)
	status.Initialized = fileExists(paths.pythonDepsFile) && fileExists(pythonExecutable)
	if !fileExists(pythonExecutable) {
		if !isDefaultProfile(profile) {
			baseExecutable := profile.Executable
			if profile.Source == "managed" {
				baseExecutable = m.managedRuntimeExecutable(profile)
			}
			if err := ensureExecutableAvailable(baseExecutable); err != nil {
				status.Error = "解释器不可用: " + err.Error()
			}
		}
		return status
	}
	versionOutput, err := runRuntimeVersion(pythonExecutable)
	if err != nil {
		status.Error = "解释器不可用: " + err.Error()
		status.Initialized = false
		return status
	}
	status.VersionOutput = versionOutput
	if _, err := runRuntimeModule(pythonExecutable, "-m", "pip", "--version"); err != nil {
		status.Error = "pip 不可用: " + err.Error()
		status.Initialized = false
	}
	return status
}

func (m *Manager) npmExecutableForProfile(profile RuntimeProfile) (string, error) {
	executable := strings.TrimSpace(profile.Executable)
	if executable == "" {
		return "", fmt.Errorf("Node.js 解释器路径不能为空")
	}
	if isCommandName(executable) {
		if profile.Source == "managed" {
			return "", fmt.Errorf("托管 Node.js 解释器路径无效: %s", executable)
		}
		if resolvedNode, err := exec.LookPath(executable); err == nil {
			for _, name := range []string{"npm.cmd", "npm.exe", "npm"} {
				candidate := filepath.Join(filepath.Dir(resolvedNode), name)
				if fileExists(candidate) {
					return candidate, nil
				}
			}
		}
		if npmPath, err := exec.LookPath("npm"); err == nil {
			return npmPath, nil
		}
		return "", fmt.Errorf("未找到可用 npm")
	}
	for _, name := range []string{"npm.cmd", "npm.exe", "npm"} {
		candidate := filepath.Join(filepath.Dir(executable), name)
		if fileExists(candidate) {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("Node.js 解释器同目录缺少 npm.cmd: %s", filepath.Dir(executable))
}

func (m *Manager) getPipPath() string {
	pipPath := ""
	if os.PathSeparator == '\\' {
		pipPath = filepath.Join(m.pythonVenv, "Scripts", "pip.exe")
	} else {
		pipPath = filepath.Join(m.pythonVenv, "bin", "pip")
	}
	if absPath, err := filepath.Abs(pipPath); err == nil {
		return absPath
	}
	return pipPath
}

func (m *Manager) loadPythonDeps() (*PythonDeps, error) {
	return m.loadPythonDepsFile(m.pythonDepsFile)
}

func (m *Manager) loadPythonDepsFile(path string) (*PythonDeps, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return &PythonDeps{Packages: make(map[string]string)}, nil
	}

	var deps PythonDeps
	if err := json.Unmarshal(data, &deps); err != nil {
		return nil, err
	}
	if deps.Packages == nil {
		deps.Packages = make(map[string]string)
	}
	return &deps, nil
}

func (m *Manager) savePythonDeps(deps *PythonDeps) error {
	return m.savePythonDepsFile(m.pythonDepsFile, deps)
}

func (m *Manager) savePythonDepsFile(path string, deps *PythonDeps) error {
	data, err := json.MarshalIndent(deps, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func (m *Manager) loadNodeDeps() (*NodeDeps, error) {
	return m.loadNodeDepsFile(m.nodeDepsFile)
}

func (m *Manager) loadNodeDepsFile(path string) (*NodeDeps, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return &NodeDeps{Dependencies: make(map[string]string)}, nil
	}

	var deps NodeDeps
	if err := json.Unmarshal(data, &deps); err != nil {
		return nil, err
	}
	if deps.Dependencies == nil {
		deps.Dependencies = make(map[string]string)
	}
	return &deps, nil
}

func (m *Manager) saveNodeDeps(deps *NodeDeps) error {
	return m.saveNodeDepsFile(m.nodeDepsFile, deps)
}

func (m *Manager) saveNodeDepsFile(path string, deps *NodeDeps) error {
	data, err := json.MarshalIndent(deps, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// GetPythonDeps 获取默认 Python 依赖。
func (m *Manager) GetPythonDeps() (map[string]string, error) {
	return m.GetPythonDepsForProfile("")
}

func (m *Manager) GetPythonDepsForProfile(profileID string) (map[string]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	profile, err := m.resolveProfileLocked("python", profileID)
	if err != nil {
		return nil, err
	}
	paths := m.profileDependencyPaths(profile)
	deps, err := m.loadPythonDepsFile(paths.pythonDepsFile)
	if err != nil {
		return nil, err
	}
	pythonPath := m.GetPythonPath()
	if !isDefaultProfile(profile) {
		pythonPath = profilePythonPath(paths.pythonVenv)
	}
	changed := false
	for pkg, version := range deps.Packages {
		if version != "latest" && version != "" {
			continue
		}
		actualVersion, err := m.getPythonPackageVersionWithPython(pythonPath, pkg)
		if err == nil && actualVersion != "" {
			deps.Packages[pkg] = actualVersion
			changed = true
		}
	}
	if changed {
		if err := m.savePythonDepsFile(paths.pythonDepsFile, deps); err != nil {
			return nil, err
		}
	}
	return deps.Packages, nil
}

// GetNodeDeps 获取默认 Node.js 依赖。
func (m *Manager) GetNodeDeps() (map[string]string, error) {
	return m.GetNodeDepsForProfile("")
}

func (m *Manager) GetNodeDepsForProfile(profileID string) (map[string]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	profile, err := m.resolveProfileLocked("nodejs", profileID)
	if err != nil {
		return nil, err
	}
	paths := m.profileDependencyPaths(profile)
	deps, err := m.loadNodeDepsFile(paths.nodeDepsFile)
	if err != nil {
		return nil, err
	}
	changed := false
	for pkg, version := range deps.Dependencies {
		if version != "latest" && version != "" {
			continue
		}
		actualVersion, err := getNodePackageVersionAt(paths.nodeModules, pkg)
		if err == nil && actualVersion != "" {
			deps.Dependencies[pkg] = actualVersion
			changed = true
		}
	}
	if changed {
		if err := m.saveNodeDepsFile(paths.nodeDepsFile, deps); err != nil {
			return nil, err
		}
	}
	return deps.Dependencies, nil
}

// UninstallPythonDep 卸载默认 Python 依赖。
func (m *Manager) UninstallPythonDep(name string) error {
	return m.UninstallPythonDepForProfile("", name)
}

func (m *Manager) UninstallPythonDepForProfile(profileID, name string) error {
	name = strings.TrimSpace(name)
	if err := validatePythonDependencyName(name); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	profile, err := m.resolveProfileLocked("python", profileID)
	if err != nil {
		return err
	}
	paths := m.profileDependencyPaths(profile)
	installed, err := m.loadPythonDepsFile(paths.pythonDepsFile)
	if err != nil {
		return err
	}
	if _, exists := installed.Packages[name]; !exists {
		return fmt.Errorf("依赖 %s 未安装", name)
	}

	pipPath := profilePipPath(paths.pythonVenv)
	fmt.Printf("正在卸载 Python 包: %s（%s）\n", name, profile.ID)
	cmd := exec.Command(pipPath, "uninstall", "-y", name)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("卸载 %s 失败: %w", name, err)
	}

	delete(installed.Packages, name)
	return m.savePythonDepsFile(paths.pythonDepsFile, installed)
}

// UninstallNodeDep 卸载默认 Node.js 依赖。
func (m *Manager) UninstallNodeDep(name string) error {
	return m.UninstallNodeDepForProfile("", name)
}

func (m *Manager) UninstallNodeDepForProfile(profileID, name string) error {
	name = strings.TrimSpace(name)
	if err := validateNodeDependencyName(name); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	profile, err := m.resolveProfileLocked("nodejs", profileID)
	if err != nil {
		return err
	}
	paths := m.profileDependencyPaths(profile)
	npmExecutable, err := m.npmExecutableForProfile(profile)
	if err != nil {
		return err
	}
	nodeDeps, err := m.loadNodeDepsFile(paths.nodeDepsFile)
	if err != nil {
		return err
	}
	if _, exists := nodeDeps.Dependencies[name]; !exists {
		return fmt.Errorf("依赖 %s 未安装", name)
	}

	fmt.Printf("正在卸载 Node.js 包: %s（%s）\n", name, profile.ID)
	cmd := exec.Command(npmExecutable, "uninstall", name)
	cmd.Dir = paths.nodeRuntimeDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("npm uninstall 失败: %w", err)
	}

	delete(nodeDeps.Dependencies, name)
	if err := m.saveNodeDepsFile(paths.nodeDepsFile, nodeDeps); err != nil {
		return err
	}
	fmt.Println("Node.js 依赖卸载成功")
	return nil
}

func (m *Manager) getPythonPackageVersion(pkg string) (string, error) {
	return m.getPythonPackageVersionWithPython(m.GetPythonPath(), pkg)
}

func (m *Manager) getPythonPackageVersionWithPython(pythonPath, pkg string) (string, error) {
	lookupName := pythonPackageLookupName(pkg)
	cmd := exec.Command(pythonPath, "-m", "pip", "show", lookupName)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("获取 Python 包 %s 版本失败: %w", pkg, err)
	}
	for _, line := range strings.Split(string(output), "\n") {
		if strings.HasPrefix(line, "Version:") {
			version := strings.TrimSpace(strings.TrimPrefix(line, "Version:"))
			if version != "" {
				return version, nil
			}
		}
	}
	return "", fmt.Errorf("获取 Python 包 %s 版本失败: 未找到版本号", pkg)
}

func (m *Manager) getNodePackageVersion(pkg string) (string, error) {
	return getNodePackageVersionAt(m.nodeModules, pkg)
}

func getNodePackageVersionAt(nodeModules, pkg string) (string, error) {
	packageJSON := filepath.Join(nodeModules, pkg, "package.json")
	data, err := os.ReadFile(packageJSON)
	if err != nil {
		return "", fmt.Errorf("获取 Node.js 包 %s 版本失败: %w", pkg, err)
	}
	var info struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(data, &info); err != nil {
		return "", err
	}
	if info.Version == "" {
		return "", fmt.Errorf("获取 Node.js 包 %s 版本失败: 未找到版本号", pkg)
	}
	return info.Version, nil
}
