package app

import (
	"fmt"
	"github.com/samber/lo"
	"github.com/sh2001sh/CodeGo-Api/dto"
	gatewaydomain "github.com/sh2001sh/CodeGo-Api/internal/gateway/domain"
	gatewayschema "github.com/sh2001sh/CodeGo-Api/internal/gateway/schema"
	gatewaystore "github.com/sh2001sh/CodeGo-Api/internal/gateway/store"
	platformconfig "github.com/sh2001sh/CodeGo-Api/internal/platform/config"
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
	platformhttpx "github.com/sh2001sh/CodeGo-Api/internal/platform/httpx"
	"github.com/sh2001sh/CodeGo-Api/internal/platform/notifyx"
	platformobservability "github.com/sh2001sh/CodeGo-Api/internal/platform/observability"
	platformruntime "github.com/sh2001sh/CodeGo-Api/internal/platform/runtime"
	"strings"
	"time"
)

func updateChannelUpstreamModelSettings(channel *gatewayschema.Channel, settings dto.ChannelOtherSettings, updateModels bool) error {
	gatewaydomain.SetOtherSettings(channel, settings)
	updates := map[string]interface{}{
		"settings": channel.OtherSettings,
	}
	if updateModels {
		updates["models"] = channel.Models
	}
	return platformdb.DB.Model(&gatewayschema.Channel{}).Where("id = ?", channel.Id).Updates(updates).Error
}

func checkAndPersistChannelUpstreamModelUpdates(channel *gatewayschema.Channel, settings *dto.ChannelOtherSettings, force bool, allowAutoApply bool) (modelsChanged bool, autoAdded int, err error) {
	now := platformruntime.GetTimestamp()
	if !force {
		minInterval := getUpstreamModelUpdateMinCheckIntervalSeconds()
		if settings.UpstreamModelUpdateLastCheckTime > 0 &&
			now-settings.UpstreamModelUpdateLastCheckTime < minInterval {
			return false, 0, nil
		}
	}

	pendingAddModels, pendingRemoveModels, fetchErr := collectPendingUpstreamModelChanges(channel, *settings)
	settings.UpstreamModelUpdateLastCheckTime = now
	if fetchErr != nil {
		if err = updateChannelUpstreamModelSettings(channel, *settings, false); err != nil {
			return false, 0, err
		}
		return false, 0, fetchErr
	}

	if allowAutoApply && settings.UpstreamModelUpdateAutoSyncEnabled && len(pendingAddModels) > 0 {
		originModels := normalizeModelNames(channel.GetModels())
		mergedModels := mergeModelNames(originModels, pendingAddModels)
		if len(mergedModels) > len(originModels) {
			channel.Models = strings.Join(mergedModels, ",")
			autoAdded = len(mergedModels) - len(originModels)
			modelsChanged = true
		}
		settings.UpstreamModelUpdateLastDetectedModels = []string{}
	} else {
		settings.UpstreamModelUpdateLastDetectedModels = pendingAddModels
	}
	settings.UpstreamModelUpdateLastRemovedModels = pendingRemoveModels

	if err = updateChannelUpstreamModelSettings(channel, *settings, modelsChanged); err != nil {
		return false, autoAdded, err
	}
	if modelsChanged {
		if err = gatewaystore.UpdateChannelAbilities(channel, nil); err != nil {
			return true, autoAdded, err
		}
	}
	return modelsChanged, autoAdded, nil
}

func refreshChannelRuntimeCache() {
	if platformconfig.MemoryCacheEnabled {
		func() {
			defer func() {
				if r := recover(); r != nil {
					platformobservability.SysLog(fmt.Sprintf("InitChannelCache panic: %v", r))
				}
			}()
			gatewaystore.InitChannelCache()
		}()
	}
	platformhttpx.ResetProxyClientCache()
}

func shouldSendUpstreamModelUpdateNotification(now int64, changedChannels int, failedChannels int) bool {
	if changedChannels <= 0 && failedChannels <= 0 {
		return true
	}

	channelUpstreamModelUpdateNotifyState.Lock()
	defer channelUpstreamModelUpdateNotifyState.Unlock()

	if channelUpstreamModelUpdateNotifyState.lastNotifiedAt > 0 &&
		now-channelUpstreamModelUpdateNotifyState.lastNotifiedAt < ChannelUpstreamModelUpdateNotifySuppressWindowSeconds &&
		channelUpstreamModelUpdateNotifyState.lastChangedChannels == changedChannels &&
		channelUpstreamModelUpdateNotifyState.lastFailedChannels == failedChannels {
		return false
	}

	channelUpstreamModelUpdateNotifyState.lastNotifiedAt = now
	channelUpstreamModelUpdateNotifyState.lastChangedChannels = changedChannels
	channelUpstreamModelUpdateNotifyState.lastFailedChannels = failedChannels
	return true
}

