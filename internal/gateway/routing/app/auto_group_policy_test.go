package app

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	gatewaygroups "github.com/sh2001sh/CodeGo-Api/internal/gateway/groupsettings"
	gatewayruntime "github.com/sh2001sh/CodeGo-Api/internal/gateway/runtime"
	gatewayschema "github.com/sh2001sh/CodeGo-Api/internal/gateway/schema"
	gatewaystore "github.com/sh2001sh/CodeGo-Api/internal/gateway/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeTokenGroupDefaultsToAuto(t *testing.T) {
	assert.Equal(t, AutoGroupName, NormalizeTokenGroup(""))
	assert.Equal(t, AutoGroupName, NormalizeTokenGroup("  "))
	assert.Equal(t, "premium", NormalizeTokenGroup(" premium "))
}

func TestGetHealthySatisfiedChannelFallsBackAfterPrimaryCooldown(t *testing.T) {
	const modelName = "gpt-route-fallback-test"
	primaryPriority := int64(3)
	fallbackPriority := int64(2)
	originalSelector := selectRandomSatisfiedChannel
	t.Cleanup(func() {
		selectRandomSatisfiedChannel = originalSelector
		gatewayruntime.RecordChannelSuccess(42, modelName, 0)
	})

	var retries []int
	selectRandomSatisfiedChannel = func(_ string, _ string, retry int) (*gatewayschema.Channel, error) {
		retries = append(retries, retry)
		if retry == 0 {
			return &gatewayschema.Channel{Id: 42, Priority: &primaryPriority}, nil
		}
		return &gatewayschema.Channel{Id: 39, Priority: &fallbackPriority}, nil
	}
	gatewayruntime.CoolChannelModelForUpstreamFailure(42, modelName)

	channel, err := getHealthySatisfiedChannel("default", modelName, 0)
	require.NoError(t, err)
	require.NotNil(t, channel)
	require.Equal(t, 39, channel.Id)
	require.Contains(t, retries, 1)
}

func TestCacheSelectionSkipsCoolingChannelEvenWithLegacyProbeContext(t *testing.T) {
	const modelName = "gpt-route-no-user-probe-test"
	primaryPriority := int64(3)
	fallbackPriority := int64(2)
	originalSelector := selectRandomSatisfiedChannel
	t.Cleanup(func() {
		selectRandomSatisfiedChannel = originalSelector
		gatewayruntime.RecordChannelSuccess(42, modelName, 0)
	})

	selectRandomSatisfiedChannel = func(_ string, _ string, retry int) (*gatewayschema.Channel, error) {
		if retry == 0 {
			return &gatewayschema.Channel{Id: 42, Priority: &primaryPriority}, nil
		}
		return &gatewayschema.Channel{Id: 39, Priority: &fallbackPriority}, nil
	}
	gatewayruntime.CoolChannelModelForUpstreamFailure(42, modelName)

	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Set("model_probe_channel_id", 42)
	context.Set("model_probe_group", "default")
	channel, group, err := CacheGetRandomSatisfiedChannel(&RetryParam{
		Ctx:        context,
		TokenGroup: "default",
		ModelName:  modelName,
	})

	require.NoError(t, err)
	require.Equal(t, "default", group)
	require.NotNil(t, channel)
	require.Equal(t, 39, channel.Id)
}

func TestOrderAutoGroupsPrefersLowerRateUntilCooldown(t *testing.T) {
	originalAutoGroups := gatewaygroups.AutoGroups2JsonString()
	originalUsableGroups := gatewaygroups.UserUsableGroups2JSONString()
	originalRatios := gatewaystore.GroupRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, gatewaygroups.UpdateAutoGroupsByJsonString(originalAutoGroups))
		require.NoError(t, gatewaygroups.UpdateUserUsableGroupsByJSONString(originalUsableGroups))
		require.NoError(t, gatewaystore.UpdateGroupRatioByJSONString(originalRatios))
		require.NoError(t, resetAutoGroupCircuitCacheForTest())
	})

	require.NoError(t, gatewaygroups.UpdateAutoGroupsByJsonString(`["low","high"]`))
	require.NoError(t, gatewaygroups.UpdateUserUsableGroupsByJSONString(`{"low":"低费率","high":"高费率"}`))
	require.NoError(t, gatewaystore.UpdateGroupRatioByJSONString(`{"low":0.8,"high":1.2}`))

	assert.Equal(t, []string{"low", "high"}, OrderAutoGroups("default", "gpt-test"))
	for range autoGroupFailureThreshold {
		recordAutoGroupFailure("low", "gpt-test", time.Now())
	}
	assert.Equal(t, []string{"high", "low"}, OrderAutoGroups("default", "gpt-test"))

	recordAutoGroupSuccess("low", "gpt-test")
	assert.Equal(t, []string{"low", "high"}, OrderAutoGroups("default", "gpt-test"))
}
