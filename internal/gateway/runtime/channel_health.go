package runtime

import (
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/samber/hot"
	platformcache "github.com/sh2001sh/CodeGo-Api/internal/platform/cache"
	"github.com/sh2001sh/CodeGo-Api/internal/platform/cachex"
)

const (
	ChannelHealthHealthy  = "healthy"
	ChannelHealthDegraded = "degraded"
	ChannelHealthCooling  = "cooling"

	channelHealthFailureThreshold  = 5
	channelHealthShortCooldown     = 15 * time.Second
	channelHealthRateLimitCooldown = 30 * time.Second
	channelHealthCooldownDuration  = 2 * time.Minute
	channelHealthTTL               = 20 * time.Minute
	channelModelUnavailableTTL     = 5 * time.Minute
	channelModelUpstreamFailureTTL = 2 * time.Minute
	channelHealthShortWindow       = 2 * time.Minute
	channelHealthShortMinRequests  = 5
	channelHealthShortMaxSuccess   = 40.0
	channelHealthSlowTTFTSamples   = 3
	channelHealthSlowTTFTThreshold = 12 * time.Second
	channelHealthTTFTWindow        = 20
	channelHealthSlowTTFTP95       = 15 * time.Second
)

// ChannelHealth captures the shared routing health for one channel/model pair.
// It deliberately excludes provider credentials and other user-visible details.
type ChannelHealth struct {
	ChannelID                    int       `json:"channel_id"`
	Model                        string    `json:"model"`
	State                        string    `json:"state"`
	ConsecutiveRetryableFailures int       `json:"consecutive_retryable_failures"`
	RecoveryProbeSuccesses       int       `json:"recovery_probe_successes"`
	CoolingUntil                 time.Time `json:"cooling_until"`
	SuccessRate2m                float64   `json:"success_rate_2m"`
	SuccessRate5m                float64   `json:"success_rate_5m"`
	SuccessRate15m               float64   `json:"success_rate_15m"`
	TTFTEWMAMilliseconds         float64   `json:"ttft_ewma_ms"`
	TTFTSamples                  int       `json:"ttft_samples"`
	TTFTP50Milliseconds          float64   `json:"ttft_p50_ms"`
	TTFTP95Milliseconds          float64   `json:"ttft_p95_ms"`
	TTFTRecentMilliseconds       []int64   `json:"ttft_recent_ms"`
	LastSuccessAt                time.Time `json:"last_success_at"`
	LastFailureAt                time.Time `json:"last_failure_at"`
	LastFailureRequestID         string    `json:"last_failure_request_id"`

	Window2StartedAt  time.Time `json:"window_2_started_at"`
	Window2Requests   int       `json:"window_2_requests"`
	Window2Successes  int       `json:"window_2_successes"`
	Window5StartedAt  time.Time `json:"window_5_started_at"`
	Window5Requests   int       `json:"window_5_requests"`
	Window5Successes  int       `json:"window_5_successes"`
	Window15StartedAt time.Time `json:"window_15_started_at"`
	Window15Requests  int       `json:"window_15_requests"`
	Window15Successes int       `json:"window_15_successes"`
}

var (
	channelHealthCacheOnce sync.Once
	channelHealthCache     *cachex.HybridCache[ChannelHealth]
	channelHealthLocks     [64]sync.Mutex
)

func channelHealthKey(channelID int, model string) string {
	return strconv.Itoa(channelID) + "\x00" + model
}

func getChannelHealthCache() *cachex.HybridCache[ChannelHealth] {
	channelHealthCacheOnce.Do(func() {
		channelHealthCache = cachex.NewHybridCache[ChannelHealth](cachex.HybridCacheConfig[ChannelHealth]{
			Namespace:  cachex.Namespace("codego-api:channel_health:v1"),
			Redis:      platformcache.RDB,
			RedisCodec: cachex.JSONCodec[ChannelHealth]{},
			RedisEnabled: func() bool {
				return platformcache.RedisEnabled && platformcache.RDB != nil
			},
			Memory: func() *hot.HotCache[string, ChannelHealth] {
				return hot.NewHotCache[string, ChannelHealth](hot.LRU, 100_000).
					WithTTL(channelHealthTTL).
					WithJanitor().
					Build()
			},
		})
	})
	return channelHealthCache
}

func channelHealthLock(channelID int) *sync.Mutex {
	return &channelHealthLocks[channelID%len(channelHealthLocks)]
}

// GetChannelHealth returns the last shared health state for a channel/model pair.
func GetChannelHealth(channelID int, model string) (ChannelHealth, bool) {
	if channelID <= 0 || model == "" {
		return ChannelHealth{}, false
	}
	state, found, err := getChannelHealthCache().Get(channelHealthKey(channelID, model))
	return state, found && err == nil
}

// IsChannelCooling reports whether routing must skip the channel/model pair.
func IsChannelCooling(channelID int, model string) bool {
	state, found := GetChannelHealth(channelID, model)
	return found && state.CoolingUntil.After(time.Now())
}

