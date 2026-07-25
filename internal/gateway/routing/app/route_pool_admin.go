package app

import (
	"errors"
	"sort"
	"strings"

	gatewayruntime "github.com/sh2001sh/CodeGo-Api/internal/gateway/runtime"
	gatewayschema "github.com/sh2001sh/CodeGo-Api/internal/gateway/schema"
	gatewaystore "github.com/sh2001sh/CodeGo-Api/internal/gateway/store"
)

// RoutePoolGroup is the consolidated Root-only configuration view. Groups are
// discovered from channel assignments, rather than entered separately.
type RoutePoolGroup struct {
	Group           string                  `json:"group"`
	PoolID          int64                   `json:"pool_id"`
	Enabled         bool                    `json:"enabled"`
	AlgorithmActive bool                    `json:"algorithm_active"`
	AutoDiscover    bool                    `json:"auto_discover"`
	Channels        []RoutePoolGroupChannel `json:"channels"`
}

type RoutePoolGroupChannel struct {
	ChannelID          int     `json:"channel_id"`
	ChannelName        string  `json:"channel_name"`
	ChannelStatus      int     `json:"channel_status"`
	Models             string  `json:"models"`
	Enabled            bool    `json:"enabled"`
	CostMultiplier     float64 `json:"cost_multiplier"`
	ModelCostOverrides string  `json:"model_cost_overrides"`
}

type RoutePoolMetrics struct {
	PoolID  int64                    `json:"pool_id"`
	Group   string                   `json:"group"`
	Model   string                   `json:"model"`
	Members []RoutePoolMemberMetrics `json:"members"`
}

type RoutePoolMemberMetrics struct {
	ChannelID          int                          `json:"channel_id"`
	ChannelName        string                       `json:"channel_name"`
	Enabled            bool                         `json:"enabled"`
	Eligible           bool                         `json:"eligible"`
	CostMultiplier     float64                      `json:"cost_multiplier"`
	ModelCostOverrides string                       `json:"model_cost_overrides"`
	Score              float64                      `json:"score"`
	Health             gatewayruntime.ChannelHealth `json:"health"`
}

func ListRoutePools() ([]gatewaystore.RoutePoolDetail, error) {
	return gatewaystore.ListRoutePools()
}

// ListRoutePoolGroups builds routing configuration from the groups already
// assigned to channels. It intentionally includes groups that have not opted
// in yet, so Root can see whether they are still using legacy routing.
func ListRoutePoolGroups() ([]RoutePoolGroup, error) {
	channels, err := gatewaystore.ListAllChannelSummaries()
	if err != nil {
		return nil, err
	}
	pools, err := gatewaystore.ListRoutePools()
	if err != nil {
		return nil, err
	}
	poolByGroup := make(map[string]gatewaystore.RoutePoolDetail, len(pools))
	for _, detail := range pools {
		poolByGroup[detail.Pool.Group] = detail
	}
	groups := make(map[string]*RoutePoolGroup)
	for _, channel := range channels {
		for _, group := range channel.GetGroups() {
			group = strings.TrimSpace(group)
			if group == "" {
				continue
			}
			view := groups[group]
			if view == nil {
				view = &RoutePoolGroup{Group: group, Channels: make([]RoutePoolGroupChannel, 0)}
				if detail, found := poolByGroup[group]; found {
					view.PoolID = detail.Pool.ID
					view.Enabled = detail.Pool.Enabled
					view.AlgorithmActive = detail.Pool.Enabled
					view.AutoDiscover = detail.Pool.AutoDiscover
				}
				groups[group] = view
			}
			member := gatewayschema.RoutePoolMember{CostMultiplier: 1, ModelCostOverrides: "{}", Enabled: true}
			if detail, found := poolByGroup[group]; found {
				member.Enabled = detail.Pool.AutoDiscover
				for _, configured := range detail.Members {
					if configured.ChannelID == channel.Id {
						member = configured
						break
					}
				}
			}
			view.Channels = append(view.Channels, RoutePoolGroupChannel{
				ChannelID: channel.Id, ChannelName: channel.Name, ChannelStatus: channel.Status,
				Models: channel.Models, Enabled: member.Enabled, CostMultiplier: member.CostMultiplier,
				ModelCostOverrides: member.ModelCostOverrides,
			})
		}
	}
	result := make([]RoutePoolGroup, 0, len(groups))
	for _, group := range groups {
		sort.Slice(group.Channels, func(i, j int) bool { return group.Channels[i].ChannelID < group.Channels[j].ChannelID })
		result = append(result, *group)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Group < result[j].Group })
	return result, nil
}

