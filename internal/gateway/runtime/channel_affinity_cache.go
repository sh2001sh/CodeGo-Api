package runtime

import (
	"context"
	"fmt"
	gatewaystore "github.com/sh2001sh/CodeGo-Api/internal/gateway/store"
	platformobservability "github.com/sh2001sh/CodeGo-Api/internal/platform/observability"
	"hash/fnv"
	"strings"
	"sync"
	"time"

	"github.com/samber/hot"
	"github.com/sh2001sh/CodeGo-Api/dto"
	platformcache "github.com/sh2001sh/CodeGo-Api/internal/platform/cache"
	"github.com/sh2001sh/CodeGo-Api/internal/platform/cachex"
	"github.com/sh2001sh/CodeGo-Api/types"
)

const (
	channelAffinityCacheNamespace                = "codego-api:channel_affinity:v1"
	channelAffinityUsageCacheStatsNamespace      = "codego-api:channel_affinity_usage_cache_stats:v1"
	cacheTokenRateModeCachedOverPrompt           = "cached_over_prompt"
	cacheTokenRateModeCachedOverPromptPlusCached = "cached_over_prompt_plus_cached"
	cacheTokenRateModeMixed                      = "mixed"
)

var (
	channelAffinityCacheOnce sync.Once
	channelAffinityCache     *cachex.HybridCache[int]

	channelAffinityUsageCacheStatsOnce  sync.Once
	channelAffinityUsageCacheStatsCache *cachex.HybridCache[ChannelAffinityUsageCacheCounters]

	channelAffinityUsageCacheStatsLocks [64]sync.Mutex
	channelAffinityIndexMu              sync.Mutex
	channelAffinityIndex                = make(map[int]map[string]struct{})
)

const channelAffinityIndexNamespace = "codego-api:channel_affinity_index:v1"

type ChannelAffinityStatsContext struct {
	RuleName       string
	UsingGroup     string
	KeyFingerprint string
	TTLSeconds     int64
}

type ChannelAffinityCacheStats struct {
	Enabled       bool           `json:"enabled"`
	Total         int            `json:"total"`
	Unknown       int            `json:"unknown"`
	ByRuleName    map[string]int `json:"by_rule_name"`
	CacheCapacity int            `json:"cache_capacity"`
	CacheAlgo     string         `json:"cache_algo"`
}

type ChannelAffinityUsageCacheStats struct {
	RuleName             string `json:"rule_name"`
	UsingGroup           string `json:"using_group"`
	KeyFingerprint       string `json:"key_fp"`
	CachedTokenRateMode  string `json:"cached_token_rate_mode"`
	Hit                  int64  `json:"hit"`
	Total                int64  `json:"total"`
	WindowSeconds        int64  `json:"window_seconds"`
	PromptTokens         int64  `json:"prompt_tokens"`
	CompletionTokens     int64  `json:"completion_tokens"`
	TotalTokens          int64  `json:"total_tokens"`
	CachedTokens         int64  `json:"cached_tokens"`
	PromptCacheHitTokens int64  `json:"prompt_cache_hit_tokens"`
	LastSeenAt           int64  `json:"last_seen_at"`
}

type ChannelAffinityUsageCacheCounters struct {
	CachedTokenRateMode  string `json:"cached_token_rate_mode"`
	Hit                  int64  `json:"hit"`
	Total                int64  `json:"total"`
	WindowSeconds        int64  `json:"window_seconds"`
	PromptTokens         int64  `json:"prompt_tokens"`
	CompletionTokens     int64  `json:"completion_tokens"`
	TotalTokens          int64  `json:"total_tokens"`
	CachedTokens         int64  `json:"cached_tokens"`
	PromptCacheHitTokens int64  `json:"prompt_cache_hit_tokens"`
	LastSeenAt           int64  `json:"last_seen_at"`
}

