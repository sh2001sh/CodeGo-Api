package app

import (
	gatewaydomain "github.com/sh2001sh/CodeGo-Api/internal/gateway/domain"
	gatewayschema "github.com/sh2001sh/CodeGo-Api/internal/gateway/schema"
	gatewaystore "github.com/sh2001sh/CodeGo-Api/internal/gateway/store"
	platformruntime "github.com/sh2001sh/CodeGo-Api/internal/platform/runtime"
	"slices"
	"strings"
)

// ApplyChannelUpstreamModelUpdates applies selected upstream model changes to one channel.
func ApplyChannelUpstreamModelUpdates(channelID int, addModels []string, ignoreModels []string, removeModels []string) (*ApplyChannelUpstreamModelUpdatesResult, error) {
	channel, err := gatewaystore.LoadChannelByID(channelID, true)
	if err != nil {
		return nil, err
	}
	beforeSettings := gatewaydomain.GetOtherSettings(channel)
	ignoredModelsResult := intersectModelNames(ignoreModels, beforeSettings.UpstreamModelUpdateLastDetectedModels)

	addedModels, removedModels, remainingModels, remainingRemoveModels, modelsChanged, err := applyChannelUpstreamModelUpdates(
		channel,
		addModels,
		ignoreModels,
		removeModels,
	)
	if err != nil {
		return nil, err
	}
	if modelsChanged {
		refreshChannelRuntimeCache()
	}
	return &ApplyChannelUpstreamModelUpdatesResult{
		ChannelID:             channel.Id,
		ChannelName:           channel.Name,
		AddedModels:           addedModels,
		RemovedModels:         removedModels,
		IgnoredModels:         ignoredModelsResult,
		RemainingModels:       remainingModels,
		RemainingRemoveModels: remainingRemoveModels,
		Models:                channel.Models,
		Settings:              channel.OtherSettings,
	}, nil
}

// DetectChannelUpstreamModelUpdates fetches and persists pending upstream model changes for one channel.
func DetectChannelUpstreamModelUpdates(channelID int) (*DetectChannelUpstreamModelUpdatesResult, error) {
	channel, err := gatewaystore.LoadChannelByID(channelID, true)
	if err != nil {
		return nil, err
	}

	settings := gatewaydomain.GetOtherSettings(channel)
	modelsChanged, autoAdded, err := checkAndPersistChannelUpstreamModelUpdates(channel, &settings, true, false)
	if err != nil {
		return nil, err
	}
	if modelsChanged {
		refreshChannelRuntimeCache()
	}
	return &DetectChannelUpstreamModelUpdatesResult{
		ChannelID:       channel.Id,
		ChannelName:     channel.Name,
		AddModels:       normalizeModelNames(settings.UpstreamModelUpdateLastDetectedModels),
		RemoveModels:    normalizeModelNames(settings.UpstreamModelUpdateLastRemovedModels),
		LastCheckTime:   settings.UpstreamModelUpdateLastCheckTime,
		AutoAddedModels: autoAdded,
	}, nil
}

func applyChannelUpstreamModelUpdates(channel *gatewayschema.Channel, addModelsInput []string, ignoreModelsInput []string, removeModelsInput []string) (addedModels []string, removedModels []string, remainingModels []string, remainingRemoveModels []string, modelsChanged bool, err error) {
	settings := gatewaydomain.GetOtherSettings(channel)
	pendingAddModels := normalizeModelNames(settings.UpstreamModelUpdateLastDetectedModels)
	pendingRemoveModels := normalizeModelNames(settings.UpstreamModelUpdateLastRemovedModels)
	addModels := intersectModelNames(addModelsInput, pendingAddModels)
	ignoreModels := intersectModelNames(ignoreModelsInput, pendingAddModels)
	removeModels := intersectModelNames(removeModelsInput, pendingRemoveModels)
	removeModels = subtractModelNames(removeModels, addModels)

	originModels := normalizeModelNames(channel.GetModels())
	nextModels := applySelectedModelChanges(originModels, addModels, removeModels)
	modelsChanged = !slices.Equal(originModels, nextModels)
	if modelsChanged {
		channel.Models = strings.Join(nextModels, ",")
	}

	settings.UpstreamModelUpdateIgnoredModels = mergeModelNames(settings.UpstreamModelUpdateIgnoredModels, ignoreModels)
	if len(addModels) > 0 {
		settings.UpstreamModelUpdateIgnoredModels = subtractModelNames(settings.UpstreamModelUpdateIgnoredModels, addModels)
	}
	remainingModels = subtractModelNames(pendingAddModels, append(addModels, ignoreModels...))
	remainingRemoveModels = subtractModelNames(pendingRemoveModels, removeModels)
	settings.UpstreamModelUpdateLastDetectedModels = remainingModels
	settings.UpstreamModelUpdateLastRemovedModels = remainingRemoveModels
	settings.UpstreamModelUpdateLastCheckTime = platformruntime.GetTimestamp()

	if err := updateChannelUpstreamModelSettings(channel, settings, modelsChanged); err != nil {
		return nil, nil, nil, nil, false, err
	}
	if modelsChanged {
		if err := gatewaystore.UpdateChannelAbilities(channel, nil); err != nil {
			return addModels, removeModels, remainingModels, remainingRemoveModels, true, err
		}
	}
	return addModels, removeModels, remainingModels, remainingRemoveModels, modelsChanged, nil
}

