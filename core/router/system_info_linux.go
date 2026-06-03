package router

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
)

func memoryInfo() string {
	total, available := memoryBytes()
	if total == 0 || available > total {
		return "未知"
	}
	used := total - available
	percent := float64(used) / float64(total) * 100
	return fmt.Sprintf("%.1fG/%.1fG(%.2f%%)", bytesToGB(used), bytesToGB(total), percent)
}

func totalMemoryBytes() uint64 {
	total, _ := memoryBytes()
	return total
}

func memoryBytes() (uint64, uint64) {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, 0
	}
	defer file.Close()

	values := make(map[string]uint64)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}
		value, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			continue
		}
		values[strings.TrimSuffix(fields[0], ":")] = value * 1024
	}
	return values["MemTotal"], values["MemAvailable"]
}

func diskInfo(path string) string {
	total, free := diskSpaceBytes(path)
	if total == 0 || free > total {
		return "未知"
	}
	used := total - free
	percent := float64(used) / float64(total) * 100
	return fmt.Sprintf("%.1fG/%.1fG(%.2f%%)", bytesToGB(used), bytesToGB(total), percent)
}

func diskSpaceBytes(path string) (uint64, uint64) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil || stat.Blocks == 0 {
		return 0, 0
	}
	return stat.Blocks * uint64(stat.Bsize), stat.Bavail * uint64(stat.Bsize)
}
