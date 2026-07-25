package store

import (
	"fmt"
	gatewayschema "github.com/sh2001sh/CodeGo-Api/internal/gateway/schema"
	platformobservability "github.com/sh2001sh/CodeGo-Api/internal/platform/observability"

	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
	"gorm.io/gorm"
)

// UpdateChannelUsedQuota increments one channel's aggregated billed quota.
func UpdateChannelUsedQuota(channelID int, quota int) {
	if err := platformdb.DB.Model(&gatewayschema.Channel{}).Where("id = ?", channelID).Update("used_quota", gorm.Expr("used_quota + ?", quota)).Error; err != nil {
		platformobservability.SysLog(fmt.Sprintf("failed to update channel used quota: channel_id=%d, delta_quota=%d, error=%v", channelID, quota, err))
	}
}
