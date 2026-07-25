package app

import gatewayruntime "github.com/sh2001sh/CodeGo-Api/internal/gateway/runtime"

func BuildChannelAffinityCacheStats() gatewayruntime.ChannelAffinityCacheStats {
	return gatewayruntime.GetChannelAffinityCacheStats()
}

func ClearChannelAffinityCache(all bool, ruleName string) (int, error) {
	if all {
		return gatewayruntime.ClearChannelAffinityCacheAll(), nil
	}
	return gatewayruntime.ClearChannelAffinityCacheByRuleName(ruleName)
}

func BuildChannelAffinityUsageCacheStats(ruleName, usingGroup, keyFingerprint string) gatewayruntime.ChannelAffinityUsageCacheStats {
	return gatewayruntime.GetChannelAffinityUsageCacheStats(ruleName, usingGroup, keyFingerprint)
}
