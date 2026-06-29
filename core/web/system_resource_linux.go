//go:build linux

package web

import (
	"os"
	"strconv"
	"strings"
)

func readCPUTimes() (rawCPUTimes, bool) {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return rawCPUTimes{}, false
	}
	line := strings.SplitN(string(data), "\n", 2)[0]
	fields := strings.Fields(line)
	if len(fields) < 5 || fields[0] != "cpu" {
		return rawCPUTimes{}, false
	}
	var values []uint64
	for _, field := range fields[1:] {
		value, err := strconv.ParseUint(field, 10, 64)
		if err != nil {
			return rawCPUTimes{}, false
		}
		values = append(values, value)
	}
	var total uint64
	for _, value := range values {
		total += value
	}
	idle := values[3]
	if len(values) > 4 {
		idle += values[4]
	}
	return rawCPUTimes{idle: idle, total: total}, true
}

func readProcessCPUTimes() (rawProcessCPUTimes, bool) {
	data, err := os.ReadFile("/proc/self/stat")
	if err != nil {
		return rawProcessCPUTimes{}, false
	}
	fields := strings.Fields(string(data))
	if len(fields) < 15 {
		return rawProcessCPUTimes{}, false
	}
	userTime, err := strconv.ParseUint(fields[13], 10, 64)
	if err != nil {
		return rawProcessCPUTimes{}, false
	}
	kernelTime, err := strconv.ParseUint(fields[14], 10, 64)
	if err != nil {
		return rawProcessCPUTimes{}, false
	}
	return rawProcessCPUTimes{total: userTime + kernelTime}, true
}

func readMemoryStatus() rawMemoryStatus {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return rawMemoryStatus{}
	}
	values := map[string]uint64{}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		value, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			continue
		}
		values[strings.TrimSuffix(fields[0], ":")] = value * 1024
	}
	total := values["MemTotal"]
	available := values["MemAvailable"]
	if total == 0 || available > total {
		return rawMemoryStatus{}
	}
	return rawMemoryStatus{used: total - available, total: total}
}
