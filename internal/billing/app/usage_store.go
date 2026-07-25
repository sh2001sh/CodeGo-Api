package app

import (
	gatewaystore "github.com/sh2001sh/CodeGo-Api/internal/gateway/store"
	identitystore "github.com/sh2001sh/CodeGo-Api/internal/identity/store"
)

func RecordUsageStats(userID int, channelID int, quota int) {
	if quota <= 0 {
		return
	}
	identitystore.UpdateUserUsedQuotaAndRequestCount(userID, quota)
	gatewaystore.UpdateChannelUsedQuota(channelID, quota)
}