// RecordChannelModelUnavailable opens the model circuit only after five
// distinct request IDs fail consecutively. Repeated retries of one request
// count once so a single request cannot exhaust the error budget.
func RecordChannelModelUnavailable(channelID int, model string, requestID string) bool {
	if channelID <= 0 || model == "" {
		return false
	}
	lock := channelHealthLock(channelID)
	lock.Lock()
	defer lock.Unlock()

	now := time.Now().UTC()
	cooling := false
	err := getChannelHealthCache().UpdateWithTTL(channelHealthKey(channelID, model), channelHealthTTL, func(state ChannelHealth, _ bool) (ChannelHealth, error) {
		state.ChannelID = channelID
		state.Model = model
		state.LastFailureAt = now
		recordChannelHealthWindow(&state, now, false)
		if state.CoolingUntil.After(now) {
			cooling = true
			return state, nil
		}
		if requestID != "" && state.LastFailureRequestID == requestID {
			return state, nil
		}
		state.LastFailureRequestID = requestID
		state.ConsecutiveRetryableFailures++
		state.RecoveryProbeSuccesses = 0
		if state.ConsecutiveRetryableFailures >= channelHealthFailureThreshold {
			state.State = ChannelHealthCooling
			state.CoolingUntil = now.Add(channelModelUnavailableTTL)
			cooling = true
		} else {
			state.State = ChannelHealthDegraded
		}
		return state, nil
	})
	return err == nil && cooling
}

// CoolChannelModelForUpstreamFailure immediately isolates one failing model
// route while leaving the channel available for every other model.
func CoolChannelModelForUpstreamFailure(channelID int, model string) bool {
	if channelID <= 0 || model == "" {
		return false
	}
	lock := channelHealthLock(channelID)
	lock.Lock()
	defer lock.Unlock()

	now := time.Now().UTC()
	err := getChannelHealthCache().UpdateWithTTL(channelHealthKey(channelID, model), channelHealthTTL, func(state ChannelHealth, _ bool) (ChannelHealth, error) {
		state.ChannelID = channelID
		state.Model = model
		state.State = ChannelHealthCooling
		state.ConsecutiveRetryableFailures = 0
		state.RecoveryProbeSuccesses = 0
		state.CoolingUntil = now.Add(channelModelUpstreamFailureTTL)
		state.LastFailureAt = now
		state.RecoveryProbeSuccesses = 0
		recordChannelHealthWindow(&state, now, false)
		return state, nil
	})
	return err == nil
}

// RecordChannelRetryableFailure advances the shared circuit for a retryable upstream error.
func RecordChannelRetryableFailure(channelID int, model string) {
	RecordChannelRetryableFailureWithCooldown(channelID, model, channelHealthShortCooldown)
}

// RecordChannelRetryableFailureWithCooldown applies a short model-level
// cooldown for a transient failure. Repeated failures in the rolling window
// still escalate to the longer circuit cooldown.
func RecordChannelRetryableFailureWithCooldown(channelID int, model string, shortCooldown time.Duration) {
	if channelID <= 0 || model == "" {
		return
	}
	if shortCooldown <= 0 {
		shortCooldown = channelHealthShortCooldown
	}
	lock := channelHealthLock(channelID)
	lock.Lock()
	defer lock.Unlock()

	now := time.Now().UTC()
	_ = getChannelHealthCache().UpdateWithTTL(channelHealthKey(channelID, model), channelHealthTTL, func(state ChannelHealth, _ bool) (ChannelHealth, error) {
		state.ChannelID = channelID
		state.Model = model
		state.LastFailureAt = now
		recordChannelHealthWindow(&state, now, false)
		state.ConsecutiveRetryableFailures++
		if shouldCoolForShortTermFailureRate(state) || state.ConsecutiveRetryableFailures >= channelHealthFailureThreshold {
			state.ConsecutiveRetryableFailures = 0
			state.CoolingUntil = now.Add(channelHealthCooldownDuration)
			state.State = ChannelHealthCooling
			return state, nil
		}
		if state.CoolingUntil.After(now) {
			return state, nil
		}
		state.CoolingUntil = now.Add(shortCooldown)
		state.State = ChannelHealthCooling
		return state, nil
	})
}

// RetryableFailureCooldown selects a short circuit duration without exposing
// channel-specific rules. Rate limits and gateway timeouts recover more slowly
// than connection and transient 5xx errors.
func RetryableFailureCooldown(statusCode int) time.Duration {
	switch statusCode {
	case 429, 504, 524:
		return channelHealthRateLimitCooldown
	default:
		return channelHealthShortCooldown
	}
}

func shouldCoolForShortTermFailureRate(state ChannelHealth) bool {
	if state.Window2Requests < channelHealthShortMinRequests {
		return false
	}
	failures := state.Window2Requests - state.Window2Successes
	return failures >= 3 && state.SuccessRate2m <= channelHealthShortMaxSuccess
}

