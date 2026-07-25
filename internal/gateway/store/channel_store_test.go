package store

import (
	"testing"

	"github.com/sh2001sh/CodeGo-Api/constant"
	gatewayschema "github.com/sh2001sh/CodeGo-Api/internal/gateway/schema"
	platformconfig "github.com/sh2001sh/CodeGo-Api/internal/platform/config"
	"github.com/stretchr/testify/require"
)

func TestCacheUpdateChannelStatus_RestoresEnabledChannelToRoutingCache(t *testing.T) {
	originalCacheEnabled := platformconfig.MemoryCacheEnabled
	originalGroups := group2model2channels
	originalChannels := channelsIDM
	t.Cleanup(func() {
		platformconfig.MemoryCacheEnabled = originalCacheEnabled
		group2model2channels = originalGroups
		channelsIDM = originalChannels
	})

	platformconfig.MemoryCacheEnabled = true
	group2model2channels = map[string]map[string][]int{
		"free": {"deepseek-v4-flash": {1, 2}},
	}
	priority := int64(10)
	channelsIDM = map[int]*gatewayschema.Channel{
		1: {Id: 1, Status: constant.ChannelStatusEnabled, Group: "free", Models: "deepseek-v4-flash", Priority: &priority},
		2: {Id: 2, Status: constant.ChannelStatusEnabled, Group: "free", Models: "deepseek-v4-flash"},
	}

	cacheUpdateChannelStatus(1, constant.ChannelStatusAutoDisabled)
	require.False(t, IsChannelEnabledForGroupModel("free", "deepseek-v4-flash", 1))

	cacheUpdateChannelStatus(1, constant.ChannelStatusEnabled)
	require.True(t, IsChannelEnabledForGroupModel("free", "deepseek-v4-flash", 1))
	require.Equal(t, []int{1, 2}, group2model2channels["free"]["deepseek-v4-flash"])

	cacheUpdateChannelStatus(1, constant.ChannelStatusEnabled)
	require.Equal(t, []int{1, 2}, group2model2channels["free"]["deepseek-v4-flash"])
}
