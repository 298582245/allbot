package deps

import (
	"archive/zip"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

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
		{ID: "managed-node", Name: "Managed Node", Runtime: "nodejs", Source: "managed", RequestedVersion: "18.20.4", Architecture: "linux-x64", Enabled: true},
		{ID: "python-default", Name: "默认 Python", Runtime: "python", Executable: "python", Enabled: true, Default: true},
	})
	if err == nil || !strings.Contains(err.Error(), "架构") {
		t.Fatalf("expected invalid architecture error, got %v", err)
	}
	_, err = manager.SaveRuntimeProfiles([]RuntimeProfile{
		{ID: "node-default", Name: "默认 Node.js", Runtime: "nodejs", Executable: "node", Enabled: true, Default: true},
		{ID: "python-default", Name: "默认 Python", Runtime: "python", Executable: "python", Enabled: true, Default: true},
		{ID: "managed-python", Name: "Managed Python", Runtime: "python", Source: "managed", RequestedVersion: "3.10.11", Architecture: "win-arm64", Enabled: true},
	})
	if err == nil || !strings.Contains(err.Error(), "Python 自动下载") {
		t.Fatalf("expected python architecture error, got %v", err)
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

type mockRuntimeDownloader struct {
	rootDir    string
	sourceNode string
	sourceNpm  string
	called     bool
}

func (d *mockRuntimeDownloader) EnsureRuntime(runtimeName, version, architecture string, force bool, progress RuntimeProfileInitProgressFunc) (RuntimeDownloadResult, error) {
	d.called = true
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
