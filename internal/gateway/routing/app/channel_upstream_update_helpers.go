package app

import (
	"github.com/samber/lo"
	"github.com/sh2001sh/CodeGo-Api/constant"
	"github.com/sh2001sh/CodeGo-Api/dto"
	gatewayschema "github.com/sh2001sh/CodeGo-Api/internal/gateway/schema"
	platformconfig "github.com/sh2001sh/CodeGo-Api/internal/platform/config"
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
	platformencoding "github.com/sh2001sh/CodeGo-Api/internal/platform/encodingx"
	"regexp"
	"strings"
)

func mergeModelNames(base []string, appended []string) []string {
	merged := normalizeModelNames(base)
	seen := make(map[string]struct{}, len(merged))
	for _, modelName := range merged {
		seen[modelName] = struct{}{}
	}
	for _, modelName := range normalizeModelNames(appended) {
		if _, ok := seen[modelName]; ok {
			continue
		}
		seen[modelName] = struct{}{}
		merged = append(merged, modelName)
	}
	return merged
}

func subtractModelNames(base []string, removed []string) []string {
	removeSet := make(map[string]struct{}, len(removed))
	for _, modelName := range normalizeModelNames(removed) {
		removeSet[modelName] = struct{}{}
	}
	return lo.Filter(normalizeModelNames(base), func(modelName string, _ int) bool {
		_, ok := removeSet[modelName]
		return !ok
	})
}

func intersectModelNames(base []string, allowed []string) []string {
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, modelName := range normalizeModelNames(allowed) {
		allowedSet[modelName] = struct{}{}
	}
	return lo.Filter(normalizeModelNames(base), func(modelName string, _ int) bool {
		_, ok := allowedSet[modelName]
		return ok
	})
}

func applySelectedModelChanges(originModels []string, addModels []string, removeModels []string) []string {
	normalizedAdd := normalizeModelNames(addModels)
	normalizedRemove := subtractModelNames(normalizeModelNames(removeModels), normalizedAdd)
	return subtractModelNames(mergeModelNames(originModels, normalizedAdd), normalizedRemove)
}

func normalizeChannelModelMapping(channel *gatewayschema.Channel) map[string]string {
	if channel == nil || channel.ModelMapping == nil {
		return nil
	}
	rawMapping := strings.TrimSpace(*channel.ModelMapping)
	if rawMapping == "" || rawMapping == "{}" {
		return nil
	}
	parsed := make(map[string]string)
	if err := platformencoding.UnmarshalString(rawMapping, &parsed); err != nil {
		return nil
	}
	normalized := make(map[string]string, len(parsed))
	for source, target := range parsed {
		normalizedSource := strings.TrimSpace(source)
		normalizedTarget := strings.TrimSpace(target)
		if normalizedSource == "" || normalizedTarget == "" {
			continue
		}
		normalized[normalizedSource] = normalizedTarget
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

func collectPendingUpstreamModelChangesFromModels(
	localModels []string,
	upstreamModels []string,
	ignoredModels []string,
	modelMapping map[string]string,
) (pendingAddModels []string, pendingRemoveModels []string) {
	localSet := make(map[string]struct{})
	localModels = normalizeModelNames(localModels)
	upstreamModels = normalizeModelNames(upstreamModels)
	for _, modelName := range localModels {
		localSet[modelName] = struct{}{}
	}
	upstreamSet := make(map[string]struct{}, len(upstreamModels))
	for _, modelName := range upstreamModels {
		upstreamSet[modelName] = struct{}{}
	}

	normalizedIgnoredModels := normalizeModelNames(ignoredModels)
	redirectSourceSet := make(map[string]struct{}, len(modelMapping))
	redirectTargetSet := make(map[string]struct{}, len(modelMapping))
	for source, target := range modelMapping {
		redirectSourceSet[source] = struct{}{}
		redirectTargetSet[target] = struct{}{}
	}

	coveredUpstreamSet := make(map[string]struct{}, len(localSet)+len(redirectTargetSet))
	for modelName := range localSet {
		coveredUpstreamSet[modelName] = struct{}{}
	}
	for modelName := range redirectTargetSet {
		coveredUpstreamSet[modelName] = struct{}{}
	}

	pendingAdd := lo.Filter(upstreamModels, func(modelName string, _ int) bool {
		if _, ok := coveredUpstreamSet[modelName]; ok {
			return false
		}
		if lo.ContainsBy(normalizedIgnoredModels, func(ignoredModel string) bool {
			if regexBody, ok := strings.CutPrefix(ignoredModel, "regex:"); ok {
				matched, err := regexp.MatchString(strings.TrimSpace(regexBody), modelName)
				return err == nil && matched
			}
			return ignoredModel == modelName
		}) {
			return false
		}
		return true
	})
	pendingRemove := lo.Filter(localModels, func(modelName string, _ int) bool {
		if _, ok := redirectSourceSet[modelName]; ok {
			return false
		}
		_, ok := upstreamSet[modelName]
		return !ok
	})
	return normalizeModelNames(pendingAdd), normalizeModelNames(pendingRemove)
}

func collectPendingUpstreamModelChanges(channel *gatewayschema.Channel, settings dto.ChannelOtherSettings) (pendingAddModels []string, pendingRemoveModels []string, err error) {
	upstreamModels, err := FetchChannelUpstreamModelIDs(channel)
	if err != nil {
		return nil, nil, err
	}
	pendingAddModels, pendingRemoveModels = collectPendingUpstreamModelChangesFromModels(
		channel.GetModels(),
		upstreamModels,
		settings.UpstreamModelUpdateIgnoredModels,
		normalizeChannelModelMapping(channel),
	)
	return pendingAddModels, pendingRemoveModels, nil
}

func getUpstreamModelUpdateMinCheckIntervalSeconds() int64 {
	interval := int64(platformconfig.GetEnvOrDefaultInt(
		"CHANNEL_UPSTREAM_MODEL_UPDATE_MIN_CHECK_INTERVAL_SECONDS",
		ChannelUpstreamModelUpdateMinCheckIntervalSeconds,
	))
	if interval < 0 {
		return ChannelUpstreamModelUpdateMinCheckIntervalSeconds
	}
	return interval
}

func collectPendingApplyUpstreamModelChanges(settings dto.ChannelOtherSettings) (pendingAddModels []string, pendingRemoveModels []string) {
	return normalizeModelNames(settings.UpstreamModelUpdateLastDetectedModels), normalizeModelNames(settings.UpstreamModelUpdateLastRemovedModels)
}

func findEnabledChannelsAfterID(lastID int, batchSize int) ([]*gatewayschema.Channel, error) {
	var channels []*gatewayschema.Channel
	query := platformdb.DB.
		Select(ChannelUpstreamModelUpdateSelectFields).
		Where("status = ?", constant.ChannelStatusEnabled).
		Order("id asc").
		Limit(batchSize)
	if lastID > 0 {
		query = query.Where("id > ?", lastID)
	}
	return channels, query.Find(&channels).Error
}
