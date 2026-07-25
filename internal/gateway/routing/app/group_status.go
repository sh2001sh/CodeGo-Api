package app

import (
	"math"
	"sort"
	"strings"
	"time"

	auditprojection "github.com/sh2001sh/CodeGo-Api/internal/audit/projection"
	gatewaydomain "github.com/sh2001sh/CodeGo-Api/internal/gateway/domain"
	gatewaystore "github.com/sh2001sh/CodeGo-Api/internal/gateway/store"
	identitystore "github.com/sh2001sh/CodeGo-Api/internal/identity/store"
)

type UserGroupStatusBucket struct {
	Ts           int64    `json:"ts"`
	SuccessRate  *float64 `json:"success_rate"`
	RequestCount int64    `json:"request_count"`
}

type UserGroupModelStatusItem struct {
	Model         string                  `json:"model"`
	Status        string                  `json:"status"`
	SuccessRate   *float64                `json:"success_rate"`
	SampleHours   float64                 `json:"sample_window"`
	SeriesWindow  float64                 `json:"series_window"`
	BucketSeconds int64                   `json:"bucket_seconds"`
	RequestCount  int64                   `json:"request_count"`
	Series        []UserGroupStatusBucket `json:"series"`
}

type UserGroupStatusItem struct {
	Group        string                     `json:"group"`
	Status       string                     `json:"status"`
	RequestCount int64                      `json:"request_count"`
	Models       []UserGroupModelStatusItem `json:"models"`
}

func BuildUserGroupStatus(userID int, hasUser bool) ([]UserGroupStatusItem, error) {
	const successSampleMinutes = 30
	const successSegmentCount = 1
	const timelineSampleMinutes = 24 * 60
	const timelineSegmentCount = 48

	pricing := loadGatewayPricing()
	groupNames, err := resolveVisibleGroupStatusGroups(userID, hasUser, pricing)
	if err != nil {
		return nil, err
	}

	groupSummaries := buildPricingGroupModelSummaries(pricing, groupNames)
	successRates, _, requestCounts, sampleWindowHours, _ := queryGroupModelRecentHealth(groupNames, successSampleMinutes, successSegmentCount)
	_, seriesByModel, _, seriesWindowHours, bucketSeconds := queryGroupModelRecentHealth(groupNames, timelineSampleMinutes, timelineSegmentCount)

	result := make([]UserGroupStatusItem, 0, len(groupNames))
	for _, groupName := range groupNames {
		modelSummaries := groupSummaries[groupName]
		modelItems := make([]UserGroupModelStatusItem, 0, len(modelSummaries))
		groupStatus := "unknown"
		groupRequestCount := int64(0)

		for _, summary := range modelSummaries {
			key := groupName + "::" + summary.Model
			modelRequestCount := requestCounts[key]
			modelRate := successRates[key]
			modelStatus := resolveGroupModelStatus(summary.Status, modelRate, modelRequestCount)
			if groupStatus == "unknown" || modelStatusWeight(modelStatus) < modelStatusWeight(groupStatus) {
				groupStatus = modelStatus
			}
			groupRequestCount += modelRequestCount

			series := seriesByModel[key]
			if len(series) == 0 {
				series = emptyStatusSeries(timelineSampleMinutes, timelineSegmentCount, bucketSeconds)
			}

			modelItems = append(modelItems, UserGroupModelStatusItem{
				Model:         summary.Model,
				Status:        modelStatus,
				SuccessRate:   modelRate,
				SampleHours:   sampleWindowHours,
				SeriesWindow:  seriesWindowHours,
				BucketSeconds: bucketSeconds,
				RequestCount:  modelRequestCount,
				Series:        series,
			})
		}

		sort.Slice(modelItems, func(i, j int) bool {
			if modelItems[i].RequestCount != modelItems[j].RequestCount {
				return modelItems[i].RequestCount > modelItems[j].RequestCount
			}
			left := modelStatusWeight(modelItems[i].Status)
			right := modelStatusWeight(modelItems[j].Status)
			if left != right {
				return left < right
			}
			return modelItems[i].Model < modelItems[j].Model
		})

		result = append(result, UserGroupStatusItem{
			Group:        groupName,
			Status:       groupStatus,
			RequestCount: groupRequestCount,
			Models:       modelItems,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].RequestCount == result[j].RequestCount {
			return result[i].Group < result[j].Group
		}
		return result[i].RequestCount > result[j].RequestCount
	})

	return result, nil
}

