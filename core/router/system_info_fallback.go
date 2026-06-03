//go:build !windows && !linux

package router

func memoryInfo() string {
	return "未知"
}

func diskInfo(path string) string {
	return "未知"
}

func totalMemoryBytes() uint64 {
	return 0
}

func diskSpaceBytes(path string) (uint64, uint64) {
	return 0, 0
}
