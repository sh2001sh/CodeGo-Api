package app

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	platformcache "github.com/sh2001sh/CodeGo-Api/internal/platform/cache"
	platformconfig "github.com/sh2001sh/CodeGo-Api/internal/platform/config"
	platformhostinfo "github.com/sh2001sh/CodeGo-Api/internal/platform/hostinfo"
	"github.com/sh2001sh/CodeGo-Api/internal/platform/logger"
	platformobservability "github.com/sh2001sh/CodeGo-Api/internal/platform/observability"
)

type PerformanceStats struct {
	CacheStats    platformcache.DiskCacheStats        `json:"cache_stats"`
	MemoryStats   MemoryStats                         `json:"memory_stats"`
	DiskCacheInfo DiskCacheInfo                       `json:"disk_cache_info"`
	DiskSpaceInfo platformobservability.DiskSpaceInfo `json:"disk_space_info"`
	Config        PerformanceConfig                   `json:"config"`
}

type MemoryStats struct {
	Alloc        uint64 `json:"alloc"`
	TotalAlloc   uint64 `json:"total_alloc"`
	Sys          uint64 `json:"sys"`
	NumGC        uint32 `json:"num_gc"`
	NumGoroutine int    `json:"num_goroutine"`
}

type DiskCacheInfo struct {
	Path      string `json:"path"`
	Exists    bool   `json:"exists"`
	FileCount int    `json:"file_count"`
	TotalSize int64  `json:"total_size"`
}

type PerformanceConfig struct {
	DiskCacheEnabled       bool   `json:"disk_cache_enabled"`
	DiskCacheThresholdMB   int    `json:"disk_cache_threshold_mb"`
	DiskCacheMaxSizeMB     int    `json:"disk_cache_max_size_mb"`
	DiskCachePath          string `json:"disk_cache_path"`
	IsRunningInContainer   bool   `json:"is_running_in_container"`
	MonitorEnabled         bool   `json:"monitor_enabled"`
	MonitorCPUThreshold    int    `json:"monitor_cpu_threshold"`
	MonitorMemoryThreshold int    `json:"monitor_memory_threshold"`
	MonitorDiskThreshold   int    `json:"monitor_disk_threshold"`
}

type LogFileInfo struct {
	Name    string    `json:"name"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"mod_time"`
}

type LogFilesResponse struct {
	LogDir     string        `json:"log_dir"`
	Enabled    bool          `json:"enabled"`
	FileCount  int           `json:"file_count"`
	TotalSize  int64         `json:"total_size"`
	OldestTime *time.Time    `json:"oldest_time,omitempty"`
	NewestTime *time.Time    `json:"newest_time,omitempty"`
	Files      []LogFileInfo `json:"files"`
}

type LogCleanupResult struct {
	DeletedCount int      `json:"deleted_count"`
	FreedBytes   int64    `json:"freed_bytes"`
	FailedFiles  []string `json:"failed_files"`
}

func BuildPerformanceStats() PerformanceStats {
	cacheStats := platformcache.GetDiskCacheStats()

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	diskConfig := platformcache.GetDiskCacheConfig()
	monitorConfig := platformobservability.GetPerformanceMonitorConfig()

	return PerformanceStats{
		CacheStats: cacheStats,
		MemoryStats: MemoryStats{
			Alloc:        memStats.Alloc,
			TotalAlloc:   memStats.TotalAlloc,
			Sys:          memStats.Sys,
			NumGC:        memStats.NumGC,
			NumGoroutine: runtime.NumGoroutine(),
		},
		DiskCacheInfo: getDiskCacheInfo(),
		DiskSpaceInfo: platformobservability.GetDiskSpaceInfo(),
		Config: PerformanceConfig{
			DiskCacheEnabled:       diskConfig.Enabled,
			DiskCacheThresholdMB:   diskConfig.ThresholdMB,
			DiskCacheMaxSizeMB:     diskConfig.MaxSizeMB,
			DiskCachePath:          diskConfig.Path,
			IsRunningInContainer:   platformhostinfo.IsRunningInContainer(),
			MonitorEnabled:         monitorConfig.Enabled,
			MonitorCPUThreshold:    monitorConfig.CPUThreshold,
			MonitorMemoryThreshold: monitorConfig.MemoryThreshold,
			MonitorDiskThreshold:   monitorConfig.DiskThreshold,
		},
	}
}