func getChannelAffinityCache() *cachex.HybridCache[int] {
	channelAffinityCacheOnce.Do(func() {
		setting := gatewaystore.GetChannelAffinitySetting()
		capacity := 100_000
		defaultTTLSeconds := 3600
		if setting != nil {
			if setting.MaxEntries > 0 {
				capacity = setting.MaxEntries
			}
			if setting.DefaultTTLSeconds > 0 {
				defaultTTLSeconds = setting.DefaultTTLSeconds
			}
		}

		channelAffinityCache = cachex.NewHybridCache[int](cachex.HybridCacheConfig[int]{
			Namespace: cachex.Namespace(channelAffinityCacheNamespace),
			Redis:     platformcache.RDB,
			RedisEnabled: func() bool {
				return platformcache.RedisEnabled && platformcache.RDB != nil
			},
			RedisCodec: cachex.IntCodec{},
			Memory: func() *hot.HotCache[string, int] {
				return hot.NewHotCache[string, int](hot.LRU, capacity).
					WithTTL(time.Duration(defaultTTLSeconds) * time.Second).
					WithJanitor().
					Build()
			},
		})
	})
	return channelAffinityCache
}

func GetPreferredChannel(cacheKey string) (int, bool, error) {
	return getChannelAffinityCache().Get(cacheKey)
}

func RecordPreferredChannel(cacheKey string, channelID int, ttlSeconds int) error {
	if ttlSeconds <= 0 {
		ttlSeconds = 3600
	}
	if err := getChannelAffinityCache().SetWithTTL(cacheKey, channelID, time.Duration(ttlSeconds)*time.Second); err != nil {
		return err
	}
	recordChannelAffinityIndex(channelID, cacheKey, time.Duration(ttlSeconds)*time.Second)
	return nil
}

func channelAffinityIndexKey(channelID int) string {
	return fmt.Sprintf("%s:%d", channelAffinityIndexNamespace, channelID)
}

func recordChannelAffinityIndex(channelID int, cacheKey string, ttl time.Duration) {
	if channelID <= 0 || strings.TrimSpace(cacheKey) == "" {
		return
	}
	channelAffinityIndexMu.Lock()
	keys := channelAffinityIndex[channelID]
	if keys == nil {
		keys = make(map[string]struct{})
		channelAffinityIndex[channelID] = keys
	}
	keys[cacheKey] = struct{}{}
	channelAffinityIndexMu.Unlock()

	if !platformcache.RedisEnabled || platformcache.RDB == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	pipe := platformcache.RDB.TxPipeline()
	pipe.SAdd(ctx, channelAffinityIndexKey(channelID), cacheKey)
	pipe.Expire(ctx, channelAffinityIndexKey(channelID), ttl)
	_, _ = pipe.Exec(ctx)
}

// InvalidateChannelAffinityForChannel clears all affinity entries that currently point to a channel.
// Redis stores the reverse index so disabled channels are invalidated across gateway replicas.
func InvalidateChannelAffinityForChannel(channelID int) int {
	if channelID <= 0 {
		return 0
	}
	keys := make(map[string]struct{})
	channelAffinityIndexMu.Lock()
	for key := range channelAffinityIndex[channelID] {
		keys[key] = struct{}{}
	}
	delete(channelAffinityIndex, channelID)
	channelAffinityIndexMu.Unlock()

	if platformcache.RedisEnabled && platformcache.RDB != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		redisKeys, err := platformcache.RDB.SMembers(ctx, channelAffinityIndexKey(channelID)).Result()
		if err == nil {
			for _, key := range redisKeys {
				keys[key] = struct{}{}
			}
		}
		_ = platformcache.RDB.Del(ctx, channelAffinityIndexKey(channelID)).Err()
		cancel()
	}

	cacheKeys := make([]string, 0, len(keys))
	for key := range keys {
		cacheKeys = append(cacheKeys, key)
	}
	deleted, err := getChannelAffinityCache().DeleteMany(cacheKeys)
	if err != nil {
		platformobservability.SysError(fmt.Sprintf("channel affinity invalidation failed: channel_id=%d, err=%v", channelID, err))
		return 0
	}
	count := 0
	for _, didDelete := range deleted {
		if didDelete {
			count++
		}
	}
	return count
}

func invalidateChannelAffinityCacheKey(cacheKey string) {
	if strings.TrimSpace(cacheKey) == "" {
		return
	}
	_, _ = getChannelAffinityCache().DeleteMany([]string{cacheKey})
}

// InvalidatePreferredChannel removes one affinity entry. Automatic route pools
// use this when a short-lived sticky route becomes unhealthy or unavailable.
func InvalidatePreferredChannel(cacheKey string) {
	invalidateChannelAffinityCacheKey(cacheKey)
}

