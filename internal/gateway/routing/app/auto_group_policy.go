package app

import (
	"sort"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/samber/hot"
	"github.com/sh2001sh/CodeGo-Api/constant"
	platformcache "github.com/sh2001sh/CodeGo-Api/internal/platform/cache"
	"github.com/sh2001sh/CodeGo-Api/internal/platform/cachex"
	platformhttpctx "github.com/sh2001sh/CodeGo-Api/internal/platform/transport/http/httpctx"
)

const (
	autoGroupFailureThreshold = 3
	autoGroupCooldownDuration = 2 * time.Minute
	autoGroupCircuitTTL       = 10 * time.Minute
)

type autoGroupCircuit struct {
	ConsecutiveFailures int       `json:"consecutive_failures"`
	CooldownUntil       time.Time `json:"cooldown_until"`
}

var (
	autoGroupCircuitCacheOnce sync.Once
	autoGroupCircuitCache     *cachex.HybridCache[autoGroupCircuit]
	autoGroupCircuitLocks     [64]sync.Mutex
)

func autoGroupCircuitKey(group string, model string) string {
	return group + "\x00" + model
}

func getAutoGroupCircuitCache() *cachex.HybridCache[autoGroupCircuit] {
	autoGroupCircuitCacheOnce.Do(func() {
		autoGroupCircuitCache = cachex.NewHybridCache[autoGroupCircuit](cachex.HybridCacheConfig[autoGroupCircuit]{
			Namespace:  cachex.Namespace("codego-api:auto_group_circuit:v1"),
			Redis:      platformcache.RDB,
			RedisCodec: cachex.JSONCodec[autoGroupCircuit]{},
			RedisEnabled: func() bool {
				return platformcache.RedisEnabled && platformcache.RDB != nil
			},
			Memory: func() *hot.HotCache[string, autoGroupCircuit] {
				return hot.NewHotCache[string, autoGroupCircuit](hot.LRU, 10_000).
					WithTTL(autoGroupCircuitTTL).
					WithJanitor().
					Build()
			},
		})
	})
	return autoGroupCircuitCache
}

func autoGroupCircuitLock(key string) *sync.Mutex {
	index := 0
	for _, char := range key {
		index = (index*31 + int(char)) % len(autoGroupCircuitLocks)
	}
	return &autoGroupCircuitLocks[index]
}

func isAutoGroupCooling(group string, model string, now time.Time) bool {
	state, found, err := getAutoGroupCircuitCache().Get(autoGroupCircuitKey(group, model))
	return err == nil && found && state.CooldownUntil.After(now)
}

func recordAutoGroupSuccess(group string, model string) {
	if group == "" || model == "" {
		return
	}
	key := autoGroupCircuitKey(group, model)
	lock := autoGroupCircuitLock(key)
	lock.Lock()
	defer lock.Unlock()
	_, _ = getAutoGroupCircuitCache().DeleteMany([]string{key})
}

func recordAutoGroupFailure(group string, model string, now time.Time) {
	if group == "" || model == "" {
		return
	}

	key := autoGroupCircuitKey(group, model)
	lock := autoGroupCircuitLock(key)
	lock.Lock()
	defer lock.Unlock()

	state, found, err := getAutoGroupCircuitCache().Get(key)
	if err != nil {
		return
	}
	if !found {
		state = autoGroupCircuit{}
	}
	if state.CooldownUntil.After(now) {
		return
	}
	state.ConsecutiveFailures++
	if state.ConsecutiveFailures >= autoGroupFailureThreshold {
		state.ConsecutiveFailures = 0
		state.CooldownUntil = now.Add(autoGroupCooldownDuration)
	}
	_ = getAutoGroupCircuitCache().SetWithTTL(key, state, autoGroupCircuitTTL)
}

func resetAutoGroupCircuitCacheForTest() error {
	if autoGroupCircuitCache != nil {
		if err := autoGroupCircuitCache.Purge(); err != nil {
			return err
		}
	}
	autoGroupCircuitCacheOnce = sync.Once{}
	autoGroupCircuitCache = nil
	return nil
}

// OrderAutoGroups returns permitted auto groups ordered by availability policy.
// Lower effective user pricing is preferred unless that group/model is cooling.
func OrderAutoGroups(userGroup string, model string) []string {
	groups := GetUserAutoGroup(userGroup)
	now := time.Now()
	sort.SliceStable(groups, func(i, j int) bool {
		leftCooling := isAutoGroupCooling(groups[i], model, now)
		rightCooling := isAutoGroupCooling(groups[j], model, now)
		if leftCooling != rightCooling {
			return !leftCooling
		}
		return GetUserGroupRatio(userGroup, groups[i]) < GetUserGroupRatio(userGroup, groups[j])
	})
	return groups
}

func selectedAutoGroup(ctx *gin.Context) string {
	if ctx == nil || platformhttpctx.GetContextKeyString(ctx, constant.ContextKeyTokenGroup) != AutoGroupName {
		return ""
	}
	return platformhttpctx.GetContextKeyString(ctx, constant.ContextKeyAutoGroup)
}

// RecordAutoGroupSuccess closes the model-specific circuit after a successful relay.
func RecordAutoGroupSuccess(ctx *gin.Context, model string) {
	recordAutoGroupSuccess(selectedAutoGroup(ctx), model)
}

// RecordAutoGroupFailure advances the model-specific circuit after a retryable relay failure.
func RecordAutoGroupFailure(ctx *gin.Context, model string) {
	recordAutoGroupFailure(selectedAutoGroup(ctx), model, time.Now())
}
