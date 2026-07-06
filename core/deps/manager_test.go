package deps

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/klauspost/compress/zstd"
)

func TestValidatePythonDependencyNameAllowsExtras(t *testing.T) {
	valid := []string{"requests", "qrcode[pil]", "my-package[extra_one,extra.two]"}
	for _, name := range valid {
		if err := validatePythonDependencyName(name); err != nil {
			t.Fatalf("expected %q to be valid, got %v", name, err)
		}
	}
	invalid := []string{"-r", "--upgrade", "../pkg", "https://example.com/pkg.whl", "qrcode[pil] --index-url http://evil", "pkg[]", "pkg[bad extra]"}
	for _, name := range invalid {
		if err := validatePythonDependencyName(name); err == nil {
			t.Fatalf("expected %q to be invalid", name)
		}
	}
}

func TestPythonPackageLookupNameRemovesExtras(t *testing.T) {
	cases := map[string]string{
		"qrcode[pil]":                 "qrcode",
		"my-package[extra_one,extra]": "my-package",
		"requests":                    "requests",
	}
	for input, expected := range cases {
		if actual := pythonPackageLookupName(input); actual != expected {
			t.Fatalf("pythonPackageLookupName(%q) = %q, want %q", input, actual, expected)
		}
	}
}

func TestValidateNodeDependencyNameRejectsArgumentsAndPaths(t *testing.T) {
	valid := []string{"axios", "@scope/pkg", "my-package.name"}
	for _, name := range valid {
		if err := validateNodeDependencyName(name); err != nil {
			t.Fatalf("expected %q to be valid, got %v", name, err)
		}
	}
	invalid := []string{"--save", "../pkg", "http://example.com/pkg.tgz", "@scope/", "scope/pkg", "pkg --registry http://evil"}
	for _, name := range invalid {
		if err := validateNodeDependencyName(name); err == nil {
			t.Fatalf("expected %q to be invalid", name)
		}
	}
}

func TestValidateDependencyVersionRejectsArguments(t *testing.T) {
	valid := []string{"", "latest", "1.2.3", "^1.2.0", "~1.2.0", "1.0.0-beta.1+build.1"}
	for _, version := range valid {
		if err := validateDependencyVersion(version); err != nil {
			t.Fatalf("expected %q to be valid, got %v", version, err)
		}
	}
	invalid := []string{"1.0.0 --index-url http://evil", "../1.0.0", "https://example.com/pkg"}
	for _, version := range invalid {
		if err := validateDependencyVersion(version); err == nil {
			t.Fatalf("expected %q to be invalid", version)
		}
	}
}

