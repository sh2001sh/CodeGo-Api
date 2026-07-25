package observability

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/mem"
)

// DiskSpaceInfo reports disk usage for the active cache volume.
type DiskSpaceInfo struct {
	Total       uint64  `json:"total"`
	Free        uint64  `json:"free"`
	Used        uint64  `json:"used"`
	UsedPercent float64 `json:"used_percent"`
}

// SystemStatus holds the latest sampled system usage values.
type SystemStatus struct {
	CPUUsage    float64
	MemoryUsage float64
	DiskUsage   float64
}

var (
	latestSystemStatus atomic.Value
	systemMonitorOnce  sync.Once
)

func init() {
	latestSystemStatus.Store(SystemStatus{})
}

// StartSystemMonitor launches the background runtime sampler once for the process.
func StartSystemMonitor() {
	systemMonitorOnce.Do(func() {
		go func() {
			for {
				config := GetPerformanceMonitorConfig()
				if !config.Enabled {
					time.Sleep(30 * time.Second)
					continue
				}

				updateSystemStatus()
				time.Sleep(5 * time.Second)
			}
		}()
	})
}

func updateSystemStatus() {
	var status SystemStatus

	percents, err := cpu.Percent(0, false)
	if err == nil && len(percents) > 0 {
		status.CPUUsage = percents[0]
	}

	memInfo, err := mem.VirtualMemory()
	if err == nil {
		status.MemoryUsage = memInfo.UsedPercent
	}

	diskInfo := GetDiskSpaceInfo()
	if diskInfo.Total > 0 {
		status.DiskUsage = diskInfo.UsedPercent
	}

	latestSystemStatus.Store(status)
}

// GetSystemStatus returns the latest sampled system usage snapshot.
func GetSystemStatus() SystemStatus {
	return latestSystemStatus.Load().(SystemStatus)
}