func buildUpstreamModelUpdateTaskNotificationContent(
	checkedChannels int,
	changedChannels int,
	detectedAddModels int,
	detectedRemoveModels int,
	autoAddedModels int,
	failedChannelIDs []int,
	channelSummaries []upstreamModelUpdateChannelSummary,
	addModelSamples []string,
	removeModelSamples []string,
) string {
	var builder strings.Builder
	failedChannels := len(failedChannelIDs)
	builder.WriteString(fmt.Sprintf(
		"上游模型巡检摘要：检测渠道 %d 个，发现变更 %d 个，新增 %d 个，删除 %d 个，自动同步新增 %d 个，失败 %d 个。",
		checkedChannels,
		changedChannels,
		detectedAddModels,
		detectedRemoveModels,
		autoAddedModels,
		failedChannels,
	))

	if len(channelSummaries) > 0 {
		displayCount := min(len(channelSummaries), ChannelUpstreamModelUpdateNotifyMaxChannelDetails)
		builder.WriteString(fmt.Sprintf("\n\n变更渠道明细（展示 %d/%d）：", displayCount, len(channelSummaries)))
		for _, summary := range channelSummaries[:displayCount] {
			builder.WriteString(fmt.Sprintf("\n- %s (+%d / -%d)", summary.ChannelName, summary.AddCount, summary.RemoveCount))
		}
		if len(channelSummaries) > displayCount {
			builder.WriteString(fmt.Sprintf("\n- 其余 %d 个渠道已省略", len(channelSummaries)-displayCount))
		}
	}

	normalizedAddModelSamples := normalizeModelNames(addModelSamples)
	if len(normalizedAddModelSamples) > 0 {
		displayCount := min(len(normalizedAddModelSamples), ChannelUpstreamModelUpdateNotifyMaxModelDetails)
		builder.WriteString(fmt.Sprintf("\n\n新增模型示例（展示 %d/%d）：%s",
			displayCount,
			len(normalizedAddModelSamples),
			strings.Join(normalizedAddModelSamples[:displayCount], ", "),
		))
		if len(normalizedAddModelSamples) > displayCount {
			builder.WriteString(fmt.Sprintf("（其余 %d 个已省略）", len(normalizedAddModelSamples)-displayCount))
		}
	}

	normalizedRemoveModelSamples := normalizeModelNames(removeModelSamples)
	if len(normalizedRemoveModelSamples) > 0 {
		displayCount := min(len(normalizedRemoveModelSamples), ChannelUpstreamModelUpdateNotifyMaxModelDetails)
		builder.WriteString(fmt.Sprintf("\n\n删除模型示例（展示 %d/%d）：%s",
			displayCount,
			len(normalizedRemoveModelSamples),
			strings.Join(normalizedRemoveModelSamples[:displayCount], ", "),
		))
		if len(normalizedRemoveModelSamples) > displayCount {
			builder.WriteString(fmt.Sprintf("（其余 %d 个已省略）", len(normalizedRemoveModelSamples)-displayCount))
		}
	}

	if failedChannels > 0 {
		displayCount := min(failedChannels, ChannelUpstreamModelUpdateNotifyMaxFailedChannelIDs)
		displayIDs := lo.Map(failedChannelIDs[:displayCount], func(channelID int, _ int) string {
			return fmt.Sprintf("%d", channelID)
		})
		builder.WriteString(fmt.Sprintf(
			"\n\n失败渠道 ID（展示 %d/%d）：%s",
			displayCount,
			failedChannels,
			strings.Join(displayIDs, ", "),
		))
		if failedChannels > displayCount {
			builder.WriteString(fmt.Sprintf("（其余 %d 个已省略）", failedChannels-displayCount))
		}
	}
	return builder.String()
}

