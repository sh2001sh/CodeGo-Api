package app

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bytedance/gopkg/util/gopool"
	platformconfig "github.com/sh2001sh/CodeGo-Api/internal/platform/config"
	"github.com/sh2001sh/CodeGo-Api/internal/platform/logger"
)

const (
	subscriptionMaintenanceTickInterval = 1 * time.Minute
	subscriptionMaintenanceBatchSize    = 300
	subscriptionCleanupInterval         = 30 * time.Minute
)

var (
	subscriptionMaintenanceOnce    sync.Once
	subscriptionMaintenanceRunning atomic.Bool
	subscriptionCleanupLast        atomic.Int64
)

// StartSubscriptionMaintenanceTask owns non-workflow subscription maintenance.
// Durable periodic quota resets are scheduled by workflow-worker through Temporal.
func StartSubscriptionMaintenanceTask() {
	subscriptionMaintenanceOnce.Do(func() {
		if !platformconfig.IsMasterNode {
			return
		}
		gopool.Go(func() {
			logger.LogInfo(context.Background(), fmt.Sprintf("subscription maintenance task started: tick=%s", subscriptionMaintenanceTickInterval))
			ticker := time.NewTicker(subscriptionMaintenanceTickInterval)
			defer ticker.Stop()

			runSubscriptionMaintenanceOnce()
			for range ticker.C {
				runSubscriptionMaintenanceOnce()
			}
		})
	})
}

func runSubscriptionMaintenanceOnce() {
	if !subscriptionMaintenanceRunning.CompareAndSwap(false, true) {
		return
	}
	defer subscriptionMaintenanceRunning.Store(false)

	ctx := context.Background()
	totalExpired := 0
	for {
		n, err := ExpireDueSubscriptions(subscriptionMaintenanceBatchSize)
		if err != nil {
			logger.LogWarn(ctx, fmt.Sprintf("subscription expire task failed: %v", err))
			return
		}
		if n == 0 {
			break
		}
		totalExpired += n
		if n < subscriptionMaintenanceBatchSize {
			break
		}
	}
	lastCleanup := time.Unix(subscriptionCleanupLast.Load(), 0)
	if time.Since(lastCleanup) >= subscriptionCleanupInterval {
		if _, err := CleanupSubscriptionPreConsumeRecords(7 * 24 * 3600); err == nil {
			subscriptionCleanupLast.Store(time.Now().Unix())
		}
	}
	if platformconfig.DebugEnabled && totalExpired > 0 {
		logger.LogDebug(ctx, "subscription maintenance: expired_count=%d", totalExpired)
	}
}
