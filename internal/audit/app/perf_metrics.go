package app

import (
	auditprojection "github.com/sh2001sh/CodeGo-Api/internal/audit/projection"
	gatewaystore "github.com/sh2001sh/CodeGo-Api/internal/gateway/store"
)

func BuildPerfMetricsSummary(hours int) (any, error) {
	if hours <= 0 {
		hours = 24
	}
	return auditprojection.QuerySummaryAll(hours)
}

func BuildPerfMetrics(modelName string, group string, hours int) (*auditprojection.QueryResult, error) {
	if hours <= 0 {
		hours = 24
	}

	result, err := auditprojection.Query(auditprojection.QueryParams{
		Model: modelName,
		Group: group,
		Hours: hours,
	})
	if err != nil {
		return nil, err
	}

	result.Groups = filterActivePerfMetricGroups(result.Groups)
	return &result, nil
}

func filterActivePerfMetricGroups(groups []auditprojection.GroupResult) []auditprojection.GroupResult {
	activeGroups := gatewaystore.GetGroupRatioCopy()
	filtered := make([]auditprojection.GroupResult, 0, len(groups))
	for _, groupItem := range groups {
		if _, ok := activeGroups[groupItem.Group]; ok || groupItem.Group == "auto" {
			filtered = append(filtered, groupItem)
		}
	}
	return filtered
}