func ResetChannelAffinityCacheForTest() error {
	if channelAffinityCache != nil {
		if err := channelAffinityCache.Purge(); err != nil {
			return err
		}
	}
	channelAffinityCacheOnce = sync.Once{}
	channelAffinityCache = nil
	channelAffinityIndexMu.Lock()
	channelAffinityIndex = make(map[int]map[string]struct{})
	channelAffinityIndexMu.Unlock()
	return nil
}

func GetChannelAffinityCacheStats() ChannelAffinityCacheStats {
	setting := gatewaystore.GetChannelAffinitySetting()
	if setting == nil {
		return ChannelAffinityCacheStats{
			Enabled:    false,
			Total:      0,
			Unknown:    0,
			ByRuleName: map[string]int{},
		}
	}

	cache := getChannelAffinityCache()
	mainCap, _ := cache.Capacity()
	mainAlgo, _ := cache.Algorithm()

	ruleByName := make(map[string]gatewaystore.ChannelAffinityRule, len(setting.Rules))
	for _, rule := range setting.Rules {
		name := strings.TrimSpace(rule.Name)
		if name == "" || !rule.IncludeRuleName {
			continue
		}
		ruleByName[name] = rule
	}

	byRuleName := make(map[string]int, len(ruleByName))
	for name := range ruleByName {
		byRuleName[name] = 0
	}

	keys, err := cache.Keys()
	if err != nil {
		platformobservability.SysError(fmt.Sprintf("channel affinity cache list keys failed: err=%v", err))
		keys = nil
	}

	total := 0
	unknown := 0
	for _, key := range keys {
		prefix := channelAffinityCacheNamespace + ":"
		if !strings.HasPrefix(key, prefix) {
			unknown++
			continue
		}
		rest := strings.TrimPrefix(key, prefix)
		if strings.HasPrefix(rest, "route_pool:") {
			continue
		}
		total++
		parts := strings.Split(rest, ":")
		if len(parts) < 2 {
			unknown++
			continue
		}

		ruleName := parts[0]
		rule, ok := ruleByName[ruleName]
		if !ok {
			unknown++
			continue
		}
		if rule.IncludeModelName && len(parts) < 3 {
			unknown++
			continue
		}
		if rule.IncludeUsingGroup {
			minParts := 3
			if rule.IncludeModelName {
				minParts = 4
			}
			if len(parts) < minParts {
				unknown++
				continue
			}
		}
		byRuleName[ruleName]++
	}

	return ChannelAffinityCacheStats{
		Enabled:       setting.Enabled,
		Total:         total,
		Unknown:       unknown,
		ByRuleName:    byRuleName,
		CacheCapacity: mainCap,
		CacheAlgo:     mainAlgo,
	}
}

func ClearChannelAffinityCacheAll() int {
	cache := getChannelAffinityCache()
	keys, err := cache.Keys()
	if err != nil {
		platformobservability.SysError(fmt.Sprintf("channel affinity cache list keys failed: err=%v", err))
		keys = nil
	}
	if len(keys) > 0 {
		if _, err := cache.DeleteMany(keys); err != nil {
			platformobservability.SysError(fmt.Sprintf("channel affinity cache delete many failed: err=%v", err))
		}
	}
	return len(keys)
}

func ClearChannelAffinityCacheByRuleName(ruleName string) (int, error) {
	ruleName = strings.TrimSpace(ruleName)
	if ruleName == "" {
		return 0, fmt.Errorf("rule_name 不能为空")
	}

	setting := gatewaystore.GetChannelAffinitySetting()
	if setting == nil {
		return 0, fmt.Errorf("channel_affinity_setting 未初始化")
	}

	var matchedRule *gatewaystore.ChannelAffinityRule
	for index := range setting.Rules {
		rule := &setting.Rules[index]
		if strings.TrimSpace(rule.Name) != ruleName {
			continue
		}
		matchedRule = rule
		break
	}
	if matchedRule == nil {
		return 0, fmt.Errorf("未知规则名称")
	}
	if !matchedRule.IncludeRuleName {
		return 0, fmt.Errorf("该规则未启用 include_rule_name，无法按规则清空缓存")
	}

	return getChannelAffinityCache().DeleteByPrefix(ruleName)
}

func ObserveChannelAffinityUsageCacheByRelayFormat(statsCtx ChannelAffinityStatsContext, usage *dto.Usage, relayFormat types.RelayFormat) {
	ObserveChannelAffinityUsageCache(statsCtx, usage, cachedTokenRateModeByRelayFormat(relayFormat))
}

