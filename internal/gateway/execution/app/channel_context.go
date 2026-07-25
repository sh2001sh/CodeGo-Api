package app

import (
	"errors"
	httpctx "github.com/sh2001sh/CodeGo-Api/internal/platform/transport/http/httpctx"

	"github.com/gin-gonic/gin"
	"github.com/sh2001sh/CodeGo-Api/constant"
	gatewaydomain "github.com/sh2001sh/CodeGo-Api/internal/gateway/domain"
	gatewayruntime "github.com/sh2001sh/CodeGo-Api/internal/gateway/runtime"
	gatewayschema "github.com/sh2001sh/CodeGo-Api/internal/gateway/schema"
	gatewaystore "github.com/sh2001sh/CodeGo-Api/internal/gateway/store"
	"github.com/sh2001sh/CodeGo-Api/types"
)

// SetupContextForSelectedChannel writes selected-channel metadata into the request context.
func SetupContextForSelectedChannel(c *gin.Context, channel *gatewayschema.Channel, modelName string) *types.APIError {
	c.Set("original_model", modelName)
	if channel == nil {
		return types.NewError(errors.New("channel is nil"), types.ErrorCodeGetChannelFailed, types.ErrOptionWithSkipRetry())
	}

	httpctx.SetContextKey(c, constant.ContextKeyChannelId, channel.Id)
	httpctx.SetContextKey(c, constant.ContextKeyChannelName, channel.Name)
	httpctx.SetContextKey(c, constant.ContextKeyChannelType, channel.Type)
	httpctx.SetContextKey(c, constant.ContextKeyChannelCreateTime, channel.CreatedTime)
	httpctx.SetContextKey(c, constant.ContextKeyChannelSetting, gatewaydomain.GetSettings(channel))
	httpctx.SetContextKey(c, constant.ContextKeyChannelOtherSetting, gatewaydomain.GetOtherSettings(channel))

	paramOverride := gatewaydomain.GetParamOverride(channel)
	headerOverride := gatewaydomain.GetHeaderOverride(channel)
	if mergedParam, applied := gatewayruntime.ApplyChannelAffinityOverrideTemplate(c, paramOverride); applied {
		paramOverride = mergedParam
	}
	httpctx.SetContextKey(c, constant.ContextKeyChannelParamOverride, paramOverride)
	httpctx.SetContextKey(c, constant.ContextKeyChannelHeaderOverride, headerOverride)

	if channel.OpenAIOrganization != nil && *channel.OpenAIOrganization != "" {
		httpctx.SetContextKey(c, constant.ContextKeyChannelOrganization, *channel.OpenAIOrganization)
	}
	httpctx.SetContextKey(c, constant.ContextKeyChannelAutoBan, channel.GetAutoBan())
	httpctx.SetContextKey(c, constant.ContextKeyChannelModelMapping, channel.GetModelMapping())
	httpctx.SetContextKey(c, constant.ContextKeyChannelStatusCodeMapping, channel.GetStatusCodeMapping())

	key, index, apiError := gatewaystore.GetNextEnabledChannelKey(channel)
	if apiError != nil {
		return apiError
	}
	if channel.ChannelInfo.IsMultiKey {
		httpctx.SetContextKey(c, constant.ContextKeyChannelIsMultiKey, true)
		httpctx.SetContextKey(c, constant.ContextKeyChannelMultiKeyIndex, index)
	} else {
		httpctx.SetContextKey(c, constant.ContextKeyChannelIsMultiKey, false)
	}

	httpctx.SetContextKey(c, constant.ContextKeyChannelKey, key)
	httpctx.SetContextKey(c, constant.ContextKeyChannelBaseUrl, channel.GetBaseURL())
	httpctx.SetContextKey(c, constant.ContextKeySystemPromptOverride, false)

	switch channel.Type {
	case constant.ChannelTypeAzure:
		c.Set("api_version", channel.Other)
	case constant.ChannelTypeVertexAi:
		c.Set("region", channel.Other)
	case constant.ChannelTypeXunfei:
		c.Set("api_version", channel.Other)
	case constant.ChannelTypeGemini:
		c.Set("api_version", channel.Other)
	case constant.ChannelTypeAli:
		c.Set("plugin", channel.Other)
	case constant.ChannelCloudflare:
		c.Set("api_version", channel.Other)
	case constant.ChannelTypeMokaAI:
		c.Set("api_version", channel.Other)
	case constant.ChannelTypeCoze:
		c.Set("bot_id", channel.Other)
	}
	return nil
}