// RecordChannelSuccess requires two successful recovery probes before reopening a
// cooled model route. Normal healthy routes continue to recover immediately.
func RecordChannelSuccess(channelID int, model string, ttft time.Duration) {
	if channelID <= 0 || model == "" {
		return
	}
	lock := channelHealthLock(channelID)
	lock.Lock()
	defer lock.Unlock()

	now := time.Now().UTC()
	_ = getChannelHealthCache().UpdateWithTTL(channelHealthKey(channelID, model), channelHealthTTL, func(state ChannelHealth, _ bool) (ChannelHealth, error) {
		state.ChannelID = channelID
		state.Model = model
		state.LastSuccessAt = now
		recordChannelHealthWindow(&state, now, true)
		if ttft > 0 {
			recordChannelTTFT(&state, float64(ttft.Milliseconds()))
		}
		if isSlowChannelTTFT(state) {
			state.State = ChannelHealthCooling
			state.CoolingUntil = now.Add(channelHealthShortCooldown)
			state.RecoveryProbeSuccesses = 0
			return state, nil
		}
		if state.State == ChannelHealthCooling && !state.CoolingUntil.After(now) {
			state.RecoveryProbeSuccesses++
			if state.RecoveryProbeSuccesses < 2 {
				return state, nil
			}
		}
		state.ConsecutiveRetryableFailures = 0
		state.RecoveryProbeSuccesses = 0
		state.State = ChannelHealthHealthy
		state.CoolingUntil = time.Time{}
		return state, nil
	})
}

func recordChannelTTFT(state *ChannelHealth, value float64) {
	if state == nil || value <= 0 {
		return
	}
	state.TTFTSamples++
	if state.TTFTEWMAMilliseconds == 0 {
		state.TTFTEWMAMilliseconds = value
	} else {
		state.TTFTEWMAMilliseconds = state.TTFTEWMAMilliseconds*0.8 + value*0.2
	}
	state.TTFTRecentMilliseconds = append(state.TTFTRecentMilliseconds, int64(value))
	if len(state.TTFTRecentMilliseconds) > channelHealthTTFTWindow {
		state.TTFTRecentMilliseconds = state.TTFTRecentMilliseconds[len(state.TTFTRecentMilliseconds)-channelHealthTTFTWindow:]
	}
	samples := append([]int64(nil), state.TTFTRecentMilliseconds...)
	sort.Slice(samples, func(i int, j int) bool { return samples[i] < samples[j] })
	state.TTFTP50Milliseconds = percentile(samples, 50)
	state.TTFTP95Milliseconds = percentile(samples, 95)
}

func isSlowChannelTTFT(state ChannelHealth) bool {
	if state.TTFTSamples >= channelHealthSlowTTFTSamples && state.TTFTEWMAMilliseconds >= float64(channelHealthSlowTTFTThreshold.Milliseconds()) {
		return true
	}
	return len(state.TTFTRecentMilliseconds) >= 5 && state.TTFTP95Milliseconds >= float64(channelHealthSlowTTFTP95.Milliseconds())
}

func percentile(samples []int64, percentage int) float64 {
	if len(samples) == 0 {
		return 0
	}
	index := (len(samples)*percentage + 99) / 100
	if index > 0 {
		index--
	}
	return float64(samples[index])
}

func recordChannelHealthWindow(state *ChannelHealth, now time.Time, success bool) {
	if state.Window2StartedAt.IsZero() || now.Sub(state.Window2StartedAt) >= channelHealthShortWindow {
		state.Window2StartedAt = now
		state.Window2Requests = 0
		state.Window2Successes = 0
	}
	if state.Window5StartedAt.IsZero() || now.Sub(state.Window5StartedAt) >= 5*time.Minute {
		state.Window5StartedAt = now
		state.Window5Requests = 0
		state.Window5Successes = 0
	}
	if state.Window15StartedAt.IsZero() || now.Sub(state.Window15StartedAt) >= 15*time.Minute {
		state.Window15StartedAt = now
		state.Window15Requests = 0
		state.Window15Successes = 0
	}
	state.Window2Requests++
	state.Window5Requests++
	state.Window15Requests++
	if success {
		state.Window2Successes++
		state.Window5Successes++
		state.Window15Successes++
	}
	state.SuccessRate2m = float64(state.Window2Successes) / float64(state.Window2Requests) * 100
	state.SuccessRate5m = float64(state.Window5Successes) / float64(state.Window5Requests) * 100
	state.SuccessRate15m = float64(state.Window15Successes) / float64(state.Window15Requests) * 100
}

func resetChannelHealthForTest() error {
	if channelHealthCache != nil {
		if err := channelHealthCache.Purge(); err != nil {
			return err
		}
	}
	channelHealthCacheOnce = sync.Once{}
	channelHealthCache = nil
	return nil
}