func ObserveChannelAffinityUsageCache(statsCtx ChannelAffinityStatsContext, usage *dto.Usage, cachedTokenRateMode string) {
	entryKey := channelAffinityUsageCacheEntryKey(statsCtx.RuleName, statsCtx.UsingGroup, statsCtx.KeyFingerprint)
	if entryKey == "" || statsCtx.TTLSeconds <= 0 {
		return
	}

	cache := getChannelAffinityUsageCacheStatsCache()
	lock := channelAffinityUsageCacheStatsLock(entryKey)
	lock.Lock()
	defer lock.Unlock()

	prev, found, err := cache.Get(entryKey)
	if err != nil {
		return
	}
	next := prev
	if !found {
		next = ChannelAffinityUsageCacheCounters{}
	}

	currentMode := normalizeCachedTokenRateMode(cachedTokenRateMode)
	if currentMode != "" {
		if next.CachedTokenRateMode == "" {
			next.CachedTokenRateMode = currentMode
		} else if next.CachedTokenRateMode != currentMode && next.CachedTokenRateMode != cacheTokenRateModeMixed {
			next.CachedTokenRateMode = cacheTokenRateModeMixed
		}
	}

	next.Total++
	hit, cachedTokens, promptCacheHitTokens := usageCacheSignals(usage)
	if hit {
		next.Hit++
	}
	next.WindowSeconds = statsCtx.TTLSeconds
	next.LastSeenAt = time.Now().Unix()
	next.CachedTokens += cachedTokens
	next.PromptCacheHitTokens += promptCacheHitTokens
	next.PromptTokens += int64(usagePromptTokens(usage))
	next.CompletionTokens += int64(usageCompletionTokens(usage))
	next.TotalTokens += int64(usageTotalTokens(usage))
	_ = cache.SetWithTTL(entryKey, next, time.Duration(statsCtx.TTLSeconds)*time.Second)
}

func GetChannelAffinityUsageCacheStats(ruleName, usingGroup, keyFp string) ChannelAffinityUsageCacheStats {
	ruleName = strings.TrimSpace(ruleName)
	usingGroup = strings.TrimSpace(usingGroup)
	keyFp = strings.TrimSpace(keyFp)

	entryKey := channelAffinityUsageCacheEntryKey(ruleName, usingGroup, keyFp)
	if entryKey == "" {
		return ChannelAffinityUsageCacheStats{
			RuleName:       ruleName,
			UsingGroup:     usingGroup,
			KeyFingerprint: keyFp,
		}
	}

	cache := getChannelAffinityUsageCacheStatsCache()
	value, found, err := cache.Get(entryKey)
	if err != nil || !found {
		return ChannelAffinityUsageCacheStats{
			RuleName:       ruleName,
			UsingGroup:     usingGroup,
			KeyFingerprint: keyFp,
		}
	}

	return ChannelAffinityUsageCacheStats{
		CachedTokenRateMode:  value.CachedTokenRateMode,
		RuleName:             ruleName,
		UsingGroup:           usingGroup,
		KeyFingerprint:       keyFp,
		Hit:                  value.Hit,
		Total:                value.Total,
		WindowSeconds:        value.WindowSeconds,
		PromptTokens:         value.PromptTokens,
		CompletionTokens:     value.CompletionTokens,
		TotalTokens:          value.TotalTokens,
		CachedTokens:         value.CachedTokens,
		PromptCacheHitTokens: value.PromptCacheHitTokens,
		LastSeenAt:           value.LastSeenAt,
	}
}

func ResetChannelAffinityUsageCacheStatsForTest() error {
	if channelAffinityUsageCacheStatsCache != nil {
		if err := channelAffinityUsageCacheStatsCache.Purge(); err != nil {
			return err
		}
	}
	channelAffinityUsageCacheStatsOnce = sync.Once{}
	channelAffinityUsageCacheStatsCache = nil
	return nil
}

func normalizeCachedTokenRateMode(mode string) string {
	switch mode {
	case cacheTokenRateModeCachedOverPrompt:
		return cacheTokenRateModeCachedOverPrompt
	case cacheTokenRateModeCachedOverPromptPlusCached:
		return cacheTokenRateModeCachedOverPromptPlusCached
	case cacheTokenRateModeMixed:
		return cacheTokenRateModeMixed
	default:
		return ""
	}
}

