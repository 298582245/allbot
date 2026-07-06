package builtin

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

func replySystem(ctx *Context) error {
	return ctx.SendText(SystemInfo(ctx.StartTime))
}

func SystemInfo(startTime time.Time) string {
	return FormatSystemInfo(systemDescription(), runtimeArchitectureDescription(), processorDescription(), coreThreadDescription(), formatReplyDuration(time.Since(startTime)), memoryInfo(), diskInfo("."), allBotMemoryUsage(), allBotDiskUsage())
}

func FormatSystemInfo(systemName string, architecture string, processor string, cores string, uptime string, memory string, disk string, appMemory string, appDisk string) string {
	if strings.TrimSpace(systemName) == "" {
		systemName = runtime.GOOS
	}
	if strings.TrimSpace(architecture) == "" {
		architecture = runtimeArchitectureDescription()
	}
	if strings.TrimSpace(processor) == "" {
		processor = "未知"
	}
	if strings.TrimSpace(cores) == "" {
		cores = coreThreadDescription()
	}
	return fmt.Sprintf("系统信息\n系统：%s\n运行架构：%s\n处理器：%s\n核心数：%s\n运行时间：%s\n内存信息：%s\n磁盘信息：%s\nallBot\n内存占用：%s\n磁盘占用：%s", systemName, architecture, processor, cores, uptime, memory, disk, appMemory, appDisk)
}

func runtimeArchitectureDescription() string {
	return fmt.Sprintf("%s（GOOS=%s，GOARCH=%s）", runtimeProfileArchitecture(), runtime.GOOS, runtime.GOARCH)
}

func runtimeProfileArchitecture() string {
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
		return runtime.GOOS + "-" + arch
	}
}

func systemDescription() string {
	switch runtime.GOOS {
	case "windows":
		return windowsSystemDescription()
	case "linux":
		return linuxSystemDescription()
	case "darwin":
		return darwinSystemDescription()
	default:
		return runtime.GOOS
	}
}

func processorDescription() string {
	switch runtime.GOOS {
	case "windows":
		if value := windowsRegistryValue(`HKLM\HARDWARE\DESCRIPTION\System\CentralProcessor\0`, "ProcessorNameString"); value != "" {
			return value
		}
	case "linux":
		if value := linuxCPUModel(); value != "" {
			return value
		}
	case "darwin":
		if value := commandOutput("sysctl", "-n", "machdep.cpu.brand_string"); value != "" {
			return value
		}
	}
	return runtime.GOARCH
}

func coreThreadDescription() string {
	threads := runtime.NumCPU()
	cores := physicalCoreCount()
	if cores <= 0 {
		cores = threads
	}
	return fmt.Sprintf("%d核心%d线程", cores, threads)
}

func physicalCoreCount() int {
	switch runtime.GOOS {
	case "windows":
		return windowsPhysicalCoreCount()
	case "linux":
		return linuxPhysicalCoreCount()
	case "darwin":
		return parsePositiveInt(commandOutput("sysctl", "-n", "hw.physicalcpu"))
	default:
		return 0
	}
}

func windowsPhysicalCoreCount() int {
	output := commandOutput("powershell", "-NoProfile", "-Command", "(Get-CimInstance Win32_Processor | Measure-Object -Property NumberOfCores -Sum).Sum")
	return parsePositiveInt(output)
}

func linuxPhysicalCoreCount() int {
	data, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return 0
	}
	physicalIDs := make(map[string]bool)
	currentPhysicalID := ""
	currentCoreID := ""
	cpuCores := 0
	flush := func() {
		if currentPhysicalID != "" && currentCoreID != "" {
			physicalIDs[currentPhysicalID+"/"+currentCoreID] = true
		}
		currentPhysicalID = ""
		currentCoreID = ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			flush()
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		switch strings.TrimSpace(key) {
		case "physical id":
			currentPhysicalID = strings.TrimSpace(value)
		case "core id":
			currentCoreID = strings.TrimSpace(value)
		case "cpu cores":
			if parsed := parsePositiveInt(value); parsed > cpuCores {
				cpuCores = parsed
			}
		}
	}
	flush()
	if len(physicalIDs) > 0 {
		return len(physicalIDs)
	}
	return cpuCores
}

func parsePositiveInt(value string) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || parsed <= 0 {
		return 0
	}
	return parsed
}

