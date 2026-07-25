package runtime

import (
	"fmt"
	"testing"
	"time"

	"github.com/sh2001sh/CodeGo-Api/dto"
	"github.com/sh2001sh/CodeGo-Api/types"
	"github.com/stretchr/testify/require"
)

func TestObserveChannelAffinityUsageCacheByRelayFormat_ClaudeMode(t *testing.T) {
	require.NoError(t, ResetChannelAffinityUsageCacheStatsForTest())

	statsCtx := ChannelAffinityStatsContext{
		RuleName:       fmt.Sprintf("rule_%d", time.Now().UnixNano()),
		UsingGroup:     "default",
		KeyFingerprint: fmt.Sprintf("fp_%d", time.Now().UnixNano()),
		TTLSeconds:     600,
	}
	usage := &dto.Usage{
		PromptTokens:     100,
		CompletionTokens: 40,
		TotalTokens:      140,
		PromptTokensDetails: dto.InputTokenDetails{
			CachedTokens: 30,
		},
	}

	ObserveChannelAffinityUsageCacheByRelayFormat(statsCtx, usage, types.RelayFormatClaude)
	stats := GetChannelAffinityUsageCacheStats(statsCtx.RuleName, statsCtx.UsingGroup, statsCtx.KeyFingerprint)

	require.EqualValues(t, 1, stats.Total)
	require.EqualValues(t, 1, stats.Hit)
	require.EqualValues(t, 100, stats.PromptTokens)
	require.EqualValues(t, 40, stats.CompletionTokens)
	require.EqualValues(t, 140, stats.TotalTokens)
	require.EqualValues(t, 30, stats.CachedTokens)
	require.Equal(t, cacheTokenRateModeCachedOverPromptPlusCached, stats.CachedTokenRateMode)
}

func TestObserveChannelAffinityUsageCacheByRelayFormat_MixedMode(t *testing.T) {
	require.NoError(t, ResetChannelAffinityUsageCacheStatsForTest())

	statsCtx := ChannelAffinityStatsContext{
		RuleName:       fmt.Sprintf("rule_%d", time.Now().UnixNano()),
		UsingGroup:     "default",
		KeyFingerprint: fmt.Sprintf("fp_%d", time.Now().UnixNano()),
		TTLSeconds:     600,
	}
	openAIUsage := &dto.Usage{
		PromptTokens: 100,
		PromptTokensDetails: dto.InputTokenDetails{
			CachedTokens: 10,
		},
	}
	claudeUsage := &dto.Usage{
		PromptTokens: 80,
		PromptTokensDetails: dto.InputTokenDetails{
			CachedTokens: 20,
		},
	}

	ObserveChannelAffinityUsageCacheByRelayFormat(statsCtx, openAIUsage, types.RelayFormatOpenAI)
	ObserveChannelAffinityUsageCacheByRelayFormat(statsCtx, claudeUsage, types.RelayFormatClaude)
	stats := GetChannelAffinityUsageCacheStats(statsCtx.RuleName, statsCtx.UsingGroup, statsCtx.KeyFingerprint)

	require.EqualValues(t, 2, stats.Total)
	require.EqualValues(t, 2, stats.Hit)
	require.EqualValues(t, 180, stats.PromptTokens)
	require.EqualValues(t, 30, stats.CachedTokens)
	require.Equal(t, cacheTokenRateModeMixed, stats.CachedTokenRateMode)
}

func TestObserveChannelAffinityUsageCacheByRelayFormat_UnsupportedModeKeepsEmpty(t *testing.T) {
	require.NoError(t, ResetChannelAffinityUsageCacheStatsForTest())

	statsCtx := ChannelAffinityStatsContext{
		RuleName:       fmt.Sprintf("rule_%d", time.Now().UnixNano()),
		UsingGroup:     "default",
		KeyFingerprint: fmt.Sprintf("fp_%d", time.Now().UnixNano()),
		TTLSeconds:     600,
	}
	usage := &dto.Usage{
		PromptTokens: 100,
		PromptTokensDetails: dto.InputTokenDetails{
			CachedTokens: 25,
		},
	}

	ObserveChannelAffinityUsageCacheByRelayFormat(statsCtx, usage, types.RelayFormatGemini)
	stats := GetChannelAffinityUsageCacheStats(statsCtx.RuleName, statsCtx.UsingGroup, statsCtx.KeyFingerprint)

	require.EqualValues(t, 1, stats.Total)
	require.EqualValues(t, 1, stats.Hit)
	require.EqualValues(t, 25, stats.CachedTokens)
	require.Equal(t, "", stats.CachedTokenRateMode)
}

func TestInvalidateChannelAffinityForChannel(t *testing.T) {
	require.NoError(t, ResetChannelAffinityCacheForTest())
	t.Cleanup(func() { require.NoError(t, ResetChannelAffinityCacheForTest()) })

	require.NoError(t, RecordPreferredChannel("test:affinity:one", 91, 600))
	require.NoError(t, RecordPreferredChannel("test:affinity:two", 91, 600))

	require.Equal(t, 2, InvalidateChannelAffinityForChannel(91))
	_, found, err := GetPreferredChannel("test:affinity:one")
	require.NoError(t, err)
	require.False(t, found)
	_, found, err = GetPreferredChannel("test:affinity:two")
	require.NoError(t, err)
	require.False(t, found)
}