func resolveVisibleGroupStatusGroups(userID int, hasUser bool, pricing []gatewaydomain.Pricing) ([]string, error) {
	if !hasUser || userID <= 0 {
		groups := collectPricingGroups(pricing)
		if len(groups) == 0 {
			return gatewaystore.ListGroupStatusGroups()
		}
		return groups, nil
	}

	userGroup, err := identitystore.LoadUserGroup(userID, false)
	if err != nil {
		return nil, err
	}

	groups := make(map[string]struct{})
	for groupName := range GetUserUsableGroups(userGroup) {
		if groupName == "auto" {
			for _, autoGroup := range GetUserAutoGroup(userGroup) {
				addGroupStatusName(groups, autoGroup)
			}
			continue
		}
		addGroupStatusName(groups, groupName)
	}
	addGroupStatusName(groups, userGroup)

	if len(groups) == 0 {
		for _, groupName := range collectPricingGroups(pricing) {
			addGroupStatusName(groups, groupName)
		}
	}
	if len(groups) == 0 {
		return gatewaystore.ListGroupStatusGroups()
	}
	return sortedGroupStatusNames(groups), nil
}

func collectPricingGroups(pricing []gatewaydomain.Pricing) []string {
	groups := make(map[string]struct{})
	for _, item := range pricing {
		for _, groupName := range item.EnableGroup {
			groupName = strings.TrimSpace(groupName)
			if groupName == "" || groupName == "auto" || groupName == "all" {
				continue
			}
			groups[groupName] = struct{}{}
		}
	}
	return sortedGroupStatusNames(groups)
}

func buildPricingGroupModelSummaries(pricing []gatewaydomain.Pricing, groupNames []string) map[string][]*GroupModelStatusSummary {
	if len(groupNames) == 0 {
		groupNames = collectPricingGroups(pricing)
	}
	summaries := make(map[string][]*GroupModelStatusSummary, len(groupNames))
	visibleGroups := make(map[string]struct{}, len(groupNames))
	knownModels := make(map[string]map[string]struct{}, len(groupNames))
	for _, groupName := range groupNames {
		summaries[groupName] = []*GroupModelStatusSummary{}
		visibleGroups[groupName] = struct{}{}
		knownModels[groupName] = make(map[string]struct{})
	}
	if len(groupNames) == 0 {
		return summaries
	}

	for _, item := range pricing {
		targetGroups := pricingTargetGroups(item.EnableGroup, groupNames, visibleGroups)
		if len(targetGroups) == 0 {
			continue
		}
		for _, groupName := range targetGroups {
			if _, ok := knownModels[groupName][item.ModelName]; ok {
				continue
			}
			knownModels[groupName][item.ModelName] = struct{}{}
			summaries[groupName] = append(summaries[groupName], &GroupModelStatusSummary{
				Group:           groupName,
				Model:           item.ModelName,
				Status:          "healthy",
				Channels:        0,
				EnabledChannels: 0,
			})
		}
	}

	return summaries
}

func pricingTargetGroups(enableGroups []string, groupNames []string, visibleGroups map[string]struct{}) []string {
	targets := make([]string, 0, len(enableGroups))
	seen := make(map[string]struct{}, len(enableGroups))
	allGroups := len(groupNames) > 0
	for _, groupName := range enableGroups {
		groupName = strings.TrimSpace(groupName)
		if groupName == "" || groupName == "auto" {
			continue
		}
		if groupName == "all" {
			if allGroups {
				for _, visibleGroup := range groupNames {
					if _, ok := seen[visibleGroup]; ok {
						continue
					}
					if _, ok := visibleGroups[visibleGroup]; !ok {
						continue
					}
					seen[visibleGroup] = struct{}{}
					targets = append(targets, visibleGroup)
				}
			}
			continue
		}
		if _, ok := visibleGroups[groupName]; !ok {
			continue
		}
		if _, ok := seen[groupName]; ok {
			continue
		}
		seen[groupName] = struct{}{}
		targets = append(targets, groupName)
	}
	return targets
}

