package web

import (
	"math"
	"runtime"
	"time"
)

type rawCPUTimes struct {
	idle  uint64
	total uint64
}

type rawMemoryStatus struct {
	used  uint64
	total uint64
}

type rawProcessCPUTimes struct {
	total uint64
}

func (s *Server) systemResourceStatus() map[string]interface{} {
	memory := readMemoryStatus()
	memoryPercent := percentOf(memory.used, memory.total)
	allBotMemoryUsed := readAllBotMemoryUsed()
	allBotMemoryPercent := percentOf(allBotMemoryUsed, memory.total)
	return map[string]interface{}{
		"cpuUsagePercent":          roundPercent(s.sampleCPUUsage()),
		"memoryUsedBytes":          memory.used,
		"memoryTotalBytes":         memory.total,
		"memoryUsagePercent":       roundPercent(memoryPercent),
		"allBotCpuUsagePercent":    roundPercent(s.sampleAllBotCPUUsage()),
		"allBotMemoryUsedBytes":    allBotMemoryUsed,
		"allBotMemoryTotalBytes":   memory.total,
		"allBotMemoryUsagePercent": roundPercent(allBotMemoryPercent),
	}
}

func (s *Server) sampleCPUUsage() float64 {
	times, ok := readCPUTimes()
	if !ok || times.total == 0 {
		return 0
	}
	s.resourceMu.Lock()
	defer s.resourceMu.Unlock()
	if s.lastCPUTotal == 0 || times.total <= s.lastCPUTotal || times.idle < s.lastCPUIdle {
		s.lastCPUIdle = times.idle
		s.lastCPUTotal = times.total
		s.lastCPUAt = time.Now()
		return s.lastCPUPercent
	}
	idleDelta := times.idle - s.lastCPUIdle
	totalDelta := times.total - s.lastCPUTotal
	s.lastCPUIdle = times.idle
	s.lastCPUTotal = times.total
	s.lastCPUAt = time.Now()
	if totalDelta == 0 || idleDelta > totalDelta {
		return s.lastCPUPercent
	}
	s.lastCPUPercent = float64(totalDelta-idleDelta) / float64(totalDelta) * 100
	return s.lastCPUPercent
}

func (s *Server) sampleAllBotCPUUsage() float64 {
	systemTimes, systemOK := readCPUTimes()
	processTimes, processOK := readProcessCPUTimes()
	if !systemOK || !processOK || systemTimes.total == 0 || processTimes.total == 0 {
		return 0
	}
	s.resourceMu.Lock()
	defer s.resourceMu.Unlock()
	if s.lastProcessSystemCPUTotal == 0 || systemTimes.total <= s.lastProcessSystemCPUTotal || processTimes.total < s.lastProcessCPUTotal {
		s.lastProcessSystemCPUTotal = systemTimes.total
		s.lastProcessCPUTotal = processTimes.total
		return s.lastProcessCPUPercent
	}
	systemDelta := systemTimes.total - s.lastProcessSystemCPUTotal
	processDelta := processTimes.total - s.lastProcessCPUTotal
	s.lastProcessSystemCPUTotal = systemTimes.total
	s.lastProcessCPUTotal = processTimes.total
	if systemDelta == 0 {
		return s.lastProcessCPUPercent
	}
	s.lastProcessCPUPercent = float64(processDelta) / float64(systemDelta) * 100
	return s.lastProcessCPUPercent
}

func readAllBotMemoryUsed() uint64 {
	var stat runtime.MemStats
	runtime.ReadMemStats(&stat)
	return stat.Sys
}

func percentOf(used uint64, total uint64) float64 {
	if total == 0 || used > total {
		return 0
	}
	return float64(used) / float64(total) * 100
}

func roundPercent(value float64) float64 {
	if !isFinite(value) || value < 0 {
		return 0
	}
	if value > 100 {
		value = 100
	}
	return math.Round(value*100) / 100
}

func isFinite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}
