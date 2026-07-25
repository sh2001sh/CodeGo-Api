package app

import (
	"fmt"
	"github.com/sh2001sh/CodeGo-Api/constant"
	"github.com/sh2001sh/CodeGo-Api/dto"
	gatewaystore "github.com/sh2001sh/CodeGo-Api/internal/gateway/store"
	platformconfig "github.com/sh2001sh/CodeGo-Api/internal/platform/config"
	"github.com/sh2001sh/CodeGo-Api/internal/platform/notifyx"
	platformobservability "github.com/sh2001sh/CodeGo-Api/internal/platform/observability"
	platformops "github.com/sh2001sh/CodeGo-Api/internal/platform/opssettings"
	"github.com/sh2001sh/CodeGo-Api/types"
	"strings"
)

func formatNotifyType(channelID int, status int) string {
	return fmt.Sprintf("%s_%d_%d", dto.NotifyTypeChannelUpdate, channelID, status)
}

// DisableChannel auto-disables a channel and notifies the root user when enabled.
func DisableChannel(channelError types.ChannelError, reason string) {
	platformobservability.SysLog(fmt.Sprintf("通道「%s」（#%d）发生错误，准备禁用，原因：%s", channelError.ChannelName, channelError.ChannelId, reason))

	if !channelError.AutoBan {
		platformobservability.SysLog(fmt.Sprintf("通道「%s」（#%d）未启用自动禁用功能，跳过禁用操作", channelError.ChannelName, channelError.ChannelId))
		return
	}
	exclusive, err := gatewaystore.ChannelHasExclusiveEnabledAbility(channelError.ChannelId)
	if err != nil {
		platformobservability.SysError(fmt.Sprintf("检查通道「%s」（#%d）唯一模型路由失败，跳过自动禁用：%v", channelError.ChannelName, channelError.ChannelId, err))
		return
	}
	if exclusive {
		platformobservability.SysLog(fmt.Sprintf("通道「%s」（#%d）是至少一个分组模型的唯一可用渠道，跳过自动禁用", channelError.ChannelName, channelError.ChannelId))
		return
	}

	success := gatewaystore.UpdateChannelStatus(channelError.ChannelId, channelError.UsingKey, constant.ChannelStatusAutoDisabled, reason)
	if success {
		subject := fmt.Sprintf("通道「%s」（#%d）已被禁用", channelError.ChannelName, channelError.ChannelId)
		content := fmt.Sprintf("通道「%s」（#%d）已被禁用，原因：%s", channelError.ChannelName, channelError.ChannelId, reason)
		notifyx.NotifyRootUser(formatNotifyType(channelError.ChannelId, constant.ChannelStatusAutoDisabled), subject, content)
	}
}

// EnableChannel re-enables a previously auto-disabled channel and notifies the root user.
func EnableChannel(channelID int, usingKey string, channelName string) {
	success := gatewaystore.UpdateChannelStatus(channelID, usingKey, constant.ChannelStatusEnabled, "")
	if success {
		subject := fmt.Sprintf("通道「%s」（#%d）已被启用", channelName, channelID)
		content := fmt.Sprintf("通道「%s」（#%d）已被启用", channelName, channelID)
		notifyx.NotifyRootUser(formatNotifyType(channelID, constant.ChannelStatusEnabled), subject, content)
	}
}

// ShouldDisableChannel reports whether a channel failure should trigger auto-disable.
func ShouldDisableChannel(err *types.APIError) bool {
	if !platformconfig.AutomaticDisableChannelEnabled || err == nil {
		return false
	}
	if types.IsChannelError(err) {
		return true
	}
	if types.IsSkipRetryError(err) {
		return false
	}
	if gatewaystore.ShouldDisableByStatusCode(err.StatusCode) {
		return true
	}
	return containsDisableKeyword(err.Error(), platformops.GetAutomaticDisableKeywords())
}

// ShouldEnableChannel reports whether a channel should be auto-enabled again.
func ShouldEnableChannel(apiError *types.APIError, status int) bool {
	if !platformconfig.AutomaticEnableChannelEnabled {
		return false
	}
	if apiError != nil {
		return false
	}
	if status != constant.ChannelStatusAutoDisabled {
		return false
	}
	return true
}

func containsDisableKeyword(message string, keywords []string) bool {
	lowerMessage := strings.ToLower(message)
	for _, keyword := range keywords {
		keyword = strings.ToLower(strings.TrimSpace(keyword))
		if keyword != "" && strings.Contains(lowerMessage, keyword) {
			return true
		}
	}
	return false
}
