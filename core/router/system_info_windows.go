package router

import (
	"fmt"
	"syscall"
	"unsafe"
)

func memoryInfo() string {
	total := totalMemoryBytes()
	if total == 0 {
		return "未知"
	}
	var state memoryStatusEx
	state.Length = uint32(unsafe.Sizeof(state))
	ret, _, _ := globalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&state)))
	if ret == 0 {
		return "未知"
	}
	used := total - state.AvailPhys
	percent := float64(used) / float64(total) * 100
	return fmt.Sprintf("%.1fG/%.1fG(%.2f%%)", bytesToGB(used), bytesToGB(total), percent)
}

func totalMemoryBytes() uint64 {
	var state memoryStatusEx
	state.Length = uint32(unsafe.Sizeof(state))
	ret, _, _ := globalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&state)))
	if ret == 0 {
		return 0
	}
	return state.TotalPhys
}

func diskInfo(path string) string {
	totalBytes, totalFreeBytes := diskSpaceBytes(path)
	if totalBytes == 0 {
		return "未知"
	}
	used := totalBytes - totalFreeBytes
	percent := float64(used) / float64(totalBytes) * 100
	return fmt.Sprintf("%.1fG/%.1fG(%.2f%%)", bytesToGB(used), bytesToGB(totalBytes), percent)
}

func diskSpaceBytes(path string) (uint64, uint64) {
	var freeBytesAvailable, totalBytes, totalFreeBytes uint64
	pathPtr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return 0, 0
	}
	ret, _, _ := getDiskFreeSpaceEx.Call(uintptr(unsafe.Pointer(pathPtr)), uintptr(unsafe.Pointer(&freeBytesAvailable)), uintptr(unsafe.Pointer(&totalBytes)), uintptr(unsafe.Pointer(&totalFreeBytes)))
	if ret == 0 {
		return 0, 0
	}
	return totalBytes, totalFreeBytes
}

var (
	kernel32             = syscall.NewLazyDLL("kernel32.dll")
	globalMemoryStatusEx = kernel32.NewProc("GlobalMemoryStatusEx")
	getDiskFreeSpaceEx   = kernel32.NewProc("GetDiskFreeSpaceExW")
)

type memoryStatusEx struct {
	Length               uint32
	MemoryLoad           uint32
	TotalPhys            uint64
	AvailPhys            uint64
	TotalPageFile        uint64
	AvailPageFile        uint64
	TotalVirtual         uint64
	AvailVirtual         uint64
	AvailExtendedVirtual uint64
}