func queryGroupModelRecentHealth(groupNames []string, sampleMinutes int, segmentCount int) (map[string]*float64, map[string][]UserGroupStatusBucket, map[string]int64, float64, int64) {
	rates := make(map[string]*float64)
	seriesByModel := make(map[string][]UserGroupStatusBucket)
	requestCounts := make(map[string]int64)
	if len(groupNames) == 0 {
		return rates, seriesByModel, requestCounts, 0, 60
	}
	if segmentCount <= 0 {
		segmentCount = 20
	}

	windowSeconds := int64(sampleMinutes * 60)
	requestedBucketSeconds := windowSeconds / int64(segmentCount)
	if windowSeconds%int64(segmentCount) != 0 {
		requestedBucketSeconds++
	}
	if requestedBucketSeconds < 60 {
		requestedBucketSeconds = 60
	}

	sampleWindowHours := float64(windowSeconds) / 3600
	now := time.Now().Unix()
	windowStart, windowEnd, alignedSegments := buildAlignedStatusWindow(now, windowSeconds, requestedBucketSeconds)

	if shouldPreferLogHealth(requestedBucketSeconds) &&
		fillGroupModelLogHealth(rates, seriesByModel, requestCounts, windowStart, windowEnd, requestedBucketSeconds, alignedSegments, groupNames) {
		return rates, seriesByModel, requestCounts, sampleWindowHours, requestedBucketSeconds
	}

	if actualBucketSeconds, ok := fillGroupModelPerfHealth(
		rates,
		seriesByModel,
		requestCounts,
		windowStart,
		windowEnd,
		requestedBucketSeconds,
		alignedSegments,
		groupNames,
	); ok {
		return rates, seriesByModel, requestCounts, sampleWindowHours, actualBucketSeconds
	}

	fillGroupModelLogHealth(rates, seriesByModel, requestCounts, windowStart, windowEnd, requestedBucketSeconds, alignedSegments, groupNames)
	return rates, seriesByModel, requestCounts, sampleWindowHours, requestedBucketSeconds
}

func shouldPreferLogHealth(bucketSeconds int64) bool {
	return bucketSeconds < 3600
}

func fillGroupModelPerfHealth(
	rates map[string]*float64,
	seriesByModel map[string][]UserGroupStatusBucket,
	requestCounts map[string]int64,
	windowStart int64,
	windowEnd int64,
	bucketSeconds int64,
	segmentCount int,
	groupNames []string,
) (int64, bool) {
	hours := int(math.Ceil(float64(windowEnd-windowStart) / 3600))
	if hours <= 0 {
		hours = 1
	}

	summaryRows, err := auditprojection.QuerySummaryByGroupModels(hours, groupNames)
	if err != nil {
		return bucketSeconds, false
	}
	seriesRows, err := auditprojection.QuerySeriesByGroupModels(hours, groupNames)
	if err != nil && len(summaryRows) == 0 {
		return bucketSeconds, false
	}
	if len(summaryRows) == 0 && len(seriesRows) == 0 {
		return bucketSeconds, false
	}

	for _, row := range summaryRows {
		if row.RequestCount <= 0 {
			continue
		}
		key := row.Group + "::" + row.ModelName
		requestCounts[key] += row.RequestCount
		rate := row.SuccessRate
		rates[key] = &rate
	}

	actualBucketSeconds := detectBucketSecondsFromSeries(seriesRows)
	if actualBucketSeconds <= 0 {
		actualBucketSeconds = bucketSeconds
	}

	alignedStart, _, alignedSegments := buildAlignedStatusWindow(windowEnd-1, windowEnd-windowStart, actualBucketSeconds)
	for _, row := range seriesRows {
		key := row.Group + "::" + row.ModelName
		if _, ok := seriesByModel[key]; !ok {
			seriesByModel[key] = buildStatusSeries(alignedStart, alignedSegments, actualBucketSeconds)
		}
		for _, point := range row.Series {
			bucketIndex := (point.Ts - alignedStart) / actualBucketSeconds
			if bucketIndex < 0 || bucketIndex >= int64(alignedSegments) {
				continue
			}
			bucket := &seriesByModel[key][bucketIndex]
			bucket.RequestCount += point.RequestCount
			if point.RequestCount > 0 {
				rate := point.SuccessRate
				bucket.SuccessRate = &rate
			}
		}
	}

	return actualBucketSeconds, len(requestCounts) > 0 || len(seriesByModel) > 0
}

