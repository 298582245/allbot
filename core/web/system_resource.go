package web

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
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

const (
	historicalResourcePeaksKey         = "system.resource.allbot_historical_peaks"
	historicalResourcePeaksDescription = "AllBot 永久资源峰值"
)

type allBotHistoricalResourcePeaks struct {
	CPUUsagePercent    float64 `json:"cpuUsagePercent"`
	MemoryUsedBytes    uint64  `json:"memoryUsedBytes"`
	MemoryUsagePercent float64 `json:"memoryUsagePercent"`
}

func (s *Server) systemResourceStatus() map[string]interface{} {
	memory := readMemoryStatus()
	memoryPercent := percentOf(memory.used, memory.total)
	allBotCPUPercent := s.sampleAllBotCPUUsage()
	allBotMemoryUsed := readAllBotMemoryUsed()
	allBotMemoryPercent := percentOf(allBotMemoryUsed, memory.total)
	allBotResourceStats := s.updateAllBotResourceStats(allBotCPUPercent, allBotMemoryUsed, allBotMemoryPercent)
	allBotHistoricalPeaks := s.updateHistoricalResourcePeaks(allBotHistoricalResourcePeaks{
		CPUUsagePercent:    allBotResourceStats.peakCPUPercent,
		MemoryUsedBytes:    allBotResourceStats.peakMemoryUsed,
		MemoryUsagePercent: allBotResourceStats.peakMemoryPercent,
	})
	return map[string]interface{}{
		"cpuUsagePercent":                        roundPercent(s.sampleCPUUsage()),
		"memoryUsedBytes":                        memory.used,
		"memoryTotalBytes":                       memory.total,
		"memoryUsagePercent":                     roundPercent(memoryPercent),
		"allBotCpuUsagePercent":                  roundPercent(allBotCPUPercent),
		"allBotMemoryUsedBytes":                  allBotMemoryUsed,
		"allBotMemoryTotalBytes":                 memory.total,
		"allBotMemoryUsagePercent":               roundPercent(allBotMemoryPercent),
		"allBotPeakCpuUsagePercent":              roundPercent(allBotResourceStats.peakCPUPercent),
		"allBotPeakMemoryUsedBytes":              allBotResourceStats.peakMemoryUsed,
		"allBotPeakMemoryUsagePercent":           roundPercent(allBotResourceStats.peakMemoryPercent),
		"allBotHistoricalPeakCpuUsagePercent":    roundPercent(allBotHistoricalPeaks.CPUUsagePercent),
		"allBotHistoricalPeakMemoryUsedBytes":    allBotHistoricalPeaks.MemoryUsedBytes,
		"allBotHistoricalPeakMemoryUsagePercent": roundPercent(allBotHistoricalPeaks.MemoryUsagePercent),
		"allBotAverageCpuUsagePercent":           roundPercent(allBotResourceStats.averageCPUPercent),
		"allBotAverageMemoryUsedBytes":           allBotResourceStats.averageMemoryUsed,
		"allBotAverageMemoryUsagePercent":        roundPercent(allBotResourceStats.averageMemoryPercent),
	}
}

type allBotResourceStats struct {
	peakCPUPercent       float64
	peakMemoryUsed       uint64
	peakMemoryPercent    float64
	averageCPUPercent    float64
	averageMemoryUsed    uint64
	averageMemoryPercent float64
}

func (s *Server) updateAllBotResourceStats(cpuPercent float64, memoryUsed uint64, memoryPercent float64) allBotResourceStats {
	s.resourceMu.Lock()
	defer s.resourceMu.Unlock()
	if isFinite(cpuPercent) && cpuPercent > s.peakAllBotCPUPercent {
		s.peakAllBotCPUPercent = cpuPercent
	}
	if memoryUsed > s.peakAllBotMemoryUsedBytes {
		s.peakAllBotMemoryUsedBytes = memoryUsed
	}
	if isFinite(memoryPercent) && memoryPercent > s.peakAllBotMemoryUsagePercent {
		s.peakAllBotMemoryUsagePercent = memoryPercent
	}
	s.allBotResourceSampleCount++
	if isFinite(cpuPercent) {
		s.allBotCPUTotalPercent += cpuPercent
	}
	s.allBotMemoryTotalUsedBytes += memoryUsed
	if isFinite(memoryPercent) {
		s.allBotMemoryTotalPercent += memoryPercent
	}
	return allBotResourceStats{
		peakCPUPercent:       s.peakAllBotCPUPercent,
		peakMemoryUsed:       s.peakAllBotMemoryUsedBytes,
		peakMemoryPercent:    s.peakAllBotMemoryUsagePercent,
		averageCPUPercent:    s.allBotCPUTotalPercent / float64(s.allBotResourceSampleCount),
		averageMemoryUsed:    s.allBotMemoryTotalUsedBytes / s.allBotResourceSampleCount,
		averageMemoryPercent: s.allBotMemoryTotalPercent / float64(s.allBotResourceSampleCount),
	}
}

