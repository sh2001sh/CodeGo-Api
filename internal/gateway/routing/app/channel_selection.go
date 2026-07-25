package app

import (
	"errors"
	httpctx "github.com/sh2001sh/CodeGo-Api/internal/platform/transport/http/httpctx"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sh2001sh/CodeGo-Api/constant"
	gatewaygroups "github.com/sh2001sh/CodeGo-Api/internal/gateway/groupsettings"
	gatewayruntime "github.com/sh2001sh/CodeGo-Api/internal/gateway/runtime"
	gatewayschema "github.com/sh2001sh/CodeGo-Api/internal/gateway/schema"
	gatewaystore "github.com/sh2001sh/CodeGo-Api/internal/gateway/store"
	platformconfig "github.com/sh2001sh/CodeGo-Api/internal/platform/config"
	"github.com/sh2001sh/CodeGo-Api/internal/platform/logger"
)

// RetryParam carries group/model selection state across relay retries.
type RetryParam struct {
	Ctx          *gin.Context
	TokenGroup   string
	ModelName    string
	Retry        *int
	resetNextTry bool
}

var selectRandomSatisfiedChannel = gatewaystore.GetRandomSatisfiedChannel

func (p *RetryParam) GetRetry() int {
	if p.Retry == nil {
		return 0
	}
	return *p.Retry
}

func (p *RetryParam) SetRetry(retry int) {
	p.Retry = &retry
}

func (p *RetryParam) IncreaseRetry() {
	if p.resetNextTry {
		p.resetNextTry = false
		return
	}
	if p.Retry == nil {
		p.Retry = new(int)
	}
	*p.Retry++
}

func (p *RetryParam) ResetRetryNextTry() {
	p.resetNextTry = true
}

// CacheGetRandomSatisfiedChannel selects an available channel for the current retry round.
func CacheGetRandomSatisfiedChannel(param *RetryParam) (*gatewayschema.Channel, string, error) {
	var channel *gatewayschema.Channel
	var err error
	selectGroup := param.TokenGroup
	userGroup := httpctx.GetContextKeyString(param.Ctx, constant.ContextKeyUserGroup)

	if param.TokenGroup == AutoGroupName {
		if len(gatewaygroups.GetAutoGroups()) == 0 {
			return nil, selectGroup, errors.New("auto groups is not enabled")
		}
		autoGroups := OrderAutoGroups(userGroup, param.ModelName)
		gatewayruntime.UpdateRouteDecisionCandidates(param.Ctx, len(autoGroups))

		startGroupIndex := 0
		crossGroupRetry := httpctx.GetContextKeyBool(param.Ctx, constant.ContextKeyTokenCrossGroupRetry)

		if lastGroupIndex, exists := httpctx.GetContextKey(param.Ctx, constant.ContextKeyAutoGroupIndex); exists {
			if idx, ok := lastGroupIndex.(int); ok {
				startGroupIndex = idx
			}
		}

		for i := startGroupIndex; i < len(autoGroups); i++ {
			autoGroup := autoGroups[i]
			priorityRetry := param.GetRetry()
			if i > startGroupIndex {
				priorityRetry = 0
			}
			logger.LogDebug(param.Ctx, "Auto selecting group: %s, priorityRetry: %d", autoGroup, priorityRetry)

			channel, _ = getHealthySatisfiedChannelWithContext(param.Ctx, autoGroup, param.ModelName, priorityRetry)
			if channel == nil {
				gatewayruntime.ExcludeRouteDecisionCandidate(param.Ctx, "no_healthy_channel")
				logger.LogDebug(param.Ctx, "No available channel in group %s for model %s at priorityRetry %d, trying next group", autoGroup, param.ModelName, priorityRetry)
				httpctx.SetContextKey(param.Ctx, constant.ContextKeyAutoGroupIndex, i+1)
				httpctx.SetContextKey(param.Ctx, constant.ContextKeyAutoGroupRetryIndex, 0)
				param.SetRetry(0)
				continue
			}
			httpctx.SetContextKey(param.Ctx, constant.ContextKeyAutoGroup, autoGroup)
			selectGroup = autoGroup
			gatewayruntime.SelectRouteDecisionCandidate(param.Ctx, autoGroup, channel.Id, false)
			logger.LogDebug(param.Ctx, "Auto selected group: %s", autoGroup)

			if crossGroupRetry && priorityRetry >= platformconfig.RetryTimes {
				logger.LogDebug(param.Ctx, "Current group %s retries exhausted (priorityRetry=%d >= RetryTimes=%d), preparing switch to next group for next retry", autoGroup, priorityRetry, platformconfig.RetryTimes)
				httpctx.SetContextKey(param.Ctx, constant.ContextKeyAutoGroupIndex, i+1)
				param.SetRetry(0)
				param.ResetRetryNextTry()
			} else {
				httpctx.SetContextKey(param.Ctx, constant.ContextKeyAutoGroupIndex, i)
			}
			break
		}
	} else {
		channel, err = getHealthySatisfiedChannelWithContext(param.Ctx, param.TokenGroup, param.ModelName, param.GetRetry())
		if channel != nil {
			gatewayruntime.SelectRouteDecisionCandidate(param.Ctx, param.TokenGroup, channel.Id, false)
		}
		if err != nil {
			return nil, param.TokenGroup, err
		}
	}
	return channel, selectGroup, nil
}
func getHealthySatisfiedChannel(group string, modelName string, retry int) (*gatewayschema.Channel, error) {
	return getHealthySatisfiedChannelWithContext(nil, group, modelName, retry)
}