func runChannelUpstreamModelUpdateTaskOnce() {
	if !channelUpstreamModelUpdateTaskRunning.CompareAndSwap(false, true) {
		return
	}
	defer channelUpstreamModelUpdateTaskRunning.Store(false)

	checkedChannels := 0
	failedChannels := 0
	failedChannelIDs := make([]int, 0)
	changedChannels := 0
	detectedAddModels := 0
	detectedRemoveModels := 0
	autoAddedModels := 0
	channelSummaries := make([]upstreamModelUpdateChannelSummary, 0)
	addModelSamples := make([]string, 0)
	removeModelSamples := make([]string, 0)
	refreshNeeded := false

	lastID := 0
	for {
		channels, err := findEnabledChannelsAfterID(lastID, ChannelUpstreamModelUpdateTaskBatchSize)
		if err != nil {
			platformobservability.SysLog(fmt.Sprintf("upstream model update task query failed: %v", err))
			break
		}
		if len(channels) == 0 {
			break
		}
		lastID = channels[len(channels)-1].Id

		for _, channel := range channels {
			if channel == nil {
				continue
			}
			settings := gatewaydomain.GetOtherSettings(channel)
			if !settings.UpstreamModelUpdateCheckEnabled {
				continue
			}

			checkedChannels++
			modelsChanged, autoAdded, err := checkAndPersistChannelUpstreamModelUpdates(channel, &settings, false, true)
			if err != nil {
				failedChannels++
				failedChannelIDs = append(failedChannelIDs, channel.Id)
				platformobservability.SysLog(fmt.Sprintf("upstream model update check failed: channel_id=%d channel_name=%s err=%v", channel.Id, channel.Name, err))
				continue
			}
			currentAddModels := normalizeModelNames(settings.UpstreamModelUpdateLastDetectedModels)
			currentRemoveModels := normalizeModelNames(settings.UpstreamModelUpdateLastRemovedModels)
			currentAddCount := len(currentAddModels) + autoAdded
			currentRemoveCount := len(currentRemoveModels)
			detectedAddModels += currentAddCount
			detectedRemoveModels += currentRemoveCount
			if currentAddCount > 0 || currentRemoveCount > 0 {
				changedChannels++
				channelSummaries = append(channelSummaries, upstreamModelUpdateChannelSummary{
					ChannelName: channel.Name,
					AddCount:    currentAddCount,
					RemoveCount: currentRemoveCount,
				})
			}
			addModelSamples = mergeModelNames(addModelSamples, currentAddModels)
			removeModelSamples = mergeModelNames(removeModelSamples, currentRemoveModels)
			if modelsChanged {
				refreshNeeded = true
			}
			autoAddedModels += autoAdded

			if platformconfig.RequestInterval > 0 {
				time.Sleep(platformconfig.RequestInterval)
			}
		}

		if len(channels) < ChannelUpstreamModelUpdateTaskBatchSize {
			break
		}
	}

	if refreshNeeded {
		refreshChannelRuntimeCache()
	}

	if checkedChannels > 0 || platformconfig.DebugEnabled {
		platformobservability.SysLog(fmt.Sprintf(
			"upstream model update task done: checked_channels=%d changed_channels=%d detected_add_models=%d detected_remove_models=%d failed_channels=%d auto_added_models=%d",
			checkedChannels,
			changedChannels,
			detectedAddModels,
			detectedRemoveModels,
			failedChannels,
			autoAddedModels,
		))
	}
	if changedChannels > 0 || failedChannels > 0 {
		now := platformruntime.GetTimestamp()
		if !shouldSendUpstreamModelUpdateNotification(now, changedChannels, failedChannels) {
			platformobservability.SysLog(fmt.Sprintf(
				"upstream model update notification skipped in 24h window: changed_channels=%d failed_channels=%d",
				changedChannels,
				failedChannels,
			))
			return
		}
		notifyx.NotifyUpstreamModelUpdateWatchers(
			"上游模型巡检通知",
			buildUpstreamModelUpdateTaskNotificationContent(
				checkedChannels,
				changedChannels,
				detectedAddModels,
				detectedRemoveModels,
				autoAddedModels,
				failedChannelIDs,
				channelSummaries,
				addModelSamples,
				removeModelSamples,
			),
		)
	}
}

// StartChannelUpstreamModelUpdateTask starts the background upstream model update task once.
func StartChannelUpstreamModelUpdateTask() {
	channelUpstreamModelUpdateTaskOnce.Do(func() {
		if !platformconfig.IsMasterNode {
			return
		}
		if !platformconfig.GetEnvOrDefaultBool("CHANNEL_UPSTREAM_MODEL_UPDATE_TASK_ENABLED", true) {
			platformobservability.SysLog("upstream model update task disabled by CHANNEL_UPSTREAM_MODEL_UPDATE_TASK_ENABLED")
			return
		}

		intervalMinutes := platformconfig.GetEnvOrDefaultInt(
			"CHANNEL_UPSTREAM_MODEL_UPDATE_TASK_INTERVAL_MINUTES",
			ChannelUpstreamModelUpdateTaskDefaultIntervalMinutes,
		)
		if intervalMinutes < 1 {
			intervalMinutes = ChannelUpstreamModelUpdateTaskDefaultIntervalMinutes
		}
		interval := time.Duration(intervalMinutes) * time.Minute

		go func() {
			platformobservability.SysLog(fmt.Sprintf("upstream model update task started: interval=%s", interval))
			runChannelUpstreamModelUpdateTaskOnce()
			ticker := time.NewTicker(interval)
			defer ticker.Stop()
			for range ticker.C {
				runChannelUpstreamModelUpdateTaskOnce()
			}
		}()
	})
}