func TestEnsurePythonLatestSkipsWhenRecordedVersionExists(t *testing.T) {
	runtimeDir := t.TempDir()
	manager := NewManager(runtimeDir)
	if err := os.MkdirAll(filepath.Join(runtimeDir, ".venv"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := manager.savePythonDepsFile(filepath.Join(runtimeDir, "python_deps.json"), &PythonDeps{Packages: map[string]string{"requests": "2.34.2"}}); err != nil {
		t.Fatal(err)
	}

	if err := manager.EnsurePythonDepsForProfile("", map[string]string{"requests": "latest"}); err != nil {
		t.Fatal(err)
	}

	deps, err := manager.loadPythonDepsFile(filepath.Join(runtimeDir, "python_deps.json"))
	if err != nil {
		t.Fatal(err)
	}
	if deps.Packages["requests"] != "2.34.2" {
		t.Fatalf("expected recorded version to remain, got %#v", deps.Packages)
	}
}

func TestEnsurePythonDepsSkipsAfterManagerRecreated(t *testing.T) {
	runtimeDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(runtimeDir, ".venv"), 0755); err != nil {
		t.Fatal(err)
	}
	first := NewManager(runtimeDir)
	if err := first.savePythonDepsFile(filepath.Join(runtimeDir, "python_deps.json"), &PythonDeps{Packages: map[string]string{"requests": "2.34.2"}}); err != nil {
		t.Fatal(err)
	}
	if err := first.EnsurePythonDepsForProfile("", map[string]string{"requests": "latest"}); err != nil {
		t.Fatal(err)
	}

	second := NewManager(runtimeDir)
	if err := second.EnsurePythonDepsForProfile("", map[string]string{"requests": "latest"}); err != nil {
		t.Fatal(err)
	}
	deps, err := second.loadPythonDepsFile(filepath.Join(runtimeDir, "python_deps.json"))
	if err != nil {
		t.Fatal(err)
	}
	if deps.Packages["requests"] != "2.34.2" {
		t.Fatalf("expected recreated manager to keep recorded version, got %#v", deps.Packages)
	}
}

func TestNodeVersionSatisfiedSupportsCommonSemverRanges(t *testing.T) {
	cases := []struct {
		installed string
		requested string
		expected  bool
	}{
		{installed: "1.16.0", requested: "^1.6.0", expected: true},
		{installed: "2.0.0", requested: "^1.6.0", expected: false},
		{installed: "1.6.5", requested: "~1.6.0", expected: true},
		{installed: "1.7.0", requested: "~1.6.0", expected: false},
		{installed: "1.6.0", requested: "1.6.0", expected: true},
		{installed: "v1.6.0", requested: "=1.6.0", expected: true},
	}
	for _, tc := range cases {
		if actual := isNodeVersionSatisfied(tc.installed, tc.requested); actual != tc.expected {
			t.Fatalf("isNodeVersionSatisfied(%q, %q) = %v, want %v", tc.installed, tc.requested, actual, tc.expected)
		}
	}
}

func TestEnsureNodeSemverRangeSkipsSatisfiedInstalledVersion(t *testing.T) {
	runtimeDir := t.TempDir()
	manager := NewManager(runtimeDir)
	writeNodePackageForTest(t, filepath.Join(runtimeDir, "node_modules"), "axios", "1.16.0")

	if err := manager.EnsureNodeDepsForProfile("", map[string]string{"axios": "^1.6.0"}); err != nil {
		t.Fatal(err)
	}

	deps, err := manager.loadNodeDepsFile(filepath.Join(runtimeDir, "package.json"))
	if err != nil {
		t.Fatal(err)
	}
	if deps.Dependencies["axios"] != "1.16.0" {
		t.Fatalf("expected manifest to record actual version, got %#v", deps.Dependencies)
	}
}

func TestEnsureNodeDepsSkipsAfterManagerRecreated(t *testing.T) {
	runtimeDir := t.TempDir()
	writeNodePackageForTest(t, filepath.Join(runtimeDir, "node_modules"), "axios", "1.16.0")
	first := NewManager(runtimeDir)
	if err := first.EnsureNodeDepsForProfile("", map[string]string{"axios": "latest"}); err != nil {
		t.Fatal(err)
	}

	second := NewManager(runtimeDir)
	if err := second.EnsureNodeDepsForProfile("", map[string]string{"axios": "latest"}); err != nil {
		t.Fatal(err)
	}
	deps, err := second.loadNodeDepsFile(filepath.Join(runtimeDir, "package.json"))
	if err != nil {
		t.Fatal(err)
	}
	if deps.Dependencies["axios"] != "1.16.0" {
		t.Fatalf("expected recreated manager to keep actual version, got %#v", deps.Dependencies)
	}
}

func writeNodePackageForTest(t *testing.T, nodeModules, pkg, version string) {
	t.Helper()
	packageDir := filepath.Join(nodeModules, pkg)
	if err := os.MkdirAll(packageDir, 0755); err != nil {
		t.Fatal(err)
	}
	data := []byte(`{"version":"` + version + `"}`)
	if err := os.WriteFile(filepath.Join(packageDir, "package.json"), data, 0644); err != nil {
		t.Fatal(err)
	}
}

func TestProfileDependencyPathsAreIsolated(t *testing.T) {
	runtimeDir := t.TempDir()
	manager := NewManager(runtimeDir)

	profiles, err := manager.SaveRuntimeProfiles([]RuntimeProfile{
		{ID: "node-default", Name: "默认 Node.js", Runtime: "nodejs", Executable: "node", Enabled: true, Default: true},
		{ID: "node18", Name: "Node.js 18", Runtime: "nodejs", Executable: "node", Enabled: true},
		{ID: "python-default", Name: "默认 Python", Runtime: "python", Executable: "python", Enabled: true, Default: true},
		{ID: "python310", Name: "Python 3.10", Runtime: "python", Executable: "python", Enabled: true},
	})
	if err != nil {
		t.Fatal(err)
	}

	var nodeDefault, node18, pythonDefault, python310 RuntimeProfile
	for _, profile := range profiles {
		switch profile.ID {
		case "node-default":
			nodeDefault = profile
		case "node18":
			node18 = profile
		case "python-default":
			pythonDefault = profile
		case "python310":
			python310 = profile
		}
	}

	defaultNodePaths := manager.profileDependencyPaths(nodeDefault)
	customNodePaths := manager.profileDependencyPaths(node18)
	if defaultNodePaths.nodeDepsFile == customNodePaths.nodeDepsFile || defaultNodePaths.nodeModules == customNodePaths.nodeModules {
		t.Fatalf("Node.js 依赖路径未隔离: default=%#v custom=%#v", defaultNodePaths, customNodePaths)
	}

	defaultPythonPaths := manager.profileDependencyPaths(pythonDefault)
	customPythonPaths := manager.profileDependencyPaths(python310)
	if defaultPythonPaths.pythonDepsFile == customPythonPaths.pythonDepsFile || defaultPythonPaths.pythonVenv == customPythonPaths.pythonVenv {
		t.Fatalf("Python 依赖路径未隔离: default=%#v custom=%#v", defaultPythonPaths, customPythonPaths)
	}
}

func TestGetDepsForProfileReadsIsolatedManifests(t *testing.T) {
	runtimeDir := t.TempDir()
	manager := NewManager(runtimeDir)
	profiles, err := manager.SaveRuntimeProfiles([]RuntimeProfile{
		{ID: "node-default", Name: "默认 Node.js", Runtime: "nodejs", Executable: "node", Enabled: true, Default: true},
		{ID: "node18", Name: "Node.js 18", Runtime: "nodejs", Executable: "node", Enabled: true},
		{ID: "python-default", Name: "默认 Python", Runtime: "python", Executable: "python", Enabled: true, Default: true},
		{ID: "python310", Name: "Python 3.10", Runtime: "python", Executable: "python", Enabled: true},
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, profile := range profiles {
		paths := manager.profileDependencyPaths(profile)
		switch profile.ID {
		case "node-default":
			if err := manager.saveNodeDepsFile(paths.nodeDepsFile, &NodeDeps{Dependencies: map[string]string{"axios": "1.6.0"}}); err != nil {
				t.Fatal(err)
			}
		case "node18":
			if err := manager.saveNodeDepsFile(paths.nodeDepsFile, &NodeDeps{Dependencies: map[string]string{"axios": "1.7.0"}}); err != nil {
				t.Fatal(err)
			}
		case "python-default":
			if err := manager.savePythonDepsFile(paths.pythonDepsFile, &PythonDeps{Packages: map[string]string{"requests": "2.31.0"}}); err != nil {
				t.Fatal(err)
			}
		case "python310":
			if err := manager.savePythonDepsFile(paths.pythonDepsFile, &PythonDeps{Packages: map[string]string{"requests": "2.32.0"}}); err != nil {
				t.Fatal(err)
			}
		}
	}

	defaultNodeDeps, err := manager.GetNodeDepsForProfile("")
	if err != nil {
		t.Fatal(err)
	}
	customNodeDeps, err := manager.GetNodeDepsForProfile("node18")
	if err != nil {
		t.Fatal(err)
	}
	if defaultNodeDeps["axios"] != "1.6.0" || customNodeDeps["axios"] != "1.7.0" {
		t.Fatalf("Node.js 依赖读取未隔离: default=%#v custom=%#v", defaultNodeDeps, customNodeDeps)
	}

	defaultPythonDeps, err := manager.GetPythonDepsForProfile("")
	if err != nil {
		t.Fatal(err)
	}
	customPythonDeps, err := manager.GetPythonDepsForProfile("python310")
	if err != nil {
		t.Fatal(err)
	}
	if defaultPythonDeps["requests"] != "2.31.0" || customPythonDeps["requests"] != "2.32.0" {
		t.Fatalf("Python 依赖读取未隔离: default=%#v custom=%#v", defaultPythonDeps, customPythonDeps)
	}
}

func TestManualNodeProfileInitializationCreatesManifest(t *testing.T) {
	nodePath, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node 不可用，跳过 Node.js 初始化测试")
	}
	runtimeDir := t.TempDir()
	manager := NewManager(runtimeDir)
	if _, err := manager.SaveRuntimeProfiles([]RuntimeProfile{
		{ID: "node-default", Name: "默认 Node.js", Runtime: "nodejs", Executable: "node", Enabled: true, Default: true},
		{ID: "node18", Name: "Node.js 18", Runtime: "nodejs", Executable: nodePath, Enabled: true},
		{ID: "python-default", Name: "默认 Python", Runtime: "python", Executable: "python", Enabled: true, Default: true},
	}); err != nil {
		t.Fatal(err)
	}
	result, err := manager.InitializeRuntimeProfile("node18", RuntimeProfileInitOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "initialized" || result.NodePath == "" || result.VersionOutput == "" {
		t.Fatalf("unexpected init result: %#v", result)
	}
	if !fileExists(filepath.Join(runtimeDir, "envs", "node18", "package.json")) {
		t.Fatal("package.json 未生成")
	}
}

func TestManualPythonProfileInitializationCreatesVenvAndManifest(t *testing.T) {
	pythonPath, err := exec.LookPath("python")
	if err != nil {
		pythonPath, err = exec.LookPath("python3")
	}
	if err != nil {
		t.Skip("python 不可用，跳过 Python 初始化测试")
	}
	runtimeDir := t.TempDir()
	manager := NewManager(runtimeDir)
	if _, err := manager.SaveRuntimeProfiles([]RuntimeProfile{
		{ID: "node-default", Name: "默认 Node.js", Runtime: "nodejs", Executable: "node", Enabled: true, Default: true},
		{ID: "python-default", Name: "默认 Python", Runtime: "python", Executable: pythonPath, Enabled: true, Default: true},
		{ID: "python310", Name: "Python 3.10", Runtime: "python", Executable: pythonPath, Enabled: true},
	}); err != nil {
		t.Fatal(err)
	}
	result, err := manager.InitializeRuntimeProfile("python310", RuntimeProfileInitOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "initialized" || result.VersionOutput == "" {
		t.Fatalf("unexpected init result: %#v", result)
	}
	if !fileExists(filepath.Join(runtimeDir, "envs", "python310", "python_deps.json")) {
		t.Fatal("python_deps.json 未生成")
	}
	if !fileExists(profilePythonPath(filepath.Join(runtimeDir, "envs", "python310", ".venv"))) {
		t.Fatal("虚拟环境 Python 未生成")
	}
}

func TestRuntimeProfileStatusDistinguishesStates(t *testing.T) {
	runtimeDir := t.TempDir()
	missingNode := filepath.Join(runtimeDir, "missing", "node.exe")
	manager := NewManager(runtimeDir)
	profiles, err := manager.SaveRuntimeProfiles([]RuntimeProfile{
		{ID: "node-default", Name: "默认 Node.js", Runtime: "nodejs", Executable: "node", Enabled: true, Default: true},
		{ID: "node-missing", Name: "Missing Node", Runtime: "nodejs", Executable: missingNode, Enabled: true},
		{ID: "python-default", Name: "默认 Python", Runtime: "python", Executable: "python", Enabled: true, Default: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, profile := range profiles {
		if profile.ID == "node-default" {
			if err := manager.saveNodeDepsFile(manager.profileDependencyPaths(profile).nodeDepsFile, &NodeDeps{Dependencies: map[string]string{}}); err != nil {
				t.Fatal(err)
			}
		}
	}
	statuses, err := manager.ListRuntimeProfileStatuses()
	if err != nil {
		t.Fatal(err)
	}
	statusMap := map[string]RuntimeProfileStatus{}
	for _, status := range statuses {
		statusMap[status.ProfileID] = status
	}
	if statusMap["node-default"].Error == "" && !statusMap["node-default"].Initialized {
		t.Fatalf("默认 Node.js 应在 node 可用时显示已初始化或在不可用时给出错误: %#v", statusMap["node-default"])
	}
	if statusMap["node-missing"].Error == "" || statusMap["node-missing"].Initialized {
		t.Fatalf("缺失解释器状态错误: %#v", statusMap["node-missing"])
	}
}

func TestDefaultProfilesUseHostArchitecture(t *testing.T) {
	manager := NewManager(t.TempDir())
	profiles, err := manager.ListRuntimeProfiles()
	if err != nil {
		t.Fatal(err)
	}
	expected := defaultRuntimeArchitecture()
	for _, profile := range profiles {
		if profile.Architecture != expected {
			t.Fatalf("expected default architecture %s, got %#v", expected, profiles)
		}
	}
}

func TestManagedProfileValidationRejectsInvalidVersionAndArchitecture(t *testing.T) {
	manager := NewManager(t.TempDir())
	_, err := manager.SaveRuntimeProfiles([]RuntimeProfile{
		{ID: "node-default", Name: "默认 Node.js", Runtime: "nodejs", Executable: "node", Enabled: true, Default: true},
		{ID: "managed-node", Name: "Managed Node", Runtime: "nodejs", Source: "managed", RequestedVersion: "18/evil", Architecture: "win-x64", Enabled: true},
		{ID: "python-default", Name: "默认 Python", Runtime: "python", Executable: "python", Enabled: true, Default: true},
	})
	if err == nil || !strings.Contains(err.Error(), "版本号") {
		t.Fatalf("expected invalid version error, got %v", err)
	}
	_, err = manager.SaveRuntimeProfiles([]RuntimeProfile{
		{ID: "node-default", Name: "默认 Node.js", Runtime: "nodejs", Executable: "node", Enabled: true, Default: true},
		{ID: "managed-node", Name: "Managed Node", Runtime: "nodejs", Source: "managed", RequestedVersion: "18.20.4", Architecture: "linux-riscv64", Enabled: true},
		{ID: "python-default", Name: "默认 Python", Runtime: "python", Executable: "python", Enabled: true, Default: true},
	})
	if err == nil || !strings.Contains(err.Error(), "架构") {
		t.Fatalf("expected invalid architecture error, got %v", err)
	}
	_, err = manager.SaveRuntimeProfiles([]RuntimeProfile{
		{ID: "node-default", Name: "默认 Node.js", Runtime: "nodejs", Executable: "node", Enabled: true, Default: true},
		{ID: "python-default", Name: "默认 Python", Runtime: "python", Executable: "python", Enabled: true, Default: true},
		{ID: "managed-python", Name: "Managed Python", Runtime: "python", Source: "managed", RequestedVersion: "3.10.11", Architecture: "linux-x64", Enabled: true},
	})
	if err != nil {
		t.Fatalf("expected linux python managed profile to be valid, got %v", err)
	}
}

func TestManagedProfileInitializationUsesDownloader(t *testing.T) {
	nodePath, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node 不可用，跳过托管初始化 mock 测试")
	}
	npmPath, err := exec.LookPath("npm")
	if err != nil {
		t.Skip("npm 不可用，跳过托管初始化 mock 测试")
	}
	runtimeDir := t.TempDir()
	manager := NewManager(runtimeDir)
	downloader := &mockRuntimeDownloader{rootDir: runtimeDir, sourceNode: nodePath, sourceNpm: npmPath}
	manager.setRuntimeDownloader(downloader)
	if _, err := manager.SaveRuntimeProfiles([]RuntimeProfile{
		{ID: "node-default", Name: "默认 Node.js", Runtime: "nodejs", Executable: "node", Enabled: true, Default: true},
		{ID: "managed-node", Name: "Managed Node", Runtime: "nodejs", Source: "managed", RequestedVersion: "18.20.4", Architecture: "win-x64", Enabled: true},
		{ID: "python-default", Name: "默认 Python", Runtime: "python", Executable: "python", Enabled: true, Default: true},
	}); err != nil {
		t.Fatal(err)
	}
	result, err := manager.InitializeRuntimeProfile("managed-node", RuntimeProfileInitOptions{AutoDownload: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "initialized" || !downloader.called {
		t.Fatalf("unexpected result=%#v downloader=%#v", result, downloader)
	}
	if downloader.lastOptions.NodeMirrorURL != "" {
		t.Fatalf("expected empty default options, got %#v", downloader.lastOptions)
	}
	profiles, err := manager.ListRuntimeProfiles()
	if err != nil {
		t.Fatal(err)
	}
	for _, profile := range profiles {
		if profile.ID == "managed-node" && profile.Executable == "" {
			t.Fatal("托管解释器路径未写回")
		}
	}
}

func TestManagedProfileInitializationPassesDownloadOptions(t *testing.T) {
	nodePath, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node 不可用，跳过托管初始化 mock 测试")
	}
	npmPath, err := exec.LookPath("npm")
	if err != nil {
		t.Skip("npm 不可用，跳过托管初始化 mock 测试")
	}
	runtimeDir := t.TempDir()
	manager := NewManager(runtimeDir)
	downloader := &mockRuntimeDownloader{rootDir: runtimeDir, sourceNode: nodePath, sourceNpm: npmPath}
	manager.setRuntimeDownloader(downloader)
	if _, err := manager.SaveRuntimeProfiles([]RuntimeProfile{
		{ID: "node-default", Name: "默认 Node.js", Runtime: "nodejs", Executable: "node", Enabled: true, Default: true},
		{ID: "managed-node", Name: "Managed Node", Runtime: "nodejs", Source: "managed", RequestedVersion: "18.20.4", Architecture: "win-x64", Enabled: true},
		{ID: "python-default", Name: "默认 Python", Runtime: "python", Executable: "python", Enabled: true, Default: true},
	}); err != nil {
		t.Fatal(err)
	}
	options := RuntimeDownloadOptions{ProxyURL: "http://127.0.0.1:7890", NodeMirrorURL: "https://npmmirror.com/mirrors/node"}
	if _, err := manager.InitializeRuntimeProfile("managed-node", RuntimeProfileInitOptions{AutoDownload: true, DownloadOptions: options}); err != nil {
		t.Fatal(err)
	}
	if downloader.lastOptions != options {
		t.Fatalf("expected options %#v, got %#v", options, downloader.lastOptions)
	}
}

func TestRuntimeDownloadSpecsUseMirrors(t *testing.T) {
	downloader := NewHTTPRuntimeDownloader(t.TempDir())
	nodeSpec, err := downloader.nodeDownloadSpec("18.20.4", "win-x64", RuntimeDownloadOptions{NodeMirrorURL: "https://npmmirror.com/mirrors/node/"})
	if err != nil {
		t.Fatal(err)
	}
	if nodeSpec.URL != "https://npmmirror.com/mirrors/node/v18.20.4/node-v18.20.4-win-x64.zip" || nodeSpec.SHA256URL != "https://npmmirror.com/mirrors/node/v18.20.4/SHASUMS256.txt" {
		t.Fatalf("unexpected node urls: %#v", nodeSpec)
	}
	linuxNodeSpec, err := downloader.nodeDownloadSpec("18.20.4", "linux-x64", RuntimeDownloadOptions{NodeMirrorURL: "https://npmmirror.com/mirrors/node/"})
	if err != nil {
		t.Fatal(err)
	}
	if linuxNodeSpec.URL != "https://npmmirror.com/mirrors/node/v18.20.4/node-v18.20.4-linux-x64.tar.gz" || linuxNodeSpec.Executable != filepath.Join(linuxNodeSpec.RootDir, "bin", "node") {
		t.Fatalf("unexpected linux node spec: %#v", linuxNodeSpec)
	}
	if !containsString(nodeSpec.TrustedHosts, "npmmirror.com") || !containsString(nodeSpec.HashTrustedHosts, "npmmirror.com") {
		t.Fatalf("node trusted hosts missing mirror: %#v %#v", nodeSpec.TrustedHosts, nodeSpec.HashTrustedHosts)
	}

	pythonSpec, err := downloader.pythonDownloadSpec(http.DefaultClient, "3.10.11", "win-x64", RuntimeDownloadOptions{PythonPackageMirrorURL: "https://mirror.example.com/nuget/python/", PythonMetadataURL: "https://mirror.example.com/nuget/index.json/"})
	if err != nil {
		t.Fatal(err)
	}
	if managedRuntimeExecutableInRoot("python", "linux-x64", filepath.Join("runtime", "python")) != filepath.Join("runtime", "python", "bin", "python3") {
		t.Fatal("linux managed python executable path should use bin/python3")
	}
	if pythonSpec.URL != "https://mirror.example.com/nuget/python/3.10.11" || pythonSpec.NuGetIndexURL != "https://mirror.example.com/nuget/index.json" {
		t.Fatalf("unexpected python urls: %#v", pythonSpec)
	}
	if !containsString(pythonSpec.TrustedHosts, "mirror.example.com") || !containsString(pythonSpec.HashTrustedHosts, "mirror.example.com") {
		t.Fatalf("python trusted hosts missing mirror: %#v %#v", pythonSpec.TrustedHosts, pythonSpec.HashTrustedHosts)
	}
}

func TestRuntimeDownloadSpecsKeepDefaultURLs(t *testing.T) {
	downloader := NewHTTPRuntimeDownloader(t.TempDir())
	nodeSpec, err := downloader.nodeDownloadSpec("18.20.4", "win-x64", RuntimeDownloadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if nodeSpec.URL != "https://nodejs.org/dist/v18.20.4/node-v18.20.4-win-x64.zip" || nodeSpec.SHA256URL != "https://nodejs.org/dist/v18.20.4/SHASUMS256.txt" || nodeSpec.Executable != filepath.Join(nodeSpec.RootDir, "node.exe") {
		t.Fatalf("unexpected default node urls: %#v", nodeSpec)
	}
	pythonSpec, err := downloader.pythonDownloadSpec(http.DefaultClient, "3.10.11", "win-x64", RuntimeDownloadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if pythonSpec.URL != "https://www.nuget.org/api/v2/package/python/3.10.11" || pythonSpec.NuGetIndexURL != "https://api.nuget.org/v3/registration5-gz-semver2/python/index.json" {
		t.Fatalf("unexpected default python urls: %#v", pythonSpec)
	}
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func TestNodeProfileNpmPathPrefersExecutableDirectory(t *testing.T) {
	runtimeDir := t.TempDir()
	manager := NewManager(runtimeDir)
	nodeDir := filepath.Join(runtimeDir, "node")
	if err := os.MkdirAll(nodeDir, 0755); err != nil {
		t.Fatal(err)
	}
	nodePath := filepath.Join(nodeDir, "node.exe")
	if err := os.WriteFile(nodePath, []byte("node"), 0644); err != nil {
		t.Fatal(err)
	}
	npmPath := filepath.Join(nodeDir, "npm.cmd")
	if err := os.WriteFile(npmPath, []byte("npm"), 0644); err != nil {
		t.Fatal(err)
	}
	npm, err := manager.npmExecutableForProfile(RuntimeProfile{ID: "node18", Runtime: "nodejs", Executable: nodePath, Source: "manual"})
	if err != nil {
		t.Fatal(err)
	}
	if npm != npmPath {
		t.Fatalf("expected %s, got %s", npmPath, npm)
	}
}

func TestUntarSafeRestoresRelativeSymlink(t *testing.T) {
	tarPath := filepath.Join(t.TempDir(), "node.tar.gz")
	if err := writeTarGzipEntriesForTest(tarPath, []tarTestEntry{
		{Name: "node/bin/node", Data: []byte("node")},
		{Name: "node/lib/node_modules/npm/bin/npm-cli.js", Data: []byte("npm")},
		{Name: "node/bin/npm", LinkName: "../lib/node_modules/npm/bin/npm-cli.js"},
	}); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "out")
	if err := untarGzipSafe(tarPath, out); err != nil {
		t.Fatal(err)
	}
	linkPath := filepath.Join(out, "node", "bin", "npm")
	data, err := os.ReadFile(linkPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "npm" {
		t.Fatalf("expected npm shim content, got %q", data)
	}
}

func TestUntarSafeRejectsSymlinkEscape(t *testing.T) {
	tarPath := filepath.Join(t.TempDir(), "bad-link.tar.gz")
	if err := writeTarGzipEntriesForTest(tarPath, []tarTestEntry{{Name: "node/bin/npm", LinkName: "../../evil"}}); err != nil {
		t.Fatal(err)
	}
	if err := untarGzipSafe(tarPath, filepath.Join(t.TempDir(), "out")); err == nil {
		t.Fatal("expected escaping symlink to be rejected")
	}
}

func TestDownloaderRejectsUntrustedURLAndZipSlip(t *testing.T) {
	if err := validateTrustedURL("https://evil.example/runtime.zip", []string{"nodejs.org"}); err == nil {
		t.Fatal("expected untrusted URL to be rejected")
	}
	zipPath := filepath.Join(t.TempDir(), "bad.zip")
	if err := writeZipForTest(zipPath, "../evil.txt", []byte("evil")); err != nil {
		t.Fatal(err)
	}
	if err := unzipSafe(zipPath, filepath.Join(t.TempDir(), "out")); err == nil {
		t.Fatal("expected zip slip archive to be rejected")
	}
	tarPath := filepath.Join(t.TempDir(), "bad.tar.gz")
	if err := writeTarGzipForTest(tarPath, "../evil.txt", []byte("evil")); err != nil {
		t.Fatal(err)
	}
	if err := untarGzipSafe(tarPath, filepath.Join(t.TempDir(), "out")); err == nil {
		t.Fatal("expected tar slip archive to be rejected")
	}
	zstdPath := filepath.Join(t.TempDir(), "bad.tar.zst")
	if err := writeTarZstdForTest(zstdPath, "../evil.txt", []byte("evil")); err != nil {
		t.Fatal(err)
	}
	if err := untarZstdSafe(zstdPath, filepath.Join(t.TempDir(), "out")); err == nil {
		t.Fatal("expected zstd tar slip archive to be rejected")
	}
}

func writeZipForTest(path, name string, data []byte) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	writer := zip.NewWriter(file)
	entry, err := writer.Create(name)
	if err != nil {
		writer.Close()
		return err
	}
	if _, err := entry.Write(data); err != nil {
		writer.Close()
		return err
	}
	return writer.Close()
}

func writeTarGzipForTest(path, name string, data []byte) error {
	return writeTarGzipEntriesForTest(path, []tarTestEntry{{Name: name, Data: data}})
}

type tarTestEntry struct {
	Name     string
	Data     []byte
	LinkName string
}

func writeTarGzipEntriesForTest(path string, entries []tarTestEntry) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	gzipWriter := gzip.NewWriter(file)
	defer gzipWriter.Close()
	writer := tar.NewWriter(gzipWriter)
	defer writer.Close()
	for _, entry := range entries {
		if entry.LinkName != "" {
			if err := writer.WriteHeader(&tar.Header{Name: entry.Name, Typeflag: tar.TypeSymlink, Mode: 0755, Linkname: entry.LinkName}); err != nil {
				return err
			}
			continue
		}
		if err := writer.WriteHeader(&tar.Header{Name: entry.Name, Mode: 0644, Size: int64(len(entry.Data))}); err != nil {
			return err
		}
		if _, err := writer.Write(entry.Data); err != nil {
			return err
		}
	}
	return nil
}

func writeTarZstdForTest(path, name string, data []byte) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	zstdWriter, err := zstd.NewWriter(file)
	if err != nil {
		return err
	}
	defer zstdWriter.Close()
	return writeTarForTest(zstdWriter, name, data)
}

func writeTarForTest(target io.Writer, name string, data []byte) error {
	writer := tar.NewWriter(target)
	defer writer.Close()
	if err := writer.WriteHeader(&tar.Header{Name: name, Mode: 0644, Size: int64(len(data))}); err != nil {
		return err
	}
	_, err := writer.Write(data)
	return err
}

type mockRuntimeDownloader struct {
	rootDir     string
	sourceNode  string
	sourceNpm   string
	called      bool
	lastOptions RuntimeDownloadOptions
}

func (d *mockRuntimeDownloader) EnsureRuntime(runtimeName, version, architecture string, force bool, options RuntimeDownloadOptions, progress RuntimeProfileInitProgressFunc) (RuntimeDownloadResult, error) {
	d.called = true
	d.lastOptions = options
	reportRuntimeInitProgress(progress, "download", "mock 下载完成", 60)
	root := filepath.Join(d.rootDir, "mock", runtimeName, version+"-"+architecture)
	if err := os.MkdirAll(root, 0755); err != nil {
		return RuntimeDownloadResult{}, err
	}
	executable := filepath.Join(root, filepath.Base(d.sourceNode))
	if err := copyFileForTest(d.sourceNode, executable); err != nil {
		return RuntimeDownloadResult{}, err
	}
	if err := copyFileForTest(d.sourceNpm, filepath.Join(root, filepath.Base(d.sourceNpm))); err != nil {
		return RuntimeDownloadResult{}, err
	}
	return RuntimeDownloadResult{Runtime: runtimeName, Version: version, Architecture: architecture, Executable: executable, RootDir: root}, nil
}

func copyFileForTest(source, target string) error {
	data, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	return os.WriteFile(target, data, 0755)
}