func fillGroupModelLogHealth(
	rates map[string]*float64,
	seriesByModel map[string][]UserGroupStatusBucket,
	requestCounts map[string]int64,
	windowStart int64,
	windowEnd int64,
	bucketSeconds int64,
	segmentCount int,
	groupNames []string,
) bool {
	rows, err := gatewaystore.LoadGroupModelRequestBuckets(windowStart, windowEnd, bucketSeconds, groupNames)
	if err != nil {
		return false
	}

	successCounts := make(map[string]int64)
	for _, row := range rows {
		if row.BucketIndex < 0 || row.BucketIndex >= int64(segmentCount) {
			continue
		}
		key := row.GroupName + "::" + row.ModelName
		if _, ok := seriesByModel[key]; !ok {
			seriesByModel[key] = buildStatusSeries(windowStart, segmentCount, bucketSeconds)
		}
		bucket := &seriesByModel[key][row.BucketIndex]
		bucket.RequestCount += row.RequestCount
		if row.RequestCount > 0 {
			rate := float64(row.SuccessCount) / float64(row.RequestCount) * 100
			bucket.SuccessRate = &rate
			requestCounts[key] += row.RequestCount
			successCounts[key] += row.SuccessCount
		}
	}
	applyGroupModelRates(rates, requestCounts, successCounts)
	return len(requestCounts) > 0 || len(seriesByModel) > 0
}

func applyGroupModelRates(rates map[string]*float64, requestCounts map[string]int64, successCounts map[string]int64) {
	for key, requestCount := range requestCounts {
		if requestCount > 0 {
			rate := float64(successCounts[key]) / float64(requestCount) * 100
			rates[key] = &rate
		}
	}
}

func modelStatusWeight(status string) int {
	switch status {
	case "degraded":
		return 0
	case "slow":
		return 1
	case "unknown":
		return 2
	default:
		return 3
	}
}

func resolveGroupModelStatus(baseStatus string, successRate *float64, requestCount int64) string {
	if baseStatus == "degraded" {
		return "degraded"
	}
	if requestCount <= 0 || successRate == nil {
		return "unknown"
	}
	if *successRate >= 85 {
		return "healthy"
	}
	if *successRate >= 30 {
		return "slow"
	}
	return "degraded"
}

func buildStatusSeries(windowStart int64, segmentCount int, bucketSeconds int64) []UserGroupStatusBucket {
	series := make([]UserGroupStatusBucket, 0, segmentCount)
	for index := 0; index < segmentCount; index++ {
		series = append(series, UserGroupStatusBucket{
			Ts:           windowStart + int64(index)*bucketSeconds,
			SuccessRate:  nil,
			RequestCount: 0,
		})
	}
	return series
}

func buildAlignedStatusWindow(now int64, windowSeconds int64, bucketSeconds int64) (int64, int64, int) {
	if bucketSeconds <= 0 {
		bucketSeconds = 60
	}
	if windowSeconds <= 0 {
		windowSeconds = bucketSeconds
	}
	segmentCount := int((windowSeconds + bucketSeconds - 1) / bucketSeconds)
	if segmentCount <= 0 {
		segmentCount = 1
	}
	currentBucketStart := now - (now % bucketSeconds)
	windowEnd := currentBucketStart + bucketSeconds
	windowStart := windowEnd - int64(segmentCount)*bucketSeconds
	return windowStart, windowEnd, segmentCount
}

func detectBucketSecondsFromSeries(rows []auditprojection.GroupModelSeries) int64 {
	for _, row := range rows {
		for index := 1; index < len(row.Series); index++ {
			diff := row.Series[index].Ts - row.Series[index-1].Ts
			if diff > 0 {
				return diff
			}
		}
	}
	return 0
}

func emptyStatusSeries(sampleMinutes int, segmentCount int, bucketSeconds int64) []UserGroupStatusBucket {
	windowStart, _, alignedSegments := buildAlignedStatusWindow(time.Now().Unix(), int64(sampleMinutes*60), bucketSeconds)
	return buildStatusSeries(windowStart, alignedSegments, bucketSeconds)
}

func addGroupStatusName(groups map[string]struct{}, groupName string) {
	groupName = strings.TrimSpace(groupName)
	if groupName == "" || groupName == "auto" {
		return
	}
	groups[groupName] = struct{}{}
}

func sortedGroupStatusNames(groups map[string]struct{}) []string {
	result := make([]string, 0, len(groups))
	for groupName := range groups {
		result = append(result, groupName)
	}
	sort.Strings(result)
	return result
}
