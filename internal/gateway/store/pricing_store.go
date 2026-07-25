package store

import (
	"encoding/json"
	"fmt"
	platformobservability "github.com/sh2001sh/CodeGo-Api/internal/platform/observability"
	platformtext "github.com/sh2001sh/CodeGo-Api/internal/platform/textx"
	"strings"
	"sync"
	"time"

	"github.com/sh2001sh/CodeGo-Api/constant"
	gatewaycontract "github.com/sh2001sh/CodeGo-Api/internal/gateway/contract"
	gatewaydomain "github.com/sh2001sh/CodeGo-Api/internal/gateway/domain"
	gatewayschema "github.com/sh2001sh/CodeGo-Api/internal/gateway/schema"
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
	"github.com/sh2001sh/CodeGo-Api/types"
)

var (
	pricingMap           []gatewaydomain.Pricing
	vendorsList          []gatewaydomain.PricingVendor
	supportedEndpointMap map[string]gatewaycontract.EndpointInfo
	lastGetPricingTime   time.Time
	updatePricingLock    sync.Mutex

	modelEnableGroups     = make(map[string][]string)
	modelQuotaTypeMap     = make(map[string]int)
	modelEnableGroupsLock sync.RWMutex

	modelSupportEndpointTypes = make(map[string][]constant.EndpointType)
	modelSupportEndpointsLock sync.RWMutex
)

func LoadPricing() []gatewaydomain.Pricing {
	if time.Since(lastGetPricingTime) > time.Minute || len(pricingMap) == 0 {
		updatePricingLock.Lock()
		defer updatePricingLock.Unlock()
		if time.Since(lastGetPricingTime) > time.Minute || len(pricingMap) == 0 {
			modelSupportEndpointsLock.Lock()
			defer modelSupportEndpointsLock.Unlock()
			updatePricing()
		}
	}
	return pricingMap
}

func InvalidatePricingCache() {
	updatePricingLock.Lock()
	defer updatePricingLock.Unlock()

	pricingMap = nil
	vendorsList = nil
	supportedEndpointMap = nil
	lastGetPricingTime = time.Time{}
}

func RefreshPricing() {
	updatePricingLock.Lock()
	defer updatePricingLock.Unlock()

	modelSupportEndpointsLock.Lock()
	defer modelSupportEndpointsLock.Unlock()

	updatePricing()
}

func LoadPricingVendors() []gatewaydomain.PricingVendor {
	if time.Since(lastGetPricingTime) > time.Minute || len(pricingMap) == 0 {
		LoadPricing()
	}
	return vendorsList
}

func LoadSupportedEndpointMap() map[string]gatewaycontract.EndpointInfo {
	LoadPricing()
	return supportedEndpointMap
}

func LoadModelSupportedEndpointTypes(modelName string) []constant.EndpointType {
	if modelName == "" {
		return make([]constant.EndpointType, 0)
	}
	LoadPricing()
	modelSupportEndpointsLock.RLock()
	defer modelSupportEndpointsLock.RUnlock()
	if endpoints, ok := modelSupportEndpointTypes[modelName]; ok {
		return endpoints
	}
	return make([]constant.EndpointType, 0)
}

func LoadModelEnableGroups(modelName string) []string {
	LoadPricing()
	if modelName == "" {
		return make([]string, 0)
	}
	modelEnableGroupsLock.RLock()
	defer modelEnableGroupsLock.RUnlock()
	groups, ok := modelEnableGroups[modelName]
	if !ok {
		return make([]string, 0)
	}
	return groups
}

func LoadModelQuotaTypes(modelName string) []int {
	LoadPricing()
	modelEnableGroupsLock.RLock()
	defer modelEnableGroupsLock.RUnlock()
	quota, ok := modelQuotaTypeMap[modelName]
	if !ok {
		return []int{}
	}
	return []int{quota}
}

