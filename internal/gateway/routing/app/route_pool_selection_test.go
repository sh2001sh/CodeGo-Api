package app

import (
	"testing"

	gatewayruntime "github.com/sh2001sh/CodeGo-Api/internal/gateway/runtime"
	gatewayschema "github.com/sh2001sh/CodeGo-Api/internal/gateway/schema"
	"github.com/stretchr/testify/assert"
)

func TestEffectiveRoutePoolCostPrefersStableChannelOverCheapUnstableChannel(t *testing.T) {
	cheapButUnstable := effectiveRoutePoolCost(gatewayschema.RoutePoolMember{CostMultiplier: 1}, "gpt-test", gatewayruntime.ChannelHealth{
		Window5Requests: 20, SuccessRate5m: 90, State: gatewayruntime.ChannelHealthDegraded, ConsecutiveRetryableFailures: 2,
	})
	stableBackup := effectiveRoutePoolCost(gatewayschema.RoutePoolMember{CostMultiplier: 1.4}, "gpt-test", gatewayruntime.ChannelHealth{
		Window5Requests: 20, SuccessRate5m: 99, State: gatewayruntime.ChannelHealthHealthy,
	})
	assert.Greater(t, cheapButUnstable, stableBackup)
}

func TestRoutePoolModelCostOverridesMemberDefault(t *testing.T) {
	member := gatewayschema.RoutePoolMember{CostMultiplier: 1.2, ModelCostOverrides: `{"gpt-test":0.8}`}
	assert.Equal(t, 0.8, routePoolModelCost(member, "gpt-test"))
	assert.Equal(t, 1.2, routePoolModelCost(member, "other"))
}

func TestRoutePoolConservativeSuccessRatePenalizesSmallFailureSamples(t *testing.T) {
	assert.Greater(t, 98.0, routePoolConservativeSuccessRate(gatewayruntime.ChannelHealth{
		Window5Requests:  20,
		Window5Successes: 19,
		SuccessRate5m:    95,
	}))
	assert.InDelta(t, 99.0, routePoolConservativeSuccessRate(gatewayruntime.ChannelHealth{
		SuccessRate5m: 99,
	}), 0.001)
	assert.Greater(t, routePoolConservativeSuccessRate(gatewayruntime.ChannelHealth{
		Window5Requests:  20,
		Window5Successes: 20,
		SuccessRate5m:    100,
	}), 95.0)
}

func TestRoutePoolHysteresisKeepsStickyChannelForSmallImprovement(t *testing.T) {
	sticky := scoredRoutePoolCandidate{score: 1}
	nearby := scoredRoutePoolCandidate{score: 0.9}
	assert.False(t, nearby.score <= sticky.score*(1-routePoolSwitchImprovement))

	clearlyBetter := scoredRoutePoolCandidate{score: 0.84}
	assert.True(t, clearlyBetter.score <= sticky.score*(1-routePoolSwitchImprovement))
}

func TestEffectiveRoutePoolCostStronglyPenalizesPoorReliability(t *testing.T) {
	poor := effectiveRoutePoolCost(gatewayschema.RoutePoolMember{CostMultiplier: 1}, "gpt-test", gatewayruntime.ChannelHealth{
		Window5Requests: 20, Window5Successes: 18, SuccessRate5m: 90,
	})
	stable := effectiveRoutePoolCost(gatewayschema.RoutePoolMember{CostMultiplier: 1.5}, "gpt-test", gatewayruntime.ChannelHealth{
		Window5Requests: 100, Window5Successes: 100, SuccessRate5m: 100,
	})
	assert.Greater(t, poor, stable)
}

func TestRoutePoolReliabilityPenaltyDifferentiatesUnstableChannels(t *testing.T) {
	assert.Greater(t, routePoolReliabilityPenalty(60), routePoolReliabilityPenalty(75))
	assert.Greater(t, routePoolReliabilityPenalty(75), routePoolReliabilityPenalty(89))
	assert.Greater(t, routePoolReliabilityPenalty(89), routePoolReliabilityPenalty(95))
}
