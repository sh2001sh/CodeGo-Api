package app

import (
	gatewaycontract "github.com/sh2001sh/CodeGo-Api/internal/gateway/contract"
	gatewaydomain "github.com/sh2001sh/CodeGo-Api/internal/gateway/domain"
	gatewaygroups "github.com/sh2001sh/CodeGo-Api/internal/gateway/groupsettings"
	gatewaystore "github.com/sh2001sh/CodeGo-Api/internal/gateway/store"
	identitystore "github.com/sh2001sh/CodeGo-Api/internal/identity/store"
	platformstore "github.com/sh2001sh/CodeGo-Api/internal/platform/store"
)

const pricingVersion = "a42d372ccf0b5dd13ecf71203521f9d2"

type PricingPayload struct {
	Data              []gatewaydomain.Pricing                 `json:"data"`
	Vendors           []gatewaydomain.PricingVendor           `json:"vendors"`
	GroupRatio        map[string]float64                      `json:"group_ratio"`
	UsableGroup       map[string]string                       `json:"usable_group"`
	SupportedEndpoint map[string]gatewaycontract.EndpointInfo `json:"supported_endpoint"`
	AutoGroups        []string                                `json:"auto_groups"`
	PricingVersion    string                                  `json:"pricing_version"`
}

func loadGatewayPricing() []gatewaydomain.Pricing {
	return gatewaystore.LoadPricing()
}

func loadGatewayVendors() []gatewaydomain.PricingVendor {
	return gatewaystore.LoadPricingVendors()
}

func loadGatewaySupportedEndpointMap() map[string]gatewaycontract.EndpointInfo {
	return gatewaystore.LoadSupportedEndpointMap()
}

func resolveUserGroup(userID int, hasUser bool) string {
	if !hasUser || userID <= 0 {
		return ""
	}
	user, err := identitystore.LoadUserCacheSnapshot(userID)
	if err != nil {
		return ""
	}
	return user.Group
}

func resolveUsableGroups(userID int, hasUser bool) map[string]string {
	return GetUserUsableGroups(resolveUserGroup(userID, hasUser))
}

func visibleGroupRatios(userGroup string, usableGroup map[string]string) map[string]float64 {
	groupRatio := make(map[string]float64)
	for name, ratio := range gatewaystore.GetGroupRatioCopy() {
		groupRatio[name] = ratio
	}
	if userGroup != "" {
		for groupName := range groupRatio {
			if ratio, ok := gatewaystore.GetGroupGroupRatio(userGroup, groupName); ok {
				groupRatio[groupName] = ratio
			}
		}
	}
	for groupName := range gatewaystore.GetGroupRatioCopy() {
		if _, ok := usableGroup[groupName]; !ok {
			delete(groupRatio, groupName)
		}
	}
	return groupRatio
}

func filterPricingByUsableGroups(pricing []gatewaydomain.Pricing, usableGroup map[string]string) []gatewaydomain.Pricing {
	if len(pricing) == 0 {
		return pricing
	}
	if len(usableGroup) == 0 {
		return []gatewaydomain.Pricing{}
	}

	filtered := make([]gatewaydomain.Pricing, 0, len(pricing))
	for _, item := range pricing {
		for _, group := range item.EnableGroup {
			if group == "all" {
				filtered = append(filtered, item)
				goto nextPricingItem
			}
		}
		for _, group := range item.EnableGroup {
			if _, ok := usableGroup[group]; ok {
				filtered = append(filtered, item)
				break
			}
		}
	nextPricingItem:
	}
	return filtered
}

func BuildPricingPayload(userID int, hasUser bool) PricingPayload {
	pricing := filterPricingByUsableGroups(loadGatewayPricing(), resolveUsableGroups(userID, hasUser))
	userGroup := resolveUserGroup(userID, hasUser)
	usableGroup := GetUserUsableGroups(userGroup)
	groupRatio := visibleGroupRatios(userGroup, usableGroup)

	return PricingPayload{
		Data:              pricing,
		Vendors:           loadGatewayVendors(),
		GroupRatio:        groupRatio,
		UsableGroup:       usableGroup,
		SupportedEndpoint: loadGatewaySupportedEndpointMap(),
		AutoGroups:        GetUserAutoGroup(userGroup),
		PricingVersion:    pricingVersion,
	}
}

func BuildAllGroupNames() []string {
	groupNames := make([]string, 0)
	for groupName := range gatewaystore.GetGroupRatioCopy() {
		groupNames = append(groupNames, groupName)
	}
	return groupNames
}

func BuildUserGroupsPayload(userID int) map[string]map[string]any {
	userGroup, _ := identitystore.LoadUserGroup(userID, false)
	userUsableGroups := GetUserUsableGroups(userGroup)
	usableGroups := make(map[string]map[string]any)

	for groupName := range gatewaystore.GetGroupRatioCopy() {
		if desc, ok := userUsableGroups[groupName]; ok {
			usableGroups[groupName] = map[string]any{
				"ratio": GetUserGroupRatio(userGroup, groupName),
				"desc":  desc,
			}
		}
	}

	if _, ok := userUsableGroups["auto"]; ok {
		usableGroups["auto"] = map[string]any{
			"ratio": "自动",
			"desc":  gatewaygroups.GetUsableGroupDescription("auto"),
		}
	}
	return usableGroups
}

func ResetModelRatio() error {
	defaultStr := gatewaystore.DefaultModelRatio2JSONString()
	if err := platformstore.UpdateOption("ModelRatio", defaultStr); err != nil {
		return err
	}
	return gatewaystore.UpdateModelRatioByJSONString(defaultStr)
}

func ExposedRatioConfig() (any, bool) {
	if !gatewaystore.IsExposeRatioEnabled() {
		return nil, false
	}
	return gatewaystore.GetExposedData(), true
}