func getHealthySatisfiedChannelWithContext(c *gin.Context, group string, modelName string, retry int) (*gatewayschema.Channel, error) {
	if channel, managed, err := selectAutomaticPoolChannel(c, group, modelName); err != nil || managed {
		return channel, err
	}
	var degradedCandidate *gatewayschema.Channel
	seenPriorities := make(map[int64]struct{})
	for priorityRetry := retry; priorityRetry < retry+16; priorityRetry++ {
		healthy, degraded, priority, found, err := getHealthySatisfiedChannelAtPriority(group, modelName, priorityRetry)
		if err != nil {
			return nil, err
		}
		if !found {
			break
		}
		if _, seen := seenPriorities[priority]; seen {
			break
		}
		seenPriorities[priority] = struct{}{}
		if healthy != nil {
			return healthy, nil
		}
		if degradedCandidate == nil && degraded != nil {
			degradedCandidate = degraded
		}
	}
	return degradedCandidate, nil
}

func getHealthySatisfiedChannelAtPriority(group string, modelName string, retry int) (healthy *gatewayschema.Channel, degraded *gatewayschema.Channel, priority int64, found bool, err error) {
	const maxSelectionAttempts = 16
	for attempt := 0; attempt < maxSelectionAttempts; attempt++ {
		channel, err := selectRandomSatisfiedChannel(group, modelName, retry)
		if err != nil || channel == nil {
			return nil, degraded, priority, found, err
		}
		if !found {
			priority = channel.GetPriority()
			found = true
		}
		health, healthFound := gatewayruntime.GetChannelHealth(channel.Id, modelName)
		if healthFound && health.State == gatewayruntime.ChannelHealthCooling {
			if !health.CoolingUntil.After(time.Now()) && degraded == nil {
				// When legacy routing is still in use, an expired circuit may be
				// selected only after all healthy candidates are exhausted. Its
				// next successes are counted as recovery probes by channel health.
				degraded = channel
			}
			continue
		}
		if healthFound && health.State == gatewayruntime.ChannelHealthDegraded {
			if degraded == nil {
				degraded = channel
			}
			continue
		}
		return channel, degraded, priority, true, nil
	}
	return nil, degraded, priority, found, nil
}