func CleanupInactiveDiskCache() error {
	return platformcache.CleanupOldDiskCacheFiles(10 * time.Minute)
}

func ResetPerformanceStats() {
	platformcache.ResetDiskCacheStats()
}

func ForceGC() {
	runtime.GC()
}

func BuildLogFilesResponse() (LogFilesResponse, error) {
	if *platformconfig.LogDir == "" {
		return LogFilesResponse{Enabled: false}, nil
	}

	files, err := getLogFiles()
	if err != nil {
		return LogFilesResponse{}, err
	}

	var totalSize int64
	var oldest time.Time
	var newest time.Time
	for index, file := range files {
		totalSize += file.Size
		if index == 0 || file.ModTime.Before(oldest) {
			oldest = file.ModTime
		}
		if index == 0 || file.ModTime.After(newest) {
			newest = file.ModTime
		}
	}

	response := LogFilesResponse{
		LogDir:    *platformconfig.LogDir,
		Enabled:   true,
		FileCount: len(files),
		TotalSize: totalSize,
		Files:     files,
	}
	if len(files) > 0 {
		response.OldestTime = &oldest
		response.NewestTime = &newest
	}
	return response, nil
}

func CleanupLogFiles(mode string, value int) (LogCleanupResult, bool, error) {
	if mode != "by_count" && mode != "by_days" {
		return LogCleanupResult{}, false, fmt.Errorf("invalid mode, must be by_count or by_days")
	}
	if value < 1 {
		return LogCleanupResult{}, false, fmt.Errorf("invalid value, must be a positive integer")
	}
	if *platformconfig.LogDir == "" {
		return LogCleanupResult{}, false, fmt.Errorf("log directory not configured")
	}

	files, err := getLogFiles()
	if err != nil {
		return LogCleanupResult{}, false, err
	}

	activeLogPath := logger.GetCurrentLogPath()
	toDelete := make([]LogFileInfo, 0)

	switch mode {
	case "by_count":
		for index, file := range files {
			if index < value {
				continue
			}
			fullPath := filepath.Join(*platformconfig.LogDir, file.Name)
			if fullPath == activeLogPath {
				continue
			}
			toDelete = append(toDelete, file)
		}
	case "by_days":
		cutoff := time.Now().AddDate(0, 0, -value)
		for _, file := range files {
			if !file.ModTime.Before(cutoff) {
				continue
			}
			fullPath := filepath.Join(*platformconfig.LogDir, file.Name)
			if fullPath == activeLogPath {
				continue
			}
			toDelete = append(toDelete, file)
		}
	}

	result := LogCleanupResult{
		FailedFiles: make([]string, 0),
	}
	for _, file := range toDelete {
		fullPath := filepath.Join(*platformconfig.LogDir, file.Name)
		if err := os.Remove(fullPath); err != nil {
			result.FailedFiles = append(result.FailedFiles, file.Name)
			continue
		}
		result.DeletedCount++
		result.FreedBytes += file.Size
	}

	return result, len(result.FailedFiles) > 0, nil
}

func getLogFiles() ([]LogFileInfo, error) {
	if *platformconfig.LogDir == "" {
		return nil, nil
	}
	entries, err := os.ReadDir(*platformconfig.LogDir)
	if err != nil {
		return nil, err
	}

	files := make([]LogFileInfo, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, "oneapi-") || !strings.HasSuffix(name, ".log") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		files = append(files, LogFileInfo{
			Name:    name,
			Size:    info.Size(),
			ModTime: info.ModTime(),
		})
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].Name > files[j].Name
	})
	return files, nil
}

func getDiskCacheInfo() DiskCacheInfo {
	dir := platformcache.GetDiskCacheDir()
	info := DiskCacheInfo{
		Path:   dir,
		Exists: false,
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return info
	}

	info.Exists = true
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info.FileCount++
		if fileInfo, err := entry.Info(); err == nil {
			info.TotalSize += fileInfo.Size()
		}
	}

	return info
}
