//go:build windows

package web

import (
	"syscall"
	"unsafe"
)

func readCPUTimes() (rawCPUTimes, bool) {
	var idle, kernel, user filetime
	ret, _, _ := getSystemTimes.Call(
		uintptr(unsafe.Pointer(&idle)),
		uintptr(unsafe.Pointer(&kernel)),
		uintptr(unsafe.Pointer(&user)),
	)
	if ret == 0 {
		return rawCPUTimes{}, false
	}
	idleValue := idle.uint64()
	return rawCPUTimes{idle: idleValue, total: kernel.uint64() + user.uint64()}, true
}

func readMemoryStatus() rawMemoryStatus {
	var state memoryStatusEx
	state.Length = uint32(unsafe.Sizeof(state))
	ret, _, _ := globalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&state)))
	if ret == 0 || state.TotalPhys == 0 || state.AvailPhys > state.TotalPhys {
		return rawMemoryStatus{}
	}
	return rawMemoryStatus{used: state.TotalPhys - state.AvailPhys, total: state.TotalPhys}
}

func readProcessCPUTimes() (rawProcessCPUTimes, bool) {
	var creation, exit, kernelTime, userTime filetime
	handle, err := syscall.GetCurrentProcess()
	if err != nil {
		return rawProcessCPUTimes{}, false
	}
	ret, _, _ := getProcessTimes.Call(
		uintptr(handle),
		uintptr(unsafe.Pointer(&creation)),
		uintptr(unsafe.Pointer(&exit)),
		uintptr(unsafe.Pointer(&kernelTime)),
		uintptr(unsafe.Pointer(&userTime)),
	)
	if ret == 0 {
		return rawProcessCPUTimes{}, false
	}
	return rawProcessCPUTimes{total: kernelTime.uint64() + userTime.uint64()}, true
}

type filetime struct {
	LowDateTime  uint32
	HighDateTime uint32
}

func (f filetime) uint64() uint64 {
	return uint64(f.HighDateTime)<<32 | uint64(f.LowDateTime)
}

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

var (
	kernel32             = syscall.NewLazyDLL("kernel32.dll")
	getSystemTimes       = kernel32.NewProc("GetSystemTimes")
	getProcessTimes      = kernel32.NewProc("GetProcessTimes")
	globalMemoryStatusEx = kernel32.NewProc("GlobalMemoryStatusEx")
)