func updatePricing() {
	enableAbilities, err := LoadAllEnabledAbilitiesWithChannels()
	if err != nil {
		platformobservability.SysLog(fmt.Sprintf("GetAllEnableAbilityWithChannels error: %v", err))
		return
	}

	var allMeta []gatewayschema.Model
	_ = platformdb.DB.Find(&allMeta).Error
	metaMap := make(map[string]*gatewayschema.Model)
	prefixList := make([]*gatewayschema.Model, 0)
	suffixList := make([]*gatewayschema.Model, 0)
	containsList := make([]*gatewayschema.Model, 0)
	for i := range allMeta {
		m := &allMeta[i]
		if m.NameRule == gatewayschema.NameRuleExact {
			metaMap[m.ModelName] = m
		} else {
			switch m.NameRule {
			case gatewayschema.NameRulePrefix:
				prefixList = append(prefixList, m)
			case gatewayschema.NameRuleSuffix:
				suffixList = append(suffixList, m)
			case gatewayschema.NameRuleContains:
				containsList = append(containsList, m)
			}
		}
	}

	for _, m := range prefixList {
		for _, pricingModel := range enableAbilities {
			if strings.HasPrefix(pricingModel.Model, m.ModelName) {
				if _, exists := metaMap[pricingModel.Model]; !exists {
					metaMap[pricingModel.Model] = m
				}
			}
		}
	}
	for _, m := range suffixList {
		for _, pricingModel := range enableAbilities {
			if strings.HasSuffix(pricingModel.Model, m.ModelName) {
				if _, exists := metaMap[pricingModel.Model]; !exists {
					metaMap[pricingModel.Model] = m
				}
			}
		}
	}
	for _, m := range containsList {
		for _, pricingModel := range enableAbilities {
			if strings.Contains(pricingModel.Model, m.ModelName) {
				if _, exists := metaMap[pricingModel.Model]; !exists {
					metaMap[pricingModel.Model] = m
				}
			}
		}
	}

	var vendors []gatewayschema.Vendor
	_ = platformdb.DB.Find(&vendors).Error
	vendorMap := make(map[int]*gatewayschema.Vendor)
	for i := range vendors {
		vendorMap[vendors[i].Id] = &vendors[i]
	}

	initDefaultVendorMapping(metaMap, vendorMap, enableAbilities)

	vendorsList = make([]gatewaydomain.PricingVendor, 0, len(vendorMap))
	for _, v := range vendorMap {
		vendorsList = append(vendorsList, gatewaydomain.PricingVendor{
			ID:          v.Id,
			Name:        v.Name,
			Description: v.Description,
			Icon:        v.Icon,
		})
	}

	modelGroupsMap := make(map[string]*types.Set[string])
	for _, ability := range enableAbilities {
		groups, ok := modelGroupsMap[ability.Model]
		if !ok {
			groups = types.NewSet[string]()
			modelGroupsMap[ability.Model] = groups
		}
		groups.Add(ability.Group)
	}

	modelSupportEndpointsStr := make(map[string][]string)
	for _, ability := range enableAbilities {
		endpoints := modelSupportEndpointsStr[ability.Model]
		channelTypes := gatewaycontract.EndpointTypesByChannelType(ability.ChannelType, ability.Model)
		for _, channelType := range channelTypes {
			if !platformtext.StringsContains(endpoints, string(channelType)) {
				endpoints = append(endpoints, string(channelType))
			}
		}
		modelSupportEndpointsStr[ability.Model] = endpoints
	}

	for modelName, meta := range metaMap {
		if strings.TrimSpace(meta.Endpoints) == "" {
			continue
		}
		var raw map[string]interface{}
		if err := json.Unmarshal([]byte(meta.Endpoints), &raw); err == nil {
			endpoints := make([]string, 0, len(raw))
			for key, value := range raw {
				switch value.(type) {
				case string, map[string]interface{}:
					if !platformtext.StringsContains(endpoints, key) {
						endpoints = append(endpoints, key)
					}
				}
			}
			if len(endpoints) > 0 {
				modelSupportEndpointsStr[modelName] = endpoints
			}
		}
	}

	modelSupportEndpointTypes = make(map[string][]constant.EndpointType)
	for modelName, endpoints := range modelSupportEndpointsStr {
		supportedEndpoints := make([]constant.EndpointType, 0, len(endpoints))
		for _, endpointStr := range endpoints {
			supportedEndpoints = append(supportedEndpoints, constant.EndpointType(endpointStr))
		}
		modelSupportEndpointTypes[modelName] = supportedEndpoints
	}

	supportedEndpointMap = make(map[string]gatewaycontract.EndpointInfo)
	for _, endpoints := range modelSupportEndpointTypes {
		for _, endpointType := range endpoints {
			if info, ok := gatewaycontract.DefaultEndpointInfo(endpointType); ok {
				if _, exists := supportedEndpointMap[string(endpointType)]; !exists {
					supportedEndpointMap[string(endpointType)] = info
				}
			}
		}
	}
	for _, meta := range metaMap {
		if strings.TrimSpace(meta.Endpoints) == "" {
			continue
		}
		var raw map[string]interface{}
		if err := json.Unmarshal([]byte(meta.Endpoints), &raw); err == nil {
			for key, value := range raw {
				switch val := value.(type) {
				case string:
					supportedEndpointMap[key] = gatewaycontract.EndpointInfo{Path: val, Method: "POST"}
				case map[string]interface{}:
					ep := gatewaycontract.EndpointInfo{Method: "POST"}
					if path, ok := val["path"].(string); ok {
						ep.Path = path
					}
					if method, ok := val["method"].(string); ok {
						ep.Method = strings.ToUpper(method)
					}
					supportedEndpointMap[key] = ep
				}
			}
		}
	}

	pricingMap = make([]gatewaydomain.Pricing, 0)
	for modelName, groups := range modelGroupsMap {
		pricing := gatewaydomain.Pricing{
			ModelName:              modelName,
			EnableGroup:            groups.Items(),
			SupportedEndpointTypes: modelSupportEndpointTypes[modelName],
		}
		if meta, ok := metaMap[modelName]; ok {
			if meta.Status != 1 {
				continue
			}
			pricing.Description = meta.Description
			pricing.Icon = meta.Icon
			pricing.Tags = meta.Tags
			pricing.VendorID = meta.VendorID
		}
		modelPrice, findPrice := GetModelPrice(modelName, false)
		if findPrice {
			pricing.ModelPrice = modelPrice
			pricing.QuotaType = 1
		} else {
			modelRatio, _, _ := GetModelRatio(modelName)
			pricing.ModelRatio = modelRatio
			pricing.CompletionRatio = GetCompletionRatio(modelName)
			pricing.QuotaType = 0
		}
		if cacheRatio, ok := GetCacheRatio(modelName); ok {
			pricing.CacheRatio = &cacheRatio
		}
		if createCacheRatio, ok := GetCreateCacheRatio(modelName); ok {
			pricing.CreateCacheRatio = &createCacheRatio
		}
		if imageRatio, ok := GetImageRatio(modelName); ok {
			pricing.ImageRatio = &imageRatio
		}
		if ContainsAudioRatio(modelName) {
			audioRatio := GetAudioRatio(modelName)
			pricing.AudioRatio = &audioRatio
		}
		if ContainsAudioCompletionRatio(modelName) {
			audioCompletionRatio := GetAudioCompletionRatio(modelName)
			pricing.AudioCompletionRatio = &audioCompletionRatio
		}
		if billingMode := GetBillingMode(modelName); billingMode == BillingModeTieredExpr {
			if expr, ok := GetBillingExpr(modelName); ok && strings.TrimSpace(expr) != "" {
				pricing.BillingMode = billingMode
				pricing.BillingExpr = expr
			}
		}
		pricingMap = append(pricingMap, pricing)
	}

	if len(pricingMap) > 0 {
		pricingMap[0].PricingVersion = "5a90f2b86c08bd983a9a2e6d66c255f4eaef9c4bc934386d2b6ae84ef0ff1f1f"
	}

	modelEnableGroupsLock.Lock()
	modelEnableGroups = make(map[string][]string)
	modelQuotaTypeMap = make(map[string]int)
	for _, pricing := range pricingMap {
		modelEnableGroups[pricing.ModelName] = pricing.EnableGroup
		modelQuotaTypeMap[pricing.ModelName] = pricing.QuotaType
	}
	modelEnableGroupsLock.Unlock()

	lastGetPricingTime = time.Now()
}
