package app

import (
	"context"
	"fmt"
	"github.com/bytedance/gopkg/util/gopool"
	platformconfig "github.com/sh2001sh/CodeGo-Api/internal/platform/config"
	"github.com/sh2001sh/CodeGo-Api/internal/platform/logger"
	platformruntime "github.com/sh2001sh/CodeGo-Api/internal/platform/runtime"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

const (
	defaultImageWorkspaceRetentionHours = 72
	imageWorkspaceCleanupTick           = 30 * time.Minute
	imageWorkspaceCleanupBatch          = 100
	defaultImageWorkspaceDir            = "data/generated_images"
)

var (
	imageWorkspaceCleanupOnce    sync.Once
	imageWorkspaceCleanupRunning atomic.Bool
)

func GetImageWorkspaceDir() string {
	return platformconfig.GetEnvOrDefaultString("IMAGE_WORKSPACE_DIR", defaultImageWorkspaceDir)
}

func GetImageWorkspaceRetentionHours() int {
	retentionHours := platformconfig.GetEnvOrDefaultInt("IMAGE_WORKSPACE_RETENTION_HOURS", defaultImageWorkspaceRetentionHours)
	if retentionHours <= 0 {
		return defaultImageWorkspaceRetentionHours
	}
	return retentionHours
}

func GetImageWorkspaceRetentionSeconds() int64 {
	return int64(GetImageWorkspaceRetentionHours()) * 3600
}

func EnsureImageWorkspaceDir() error {
	return os.MkdirAll(GetImageWorkspaceDir(), 0755)
}

func StartImageWorkspaceCleanupTask() {
	imageWorkspaceCleanupOnce.Do(func() {
		if !platformconfig.IsMasterNode {
			return
		}
		gopool.Go(func() {
			logger.LogInfo(context.Background(), fmt.Sprintf("image workspace cleanup task started: tick=%s retention=%dh", imageWorkspaceCleanupTick, GetImageWorkspaceRetentionHours()))
			runImageWorkspaceCleanupOnce()
			ticker := time.NewTicker(imageWorkspaceCleanupTick)
			defer ticker.Stop()
			for range ticker.C {
				runImageWorkspaceCleanupOnce()
			}
		})
	})
}

func runImageWorkspaceCleanupOnce() {
	if !imageWorkspaceCleanupRunning.CompareAndSwap(false, true) {
		return
	}
	defer imageWorkspaceCleanupRunning.Store(false)

	total := 0
	for {
		n, err := cleanupExpiredImageWorkspaceItems(imageWorkspaceCleanupBatch, platformruntime.GetTimestamp())
		if err != nil {
			logger.LogWarn(context.Background(), fmt.Sprintf("image workspace cleanup failed: %v", err))
			return
		}
		total += n
		if n < imageWorkspaceCleanupBatch {
			break
		}
	}
	if platformconfig.DebugEnabled && total > 0 {
		logger.LogDebug(context.Background(), "image workspace cleanup removed=%d", total)
	}
}
