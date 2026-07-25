package app

import (
	"encoding/base64"
	httpctx "github.com/sh2001sh/CodeGo-Api/internal/platform/transport/http/httpctx"

	"github.com/gin-gonic/gin"
	"github.com/sh2001sh/CodeGo-Api/constant"
	billingapp "github.com/sh2001sh/CodeGo-Api/internal/billing/app"
	"github.com/sh2001sh/CodeGo-Api/internal/billing/domain/billingexpr"
	gatewayruntime "github.com/sh2001sh/CodeGo-Api/internal/gateway/runtime"
	relaycommon "github.com/sh2001sh/CodeGo-Api/internal/gateway/runtime"
	"github.com/sh2001sh/CodeGo-Api/types"
)

func buildGatewayTextOtherInfo(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, modelRatio, groupRatio, completionRatio float64, cacheTokens int, cacheRatio float64, modelPrice float64, userGroupRatio float64) map[string]interface{} {
	other := map[string]interface{}{
		"model_ratio":      modelRatio,
		"group_ratio":      groupRatio,
		"completion_ratio": completionRatio,
		"cache_tokens":     cacheTokens,
		"cache_ratio":      cacheRatio,
		"model_price":      modelPrice,
		"user_group_ratio": userGroupRatio,
		"frt":              float64(relayInfo.FirstResponseTime.UnixMilli() - relayInfo.StartTime.UnixMilli()),
	}
	if relayInfo.ReasoningEffort != "" {
		other["reasoning_effort"] = relayInfo.ReasoningEffort
	}
	if relayInfo.IsModelMapped {
		other["is_model_mapped"] = true
		other["upstream_model_name"] = relayInfo.UpstreamModelName
	}
	if httpctx.GetContextKeyBool(ctx, constant.ContextKeySystemPromptOverride) {
		other["is_system_prompt_overwritten"] = true
	}

	adminInfo := map[string]interface{}{
		"use_channel": ctx.GetStringSlice("use_channel"),
	}
	if httpctx.GetContextKeyBool(ctx, constant.ContextKeyChannelIsMultiKey) {
		adminInfo["is_multi_key"] = true
		adminInfo["multi_key_index"] = httpctx.GetContextKeyInt(ctx, constant.ContextKeyChannelMultiKeyIndex)
	}
	if httpctx.GetContextKeyBool(ctx, constant.ContextKeyLocalCountTokens) {
		adminInfo["local_count_tokens"] = true
	}
	gatewayruntime.AppendChannelAffinityAdminInfo(ctx, adminInfo)
	other["admin_info"] = adminInfo

	appendGatewayRequestPath(ctx, relayInfo, other)
	appendGatewayRequestConversionChain(relayInfo, other)
	appendGatewayFinalRequestFormat(relayInfo, other)
	appendGatewayBillingInfo(relayInfo, other)
	appendGatewayParamOverrideInfo(relayInfo, other)
	appendGatewayStreamStatus(relayInfo, other)
	return other
}

func injectGatewayTieredBillingInfo(other map[string]interface{}, relayInfo *relaycommon.RelayInfo, result *billingexpr.TieredResult) {
	if relayInfo == nil || other == nil {
		return
	}
	snap := relayInfo.TieredBillingSnapshot
	if snap == nil {
		return
	}
	other["billing_mode"] = "tiered_expr"
	other["expr_b64"] = base64.StdEncoding.EncodeToString([]byte(snap.ExprString))
	if result != nil {
		other["matched_tier"] = result.MatchedTier
	}
}

func appendGatewayRequestPath(ctx *gin.Context, relayInfo *relaycommon.RelayInfo, other map[string]interface{}) {
	if ctx != nil && ctx.Request != nil && ctx.Request.URL != nil && ctx.Request.URL.Path != "" {
		other["request_path"] = ctx.Request.URL.Path
		return
	}
	if relayInfo != nil && relayInfo.RequestURLPath != "" {
		other["request_path"] = relayInfo.RequestURLPath
	}
}

func appendGatewayRequestConversionChain(relayInfo *relaycommon.RelayInfo, other map[string]interface{}) {
	if relayInfo == nil || len(relayInfo.RequestConversionChain) == 0 {
		return
	}
	chain := make([]string, 0, len(relayInfo.RequestConversionChain))
	for _, f := range relayInfo.RequestConversionChain {
		switch f {
		case types.RelayFormatOpenAI:
			chain = append(chain, "OpenAI Compatible")
		case types.RelayFormatClaude:
			chain = append(chain, "Claude Messages")
		case types.RelayFormatGemini:
			chain = append(chain, "Google Gemini")
		case types.RelayFormatOpenAIResponses:
			chain = append(chain, "OpenAI Responses")
		default:
			chain = append(chain, string(f))
		}
	}
	if len(chain) > 0 {
		other["request_conversion"] = chain
	}
}

func appendGatewayFinalRequestFormat(relayInfo *relaycommon.RelayInfo, other map[string]interface{}) {
	if relayInfo != nil && relayInfo.GetFinalRequestRelayFormat() == types.RelayFormatClaude {
		other["claude"] = true
	}
}

func appendGatewayBillingInfo(relayInfo *relaycommon.RelayInfo, other map[string]interface{}) {
	if relayInfo == nil {
		return
	}
	if relayInfo.BillingSource != "" {
		other["billing_source"] = relayInfo.BillingSource
	}
	switch relayInfo.BillingSource {
	case billingapp.BillingSourceWallet:
		other["billing_quota_field"] = "quota"
	}
}

func appendGatewayParamOverrideInfo(relayInfo *relaycommon.RelayInfo, other map[string]interface{}) {
	if relayInfo != nil && len(relayInfo.ParamOverrideAudit) > 0 {
		other["po"] = relayInfo.ParamOverrideAudit
	}
}

func appendGatewayStreamStatus(relayInfo *relaycommon.RelayInfo, other map[string]interface{}) {
	if relayInfo == nil || !relayInfo.IsStream || relayInfo.StreamStatus == nil {
		return
	}
	ss := relayInfo.StreamStatus
	status := "ok"
	if !ss.IsNormalEnd() || ss.HasErrors() {
		status = "error"
	}
	streamInfo := map[string]interface{}{
		"status":     status,
		"end_reason": string(ss.EndReason),
	}
	if ss.EndError != nil {
		streamInfo["end_error"] = ss.EndError.Error()
	}
	if ss.ErrorCount > 0 {
		streamInfo["error_count"] = ss.ErrorCount
	}
	other["stream_status"] = streamInfo
}
