package app

import (
	"sort"
	"strconv"
	"strings"

	"github.com/sh2001sh/CodeGo-Api/constant"
	gatewayschema "github.com/sh2001sh/CodeGo-Api/internal/gateway/schema"
	gatewaystore "github.com/sh2001sh/CodeGo-Api/internal/gateway/store"
	platformencoding "github.com/sh2001sh/CodeGo-Api/internal/platform/encodingx"
)

type ModelMetaListResult struct {
	Items        []*gatewayschema.Model
	Total        int64
	VendorCounts map[int64]int64
}

func GetMissingModels() ([]string, error) {
	return gatewaystore.LoadMissingModels()
}

func GetAllModelsMeta(offset int, limit int, status string, syncOfficial string) (*ModelMetaListResult, error) {
	if err := gatewaystore.EnsureEnabledModelsMeta(); err != nil {
		return nil, err
	}
	modelsMeta, err := gatewaystore.ListModels(offset, limit, status, syncOfficial)
	if err != nil {
		return nil, err
	}
	enrichModels(modelsMeta)

	total, err := gatewaystore.CountModels(status, syncOfficial)
	if err != nil {
		return nil, err
	}

	vendorCounts, _ := gatewaystore.LoadVendorModelCounts()
	return &ModelMetaListResult{
		Items:        modelsMeta,
		Total:        total,
		VendorCounts: vendorCounts,
	}, nil
}

func SearchModelsMeta(keyword string, vendor string, status string, syncOfficial string, offset int, limit int) ([]*gatewayschema.Model, int64, error) {
	if err := gatewaystore.EnsureEnabledModelsMeta(); err != nil {
		return nil, 0, err
	}
	modelsMeta, total, err := gatewaystore.SearchModels(keyword, vendor, status, syncOfficial, offset, limit)
	if err != nil {
		return nil, 0, err
	}
	enrichModels(modelsMeta)
	return modelsMeta, total, nil
}

func GetModelMeta(id int) (*gatewayschema.Model, error) {
	item, err := gatewaystore.LoadModelByID(id)
	if err != nil {
		return nil, err
	}
	enrichModels([]*gatewayschema.Model{item})
	return item, nil
}

func IsModelNameDuplicated(id int, name string) (bool, error) {
	return gatewaystore.IsModelNameDuplicated(id, name)
}

func CreateModelMeta(item *gatewayschema.Model) error {
	if err := gatewaystore.CreateModelRecord(item); err != nil {
		return err
	}
	gatewaystore.RefreshPricing()
	return nil
}

func UpdateModelMeta(item *gatewayschema.Model, statusOnly bool) error {
	if statusOnly {
		if err := gatewaystore.UpdateModelStatus(item.Id, item.Status); err != nil {
			return err
		}
		gatewaystore.RefreshPricing()
		return nil
	}
	if err := gatewaystore.UpdateModelRecord(item); err != nil {
		return err
	}
	gatewaystore.RefreshPricing()
	return nil
}

func DeleteModelMeta(id int) error {
	if err := gatewaystore.DeleteModelRecord(id); err != nil {
		return err
	}
	gatewaystore.RefreshPricing()
	return nil
}