func windowsSystemDescription() string {
	product := windowsRegistryValue(`HKLM\SOFTWARE\Microsoft\Windows NT\CurrentVersion`, "ProductName")
	edition := windowsRegistryValue(`HKLM\SOFTWARE\Microsoft\Windows NT\CurrentVersion`, "EditionID")
	displayVersion := windowsRegistryValue(`HKLM\SOFTWARE\Microsoft\Windows NT\CurrentVersion`, "DisplayVersion")
	build := windowsRegistryValue(`HKLM\SOFTWARE\Microsoft\Windows NT\CurrentVersion`, "CurrentBuildNumber")
	if product == "" {
		product = "Windows"
	}
	if buildNumber, err := strconv.Atoi(build); err == nil && buildNumber >= 22000 {
		product = strings.Replace(product, "Windows 10", "Windows 11", 1)
	}
	parts := []string{product}
	if edition != "" && !strings.Contains(strings.ToLower(product), strings.ToLower(edition)) {
		parts = append(parts, "("+edition+")")
	}
	if displayVersion != "" {
		parts = append(parts, displayVersion)
	}
	if build != "" {
		parts = append(parts, "Build "+build)
	}
	return strings.Join(parts, " ")
}

func linuxSystemDescription() string {
	info := parseKeyValueFile("/etc/os-release")
	name := firstNonEmpty(info["NAME"], runtime.GOOS)
	id := info["ID"]
	version := info["VERSION_ID"]
	if id == "debian" {
		if debianVersion := readTrimmedFile("/etc/debian_version"); debianVersion != "" {
			version = debianVersion
		}
	}
	if version == "" {
		version = info["VERSION"]
	}
	if id != "" && version != "" {
		return fmt.Sprintf("%s(%s) %s", name, id, version)
	}
	if version != "" {
		return name + " " + version
	}
	return name
}

func darwinSystemDescription() string {
	version := commandOutput("sw_vers", "-productVersion")
	build := commandOutput("sw_vers", "-buildVersion")
	if version == "" {
		return "macOS"
	}
	if build != "" {
		return fmt.Sprintf("macOS %s Build %s", version, build)
	}
	return "macOS " + version
}

func linuxCPUModel() string {
	data, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		if key, value, ok := strings.Cut(line, ":"); ok && strings.TrimSpace(key) == "model name" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func windowsRegistryValue(path string, name string) string {
	output := commandOutput("cmd", "/C", "reg", "query", path, "/v", name)
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 3 && strings.EqualFold(fields[0], name) {
			return strings.Join(fields[2:], " ")
		}
	}
	return ""
}

func commandOutput(name string, args ...string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, name, args...).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

func parseKeyValueFile(path string) map[string]string {
	data, err := os.ReadFile(path)
	if err != nil {
		return map[string]string{}
	}
	result := make(map[string]string)
	for _, line := range strings.Split(string(data), "\n") {
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		value = strings.Trim(strings.TrimSpace(value), `"`)
		result[strings.TrimSpace(key)] = value
	}
	return result
}

func readTrimmedFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func firstNonEmpty(items ...string) string {
	for _, item := range items {
		if strings.TrimSpace(item) != "" {
			return strings.TrimSpace(item)
		}
	}
	return ""
}

func allBotMemoryUsage() string {
	var stat runtime.MemStats
	runtime.ReadMemStats(&stat)
	return formatUsageWithPercent(stat.Sys, totalMemoryBytes())
}

func allBotDiskUsage() string {
	root, err := os.Getwd()
	if err != nil {
		return "未知"
	}
	size, err := directorySize(root)
	if err != nil {
		return "未知"
	}
	total, _ := diskSpaceBytes(root)
	return formatUsageWithPercent(uint64(size), total)
}

func directorySize(root string) (int64, error) {
	var total int64
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		total += info.Size()
		return nil
	})
	return total, err
}

func formatReplyDuration(d time.Duration) string {
	hours, minutes, seconds := int(d.Hours()), int(d.Minutes())%60, int(d.Seconds())%60
	if hours > 0 {
		return fmt.Sprintf("%dh %dm %ds", hours, minutes, seconds)
	}
	if minutes > 0 {
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	}
	return fmt.Sprintf("%ds", seconds)
}

func formatUsageWithPercent(used uint64, total uint64) string {
	if total == 0 {
		return formatBytes(used)
	}
	return fmt.Sprintf("%s(%.2f%%)", formatBytes(used), float64(used)/float64(total)*100)
}

func formatBytes(value uint64) string {
	const unit = 1024
	if value < unit {
		return fmt.Sprintf("%dB", value)
	}
	units := []string{"KB", "MB", "GB", "TB"}
	amount := float64(value)
	for _, name := range units {
		amount /= unit
		if amount < unit {
			return fmt.Sprintf("%.1f%s", amount, name)
		}
	}
	return fmt.Sprintf("%.1fPB", amount/unit)
}

func bytesToGB(value uint64) float64 {
	return float64(value) / 1024 / 1024 / 1024
}