// SaveRoutePoolGroup opts one existing channel group into algorithmic routing.
// Every submitted member must already belong to that group; membership itself
// remains owned by channel configuration.
func SaveRoutePoolGroup(group string, enabled bool, members []gatewayschema.RoutePoolMember) (*gatewaystore.RoutePoolDetail, error) {
	group = strings.TrimSpace(group)
	if group == "" {
		return nil, errors.New("group is required")
	}
	channels, err := gatewaystore.ListAllChannelSummaries()
	if err != nil {
		return nil, err
	}
	assigned := make(map[int]struct{})
	for _, channel := range channels {
		for _, channelGroup := range channel.GetGroups() {
			if channelGroup == group {
				assigned[channel.Id] = struct{}{}
				break
			}
		}
	}
	if len(assigned) == 0 {
		return nil, errors.New("group has no assigned channels")
	}
	for _, member := range members {
		if _, found := assigned[member.ChannelID]; !found {
			return nil, errors.New("route member is not assigned to this group")
		}
	}
	pools, err := gatewaystore.ListRoutePools()
	if err != nil {
		return nil, err
	}
	pool := gatewayschema.RoutePool{Name: group + " 自动路由", Group: group, Enabled: enabled, AutoDiscover: true}
	for _, detail := range pools {
		if detail.Pool.Group == group {
			pool.ID = detail.Pool.ID
			break
		}
	}
	return gatewaystore.SaveRoutePool(&pool, members)
}

func SaveRoutePool(pool gatewayschema.RoutePool, members []gatewayschema.RoutePoolMember) (*gatewaystore.RoutePoolDetail, error) {
	return gatewaystore.SaveRoutePool(&pool, members)
}

func DeleteRoutePool(id int64) error {
	return gatewaystore.DeleteRoutePool(id)
}

func GetRoutePoolMetrics(poolID int64, modelName string) (*RoutePoolMetrics, error) {
	modelName = strings.TrimSpace(modelName)
	if poolID <= 0 || modelName == "" {
		return nil, errors.New("route pool id and model are required")
	}
	pools, err := gatewaystore.ListRoutePools()
	if err != nil {
		return nil, err
	}
	var detail *gatewaystore.RoutePoolDetail
	for index := range pools {
		if pools[index].Pool.ID == poolID {
			detail = &pools[index]
			break
		}
	}
	if detail == nil {
		return nil, errors.New("route pool not found")
	}
	members := detail.Members
	if detail.Pool.AutoDiscover {
		members, err = gatewaystore.ExpandRoutePoolMembers(detail.Pool.Group, members)
		if err != nil {
			return nil, err
		}
	}
	metrics := &RoutePoolMetrics{PoolID: detail.Pool.ID, Group: detail.Pool.Group, Model: modelName}
	scored := make([]scoredRoutePoolCandidate, 0, len(members))
	memberIndexes := make([]int, 0, len(members))
	for _, member := range members {
		metric := RoutePoolMemberMetrics{
			ChannelID:          member.ChannelID,
			Enabled:            member.Enabled,
			CostMultiplier:     member.CostMultiplier,
			ModelCostOverrides: member.ModelCostOverrides,
		}
		channel, channelErr := gatewaystore.GetCachedChannel(member.ChannelID)
		if channelErr == nil && channel != nil {
			metric.ChannelName = channel.Name
			metric.Eligible = member.Enabled && gatewaystore.IsChannelEnabledForGroupModel(detail.Pool.Group, modelName, member.ChannelID)
		}
		if health, found := gatewayruntime.GetChannelHealth(member.ChannelID, modelName); found {
			metric.Health = health
		}
		metrics.Members = append(metrics.Members, metric)
		if metric.Eligible {
			scored = append(scored, scoredRoutePoolCandidate{channel: channel, score: effectiveRoutePoolCost(member, modelName, metric.Health)})
			memberIndexes = append(memberIndexes, len(metrics.Members)-1)
		}
	}
	applyRoutePoolTTFTPenalty(scored, modelName)
	for index, candidate := range scored {
		metrics.Members[memberIndexes[index]].Score = candidate.score
	}
	return metrics, nil
}