func (s *Server) updateHistoricalResourcePeaks(candidate allBotHistoricalResourcePeaks) allBotHistoricalResourcePeaks {
	s.historicalResourceMu.Lock()
	defer s.historicalResourceMu.Unlock()

	s.historicalResourcePeaks = mergeHistoricalResourcePeaks(s.historicalResourcePeaks, candidate)
	if err := s.ensureHistoricalResourcePeaksLoadedLocked(); err != nil {
		return s.historicalResourcePeaks
	}
	_ = s.persistHistoricalResourcePeaksLocked()
	return s.historicalResourcePeaks
}

func (s *Server) persistPendingHistoricalResourcePeaks() error {
	s.historicalResourceMu.Lock()
	defer s.historicalResourceMu.Unlock()

	if err := s.ensureHistoricalResourcePeaksLoadedLocked(); err != nil {
		return err
	}
	return s.persistHistoricalResourcePeaksLocked()
}

func (s *Server) ensureHistoricalResourcePeaksLoadedLocked() error {
	if s.historicalResourceLoaded {
		return nil
	}
	persisted, err := s.loadHistoricalResourcePeaks()
	if err != nil {
		return err
	}
	s.persistedHistoricalPeaks = persisted
	s.historicalResourcePeaks = mergeHistoricalResourcePeaks(s.historicalResourcePeaks, persisted)
	s.historicalResourceLoaded = true
	return nil
}

func (s *Server) loadHistoricalResourcePeaks() (allBotHistoricalResourcePeaks, error) {
	var value string
	var err error
	if s.historicalResourceGetSetting != nil {
		value, err = s.historicalResourceGetSetting()
	} else {
		database := s.runtimeDatabase()
		if database == nil {
			return allBotHistoricalResourcePeaks{}, fmt.Errorf("数据库未初始化")
		}
		value, err = database.GetSetting(historicalResourcePeaksKey)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return allBotHistoricalResourcePeaks{}, nil
	}
	if err != nil {
		return allBotHistoricalResourcePeaks{}, err
	}
	var peaks allBotHistoricalResourcePeaks
	if err := json.Unmarshal([]byte(value), &peaks); err != nil {
		return allBotHistoricalResourcePeaks{}, err
	}
	return normalizeHistoricalResourcePeaks(peaks), nil
}

func (s *Server) persistHistoricalResourcePeaksLocked() error {
	if !historicalResourcePeaksGreater(s.historicalResourcePeaks, s.persistedHistoricalPeaks) {
		return nil
	}
	data, err := json.Marshal(s.historicalResourcePeaks)
	if err != nil {
		return err
	}
	if s.historicalResourceSetSetting != nil {
		err = s.historicalResourceSetSetting(string(data))
	} else {
		database := s.runtimeDatabase()
		if database == nil {
			return fmt.Errorf("数据库未初始化")
		}
		err = database.SetSetting(historicalResourcePeaksKey, string(data), historicalResourcePeaksDescription)
	}
	if err != nil {
		return err
	}
	s.persistedHistoricalPeaks = s.historicalResourcePeaks
	return nil
}

func mergeHistoricalResourcePeaks(current, candidate allBotHistoricalResourcePeaks) allBotHistoricalResourcePeaks {
	candidate = normalizeHistoricalResourcePeaks(candidate)
	if candidate.CPUUsagePercent > current.CPUUsagePercent {
		current.CPUUsagePercent = candidate.CPUUsagePercent
	}
	if candidate.MemoryUsedBytes > current.MemoryUsedBytes {
		current.MemoryUsedBytes = candidate.MemoryUsedBytes
	}
	if candidate.MemoryUsagePercent > current.MemoryUsagePercent {
		current.MemoryUsagePercent = candidate.MemoryUsagePercent
	}
	return current
}

func normalizeHistoricalResourcePeaks(peaks allBotHistoricalResourcePeaks) allBotHistoricalResourcePeaks {
	peaks.CPUUsagePercent = clampResourcePercent(peaks.CPUUsagePercent)
	peaks.MemoryUsagePercent = clampResourcePercent(peaks.MemoryUsagePercent)
	return peaks
}

func historicalResourcePeaksGreater(candidate, persisted allBotHistoricalResourcePeaks) bool {
	return candidate.CPUUsagePercent > persisted.CPUUsagePercent ||
		candidate.MemoryUsedBytes > persisted.MemoryUsedBytes ||
		candidate.MemoryUsagePercent > persisted.MemoryUsagePercent
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
	return math.Round(clampResourcePercent(value)*100) / 100
}

func clampResourcePercent(value float64) float64 {
	if !isFinite(value) || value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return value
}

func isFinite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}
