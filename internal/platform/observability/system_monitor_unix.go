//go:build !windows

package observability

import (
	"os"

	platformcache "github.com/sh2001sh/CodeGo-Api/internal/platform/cache"
	"golang.org/x/sys/unix"
)

// GetDiskSpaceInfo returns disk usage for the cache volume on Unix-like systems.
func GetDiskSpaceInfo() DiskSpaceInfo {
	cachePath := platformcache.GetDiskCachePath()
	if cachePath == "" {
		cachePath = os.TempDir()
	}

	info := DiskSpaceInfo{}
	var stat unix.Statfs_t
	if err := unix.Statfs(cachePath, &stat); err != nil {
		return info
	}

	bsize := uint64(stat.Bsize)
	info.Total = uint64(stat.Blocks) * bsize
	info.Free = uint64(stat.Bavail) * bsize
	info.Used = info.Total - uint64(stat.Bfree)*bsize
	if info.Total > 0 {
		info.UsedPercent = float64(info.Used) / float64(info.Total) * 100
	}
	return info
}