func cachedTokenRateModeByRelayFormat(relayFormat types.RelayFormat) string {
	switch relayFormat {
	case types.RelayFormatOpenAI, types.RelayFormatOpenAIResponses, types.RelayFormatOpenAIResponsesCompaction:
		return cacheTokenRateModeCachedOverPrompt
	case types.RelayFormatClaude:
		return cacheTokenRateModeCachedOverPromptPlusCached
	default:
		return ""
	}
}

func channelAffinityUsageCacheEntryKey(ruleName, usingGroup, keyFp string) string {
	ruleName = strings.TrimSpace(ruleName)
	usingGroup = strings.TrimSpace(usingGroup)
	keyFp = strings.TrimSpace(keyFp)
	if ruleName == "" || keyFp == "" {
		return ""
	}
	return ruleName + "\n" + usingGroup + "\n" + keyFp
}

func usageCacheSignals(usage *dto.Usage) (hit bool, cachedTokens int64, promptCacheHitTokens int64) {
	if usage == nil {
		return false, 0, 0
	}

	cached := int64(0)
	if usage.PromptTokensDetails.CachedTokens > 0 {
		cached = int64(usage.PromptTokensDetails.CachedTokens)
	} else if usage.InputTokensDetails != nil && usage.InputTokensDetails.CachedTokens > 0 {
		cached = int64(usage.InputTokensDetails.CachedTokens)
	}
	promptCacheHit := int64(0)
	if usage.PromptCacheHitTokens > 0 {
		promptCacheHit = int64(usage.PromptCacheHitTokens)
	}
	return cached > 0 || promptCacheHit > 0, cached, promptCacheHit
}

func usagePromptTokens(usage *dto.Usage) int {
	if usage == nil {
		return 0
	}
	if usage.PromptTokens > 0 {
		return usage.PromptTokens
	}
	return usage.InputTokens
}

func usageCompletionTokens(usage *dto.Usage) int {
	if usage == nil {
		return 0
	}
	if usage.CompletionTokens > 0 {
		return usage.CompletionTokens
	}
	return usage.OutputTokens
}

func usageTotalTokens(usage *dto.Usage) int {
	if usage == nil {
		return 0
	}
	if usage.TotalTokens > 0 {
		return usage.TotalTokens
	}
	promptTokens := usagePromptTokens(usage)
	completionTokens := usageCompletionTokens(usage)
	if promptTokens > 0 || completionTokens > 0 {
		return promptTokens + completionTokens
	}
	return 0
}

func getChannelAffinityUsageCacheStatsCache() *cachex.HybridCache[ChannelAffinityUsageCacheCounters] {
	channelAffinityUsageCacheStatsOnce.Do(func() {
		setting := gatewaystore.GetChannelAffinitySetting()
		capacity := 100_000
		defaultTTLSeconds := 3600
		if setting != nil {
			if setting.MaxEntries > 0 {
				capacity = setting.MaxEntries
			}
			if setting.DefaultTTLSeconds > 0 {
				defaultTTLSeconds = setting.DefaultTTLSeconds
			}
		}

		channelAffinityUsageCacheStatsCache = cachex.NewHybridCache[ChannelAffinityUsageCacheCounters](cachex.HybridCacheConfig[ChannelAffinityUsageCacheCounters]{
			Namespace: cachex.Namespace(channelAffinityUsageCacheStatsNamespace),
			Redis:     platformcache.RDB,
			RedisEnabled: func() bool {
				return platformcache.RedisEnabled && platformcache.RDB != nil
			},
			RedisCodec: cachex.JSONCodec[ChannelAffinityUsageCacheCounters]{},
			Memory: func() *hot.HotCache[string, ChannelAffinityUsageCacheCounters] {
				return hot.NewHotCache[string, ChannelAffinityUsageCacheCounters](hot.LRU, capacity).
					WithTTL(time.Duration(defaultTTLSeconds) * time.Second).
					WithJanitor().
					Build()
			},
		})
	})
	return channelAffinityUsageCacheStatsCache
}

func channelAffinityUsageCacheStatsLock(key string) *sync.Mutex {
	h := fnv.New32a()
	_, _ = h.Write([]byte(key))
	index := h.Sum32() % uint32(len(channelAffinityUsageCacheStatsLocks))
	return &channelAffinityUsageCacheStatsLocks[index]
}
