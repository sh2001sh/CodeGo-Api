package app

import (
	"strings"

	gatewaygroups "github.com/sh2001sh/CodeGo-Api/internal/gateway/groupsettings"
	gatewaystore "github.com/sh2001sh/CodeGo-Api/internal/gateway/store"
)

const AutoGroupName = "auto"

// NormalizeTokenGroup makes an omitted token group opt into automatic routing.
func NormalizeTokenGroup(group string) string {
	group = strings.TrimSpace(group)
	if group == "" {
		return AutoGroupName
	}
	return group
}

func GetUserUsableGroups(userGroup string) map[string]string {
	groupsCopy := gatewaygroups.GetUserUsableGroupsCopy()
	if userGroup != "" {
		specialSettings, ok := gatewaystore.GetGroupRatioSetting().GroupSpecialUsableGroup.Get(userGroup)
		if ok {
			for specialGroup, desc := range specialSettings {
				if strings.HasPrefix(specialGroup, "-:") {
					groupToRemove := strings.TrimPrefix(specialGroup, "-:")
					delete(groupsCopy, groupToRemove)
				} else if strings.HasPrefix(specialGroup, "+:") {
					groupToAdd := strings.TrimPrefix(specialGroup, "+:")
					groupsCopy[groupToAdd] = desc
				} else {
					groupsCopy[specialGroup] = desc
				}
			}
		}
		if _, ok := groupsCopy[userGroup]; !ok {
			groupsCopy[userGroup] = "用户分组"
		}
	}
	for _, group := range gatewaygroups.GetAutoGroups() {
		if _, ok := groupsCopy[group]; ok {
			groupsCopy[AutoGroupName] = "自动分组"
			break
		}
	}
	return groupsCopy
}

func GroupInUserUsableGroups(userGroup, groupName string) bool {
	_, ok := GetUserUsableGroups(userGroup)[groupName]
	return ok
}

func GetUserAutoGroup(userGroup string) []string {
	groups := GetUserUsableGroups(userGroup)
	autoGroups := make([]string, 0)
	for _, group := range gatewaygroups.GetAutoGroups() {
		if _, ok := groups[group]; ok {
			autoGroups = append(autoGroups, group)
		}
	}
	return autoGroups
}

func GetUserGroupRatio(userGroup, group string) float64 {
	ratio, ok := gatewaystore.GetGroupGroupRatio(userGroup, group)
	if ok {
		return ratio
	}
	return gatewaystore.GetGroupRatio(group)
}
