//go:build !windows && !linux

package web

func readCPUTimes() (rawCPUTimes, bool) {
	return rawCPUTimes{}, false
}

func readMemoryStatus() rawMemoryStatus {
	return rawMemoryStatus{}
}

func readProcessCPUTimes() (rawProcessCPUTimes, bool) {
	return rawProcessCPUTimes{}, false
}