// ApplyAllChannelUpstreamModelUpdates applies all pending upstream model changes for enabled channels.
func ApplyAllChannelUpstreamModelUpdates() (*ApplyAllChannelUpstreamModelUpdatesSummary, error) {
	results := make([]ApplyAllChannelUpstreamModelUpdatesResult, 0)
	failed := make([]int, 0)
	refreshNeeded := false
	addedModelCount := 0
	removedModelCount := 0

	lastID := 0
	for {
		channels, err := findEnabledChannelsAfterID(lastID, ChannelUpstreamModelUpdateTaskBatchSize)
		if err != nil {
			return nil, err
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

			pendingAddModels, pendingRemoveModels := collectPendingApplyUpstreamModelChanges(settings)
			if len(pendingAddModels) == 0 && len(pendingRemoveModels) == 0 {
				continue
			}

			addedModels, removedModels, remainingModels, remainingRemoveModels, modelsChanged, err := applyChannelUpstreamModelUpdates(
				channel,
				pendingAddModels,
				nil,
				pendingRemoveModels,
			)
			if err != nil {
				failed = append(failed, channel.Id)
				continue
			}
			if modelsChanged {
				refreshNeeded = true
			}
			addedModelCount += len(addedModels)
			removedModelCount += len(removedModels)
			results = append(results, ApplyAllChannelUpstreamModelUpdatesResult{
				ChannelID:             channel.Id,
				ChannelName:           channel.Name,
				AddedModels:           addedModels,
				RemovedModels:         removedModels,
				RemainingModels:       remainingModels,
				RemainingRemoveModels: remainingRemoveModels,
			})
		}

		if len(channels) < ChannelUpstreamModelUpdateTaskBatchSize {
			break
		}
	}

	if refreshNeeded {
		refreshChannelRuntimeCache()
	}
	return &ApplyAllChannelUpstreamModelUpdatesSummary{
		ProcessedChannels: len(results),
		AddedModels:       addedModelCount,
		RemovedModels:     removedModelCount,
		FailedChannelIDs:  failed,
		Results:           results,
	}, nil
}

// DetectAllChannelUpstreamModelUpdates detects pending upstream model changes for enabled channels.
func DetectAllChannelUpstreamModelUpdates() (*DetectAllChannelUpstreamModelUpdatesSummary, error) {
	results := make([]DetectChannelUpstreamModelUpdatesResult, 0)
	failed := make([]int, 0)
	detectedAddCount := 0
	detectedRemoveCount := 0
	refreshNeeded := false

	lastID := 0
	for {
		channels, err := findEnabledChannelsAfterID(lastID, ChannelUpstreamModelUpdateTaskBatchSize)
		if err != nil {
			return nil, err
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

			modelsChanged, autoAdded, err := checkAndPersistChannelUpstreamModelUpdates(channel, &settings, true, false)
			if err != nil {
				failed = append(failed, channel.Id)
				continue
			}
			if modelsChanged {
				refreshNeeded = true
			}

			addModels := normalizeModelNames(settings.UpstreamModelUpdateLastDetectedModels)
			removeModels := normalizeModelNames(settings.UpstreamModelUpdateLastRemovedModels)
			detectedAddCount += len(addModels)
			detectedRemoveCount += len(removeModels)
			results = append(results, DetectChannelUpstreamModelUpdatesResult{
				ChannelID:       channel.Id,
				ChannelName:     channel.Name,
				AddModels:       addModels,
				RemoveModels:    removeModels,
				LastCheckTime:   settings.UpstreamModelUpdateLastCheckTime,
				AutoAddedModels: autoAdded,
			})
		}

		if len(channels) < ChannelUpstreamModelUpdateTaskBatchSize {
			break
		}
	}

	if refreshNeeded {
		refreshChannelRuntimeCache()
	}
	return &DetectAllChannelUpstreamModelUpdatesSummary{
		ProcessedChannels:      len(results),
		FailedChannelIDs:       failed,
		DetectedAddModels:      detectedAddCount,
		DetectedRemoveModels:   detectedRemoveCount,
		ChannelDetectedResults: results,
	}, nil
}
