package store

import (
	platformcache "github.com/sh2001sh/CodeGo-Api/internal/platform/cache"
	platformobservability "github.com/sh2001sh/CodeGo-Api/internal/platform/observability"
	"github.com/sh2001sh/CodeGo-Api/setting/config"
)

type performanceSetting struct {
	DiskCacheEnabled       bool   `json:"disk_cache_enabled"`
	DiskCacheThresholdMB   int    `json:"disk_cache_threshold_mb"`
	DiskCacheMaxSizeMB     int    `json:"disk_cache_max_size_mb"`
	DiskCachePath          string `json:"disk_cache_path"`
	MonitorEnabled         bool   `json:"monitor_enabled"`
	MonitorCPUThreshold    int    `json:"monitor_cpu_threshold"`
	MonitorMemoryThreshold int    `json:"monitor_memory_threshold"`
	MonitorDiskThreshold   int    `json:"monitor_disk_threshold"`
}

var platformPerformanceSetting = performanceSetting{
	DiskCacheEnabled:       false,
	DiskCacheThresholdMB:   10,
	DiskCacheMaxSizeMB:     1024,
	DiskCachePath:          "",
	MonitorEnabled:         true,
	MonitorCPUThreshold:    90,
	MonitorMemoryThreshold: 90,
	MonitorDiskThreshold:   95,
}

func init() {
	config.GlobalConfig.Register("performance_setting", &platformPerformanceSetting)
	updatePerformanceSettingAndSync()
}

func updatePerformanceSettingAndSync() {
	platformcache.SetDiskCacheConfig(platformcache.DiskCacheConfig{
		Enabled:     platformPerformanceSetting.DiskCacheEnabled,
		ThresholdMB: platformPerformanceSetting.DiskCacheThresholdMB,
		MaxSizeMB:   platformPerformanceSetting.DiskCacheMaxSizeMB,
		Path:        platformPerformanceSetting.DiskCachePath,
	})

	platformobservability.SetPerformanceMonitorConfig(platformobservability.PerformanceMonitorConfig{
		Enabled:         platformPerformanceSetting.MonitorEnabled,
		CPUThreshold:    platformPerformanceSetting.MonitorCPUThreshold,
		MemoryThreshold: platformPerformanceSetting.MonitorMemoryThreshold,
		DiskThreshold:   platformPerformanceSetting.MonitorDiskThreshold,
	})
}
