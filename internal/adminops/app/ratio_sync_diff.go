package app

import "github.com/sh2001sh/CodeGo-Api/dto"

func buildDifferences(localData map[string]any, successfulChannels []ratioSyncChannelData) map[string]map[string]dto.DifferenceItem {
	differences := make(map[string]map[string]dto.DifferenceItem)
	allModels := make(map[string]struct{})

	for _, field := range pricingSyncFields {
		for modelName := range valueMap(localData[field]) {
			allModels[modelName] = struct{}{}
		}
	}
	for _, channel := range successfulChannels {
		for _, field := range pricingSyncFields {
			for modelName := range valueMap(channel.data[field]) {
				allModels[modelName] = struct{}{}
			}
		}
	}

	confidenceMap := make(map[string]map[string]bool)
	for _, channel := range successfulChannels {
		confidenceMap[channel.name] = make(map[string]bool)
		modelRatios := valueMap(channel.data["model_ratio"])
		completionRatios := valueMap(channel.data["completion_ratio"])

		if len(modelRatios) > 0 && len(completionRatios) > 0 {
			for modelName := range allModels {
				confidenceMap[channel.name][modelName] = true
				modelRatioVal, hasModelRatio := modelRatios[modelName]
				completionRatioVal, hasCompletionRatio := completionRatios[modelName]
				if !hasModelRatio || !hasCompletionRatio {
					continue
				}
				modelRatioFloat, modelRatioOK := asFloat64(modelRatioVal)
				completionRatioFloat, completionRatioOK := asFloat64(completionRatioVal)
				if modelRatioOK && completionRatioOK && nearlyEqual(modelRatioFloat, 37.5) && nearlyEqual(completionRatioFloat, 1.0) {
					confidenceMap[channel.name][modelName] = false
				}
			}
			continue
		}

		for modelName := range allModels {
			confidenceMap[channel.name][modelName] = true
		}
	}

	for modelName := range allModels {
		for _, ratioType := range pricingSyncFields {
			var localValue any
			if value, exists := valueMap(localData[ratioType])[modelName]; exists {
				localValue = normalizeSyncValue(ratioType, value)
			}

			upstreamValues := make(map[string]any)
			confidenceValues := make(map[string]bool)
			hasUpstreamValue := false
			hasDifference := false

			for _, channel := range successfulChannels {
				var upstreamValue any
				if value, exists := valueMap(channel.data[ratioType])[modelName]; exists {
					upstreamValue = normalizeSyncValue(ratioType, value)
					hasUpstreamValue = true
					if localValue != nil && !valuesEqual(localValue, upstreamValue) {
						hasDifference = true
					} else if valuesEqual(localValue, upstreamValue) {
						upstreamValue = "same"
					}
				}
				if upstreamValue == nil && localValue == nil {
					upstreamValue = "same"
				}
				if localValue == nil && upstreamValue != nil && upstreamValue != "same" {
					hasDifference = true
				}

				upstreamValues[channel.name] = upstreamValue
				confidenceValues[channel.name] = confidenceMap[channel.name][modelName]
			}

			shouldInclude := false
			if localValue != nil {
				shouldInclude = hasDifference
			} else {
				shouldInclude = hasUpstreamValue
			}
			if !shouldInclude {
				continue
			}
			if differences[modelName] == nil {
				differences[modelName] = make(map[string]dto.DifferenceItem)
			}
			differences[modelName][ratioType] = dto.DifferenceItem{
				Current:    localValue,
				Upstreams:  upstreamValues,
				Confidence: confidenceValues,
			}
		}
	}

	channelHasDiff := make(map[string]bool)
	for _, ratioMap := range differences {
		for _, item := range ratioMap {
			for channelName, value := range item.Upstreams {
				if value != nil && value != "same" {
					channelHasDiff[channelName] = true
				}
			}
		}
	}

	for modelName, ratioMap := range differences {
		for ratioType, item := range ratioMap {
			for channelName := range item.Upstreams {
				if !channelHasDiff[channelName] {
					delete(item.Upstreams, channelName)
					delete(item.Confidence, channelName)
				}
			}
			allSame := true
			for _, value := range item.Upstreams {
				if value != "same" {
					allSame = false
					break
				}
			}
			if len(item.Upstreams) == 0 || allSame {
				delete(ratioMap, ratioType)
				continue
			}
			differences[modelName][ratioType] = item
		}
		if len(ratioMap) == 0 {
			delete(differences, modelName)
		}
	}

	return differences
}
