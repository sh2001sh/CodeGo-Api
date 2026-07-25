package notifyx

import (
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/bytedance/gopkg/util/gopool"
	"github.com/sh2001sh/CodeGo-Api/constant"
	platformcache "github.com/sh2001sh/CodeGo-Api/internal/platform/cache"
)

var (
	notifyLimitStore sync.Map
	cleanupOnce      sync.Once
)

type limitCount struct {
	Count     int
	Timestamp time.Time
}

func getDuration() time.Duration {
	minute := constant.NotificationLimitDurationMinute
	return time.Duration(minute) * time.Minute
}

func startCleanupTask() {
	gopool.Go(func() {
		for {
			time.Sleep(time.Hour)
			now := time.Now()
			notifyLimitStore.Range(func(key, value interface{}) bool {
				if limit, ok := value.(limitCount); ok {
					if now.Sub(limit.Timestamp) >= getDuration() {
						notifyLimitStore.Delete(key)
					}
				}
				return true
			})
		}
	})
}

func CheckNotificationLimit(userID int, notifyType string) (bool, error) {
	if platformcache.RedisEnabled {
		return checkRedisLimit(userID, notifyType)
	}
	return checkMemoryLimit(userID, notifyType)
}

func checkRedisLimit(userID int, notifyType string) (bool, error) {
	key := fmt.Sprintf("notify_limit:%d:%s:%s", userID, notifyType, time.Now().Format("2006010215"))

	count, err := platformcache.RedisGet(key)
	if err != nil && err.Error() != "redis: nil" {
		return false, fmt.Errorf("failed to get notification count: %w", err)
	}

	if count == "" {
		err = platformcache.RedisSet(key, "1", getDuration())
		return true, err
	}

	currentCount, _ := strconv.Atoi(count)
	limit := constant.NotifyLimitCount
	if currentCount >= limit {
		return false, nil
	}

	err = platformcache.RedisIncr(key, 1)
	if err != nil {
		return false, fmt.Errorf("failed to increment notification count: %w", err)
	}

	return true, nil
}

func checkMemoryLimit(userID int, notifyType string) (bool, error) {
	cleanupOnce.Do(startCleanupTask)

	key := fmt.Sprintf("%d:%s:%s", userID, notifyType, time.Now().Format("2006010215"))
	now := time.Now()

	var currentLimit limitCount
	if value, ok := notifyLimitStore.Load(key); ok {
		currentLimit = value.(limitCount)
		if now.Sub(currentLimit.Timestamp) >= getDuration() {
			currentLimit = limitCount{Count: 0, Timestamp: now}
		}
	} else {
		currentLimit = limitCount{Count: 0, Timestamp: now}
	}

	currentLimit.Count++
	limit := constant.NotifyLimitCount
	notifyLimitStore.Store(key, currentLimit)

	return currentLimit.Count <= limit, nil
}