func enrichModels(models []*gatewayschema.Model) {
	if len(models) == 0 {
		return
	}

	exactNames := make([]string, 0)
	exactIdx := make(map[string][]int)
	ruleIndices := make([]int, 0)
	for i, item := range models {
		if item == nil {
			continue
		}
		if item.NameRule == gatewayschema.NameRuleExact {
			exactNames = append(exactNames, item.ModelName)
			exactIdx[item.ModelName] = append(exactIdx[item.ModelName], i)
		} else {
			ruleIndices = append(ruleIndices, i)
		}
	}

	channelsByModel, _ := gatewaystore.LoadBoundChannelsByModelsMap(exactNames)

	for name, indices := range exactIdx {
		chs := channelsByModel[name]
		for _, idx := range indices {
			item := models[idx]
			if item.Endpoints == "" {
				eps := loadGatewayModelSupportedEndpointTypes(item.ModelName)
				if b, err := platformencoding.Marshal(eps); err == nil {
					item.Endpoints = string(b)
				}
			}
			item.BoundChannels = chs
			item.EnableGroups = loadGatewayModelEnableGroups(item.ModelName)
			item.QuotaTypes = loadGatewayModelQuotaTypes(item.ModelName)
		}
	}

	if len(ruleIndices) == 0 {
		return
	}

	pricings := loadGatewayPricing()
	matchedNamesByIdx := make(map[int][]string)
	endpointSetByIdx := make(map[int]map[constant.EndpointType]struct{})
	groupSetByIdx := make(map[int]map[string]struct{})
	quotaSetByIdx := make(map[int]map[int]struct{})

	for _, pricing := range pricings {
		for _, idx := range ruleIndices {
			item := models[idx]
			var matched bool
			switch item.NameRule {
			case gatewayschema.NameRulePrefix:
				matched = strings.HasPrefix(pricing.ModelName, item.ModelName)
			case gatewayschema.NameRuleSuffix:
				matched = strings.HasSuffix(pricing.ModelName, item.ModelName)
			case gatewayschema.NameRuleContains:
				matched = strings.Contains(pricing.ModelName, item.ModelName)
			}
			if !matched {
				continue
			}
			matchedNamesByIdx[idx] = append(matchedNamesByIdx[idx], pricing.ModelName)

			es := endpointSetByIdx[idx]
			if es == nil {
				es = make(map[constant.EndpointType]struct{})
				endpointSetByIdx[idx] = es
			}
			for _, endpointType := range pricing.SupportedEndpointTypes {
				es[endpointType] = struct{}{}
			}

			gs := groupSetByIdx[idx]
			if gs == nil {
				gs = make(map[string]struct{})
				groupSetByIdx[idx] = gs
			}
			for _, group := range pricing.EnableGroup {
				gs[group] = struct{}{}
			}

			qs := quotaSetByIdx[idx]
			if qs == nil {
				qs = make(map[int]struct{})
				quotaSetByIdx[idx] = qs
			}
			qs[pricing.QuotaType] = struct{}{}
		}
	}

	allMatchedSet := make(map[string]struct{})
	for _, names := range matchedNamesByIdx {
		for _, name := range names {
			allMatchedSet[name] = struct{}{}
		}
	}
	allMatched := make([]string, 0, len(allMatchedSet))
	for name := range allMatchedSet {
		allMatched = append(allMatched, name)
	}
	matchedChannelsByModel, _ := gatewaystore.LoadBoundChannelsByModelsMap(allMatched)

	for _, idx := range ruleIndices {
		item := models[idx]

		if es, ok := endpointSetByIdx[idx]; ok && item.Endpoints == "" {
			eps := make([]constant.EndpointType, 0, len(es))
			for endpointType := range es {
				eps = append(eps, endpointType)
			}
			if b, err := platformencoding.Marshal(eps); err == nil {
				item.Endpoints = string(b)
			}
		}

		if gs, ok := groupSetByIdx[idx]; ok {
			groups := make([]string, 0, len(gs))
			for group := range gs {
				groups = append(groups, group)
			}
			item.EnableGroups = groups
		}

		if qs, ok := quotaSetByIdx[idx]; ok {
			arr := make([]int, 0, len(qs))
			for value := range qs {
				arr = append(arr, value)
			}
			sort.Ints(arr)
			item.QuotaTypes = arr
		}

		names := matchedNamesByIdx[idx]
		channelSet := make(map[string]gatewayschema.BoundChannel)
		for _, name := range names {
			for _, channel := range matchedChannelsByModel[name] {
				key := channel.Name + "_" + strconv.Itoa(channel.Type)
				channelSet[key] = channel
			}
		}
		if len(channelSet) > 0 {
			chs := make([]gatewayschema.BoundChannel, 0, len(channelSet))
			for _, channel := range channelSet {
				chs = append(chs, channel)
			}
			item.BoundChannels = chs
		}

		item.MatchedModels = names
		item.MatchedCount = len(names)
	}
}
