package app

import (
	"fmt"
	auditschema "github.com/sh2001sh/CodeGo-Api/internal/audit/schema"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sh2001sh/CodeGo-Api/constant"
	auditapp "github.com/sh2001sh/CodeGo-Api/internal/audit/app"
	relaycommon "github.com/sh2001sh/CodeGo-Api/internal/gateway/runtime"
	platformtext "github.com/sh2001sh/CodeGo-Api/internal/platform/textx"
)

// LogTaskConsumption records task usage logs and aggregate counters after task submission succeeds.
func LogTaskConsumption(c *gin.Context, info *relaycommon.RelayInfo) {
	tokenName := c.GetString("token_name")
	logContent := fmt.Sprintf("操作 %s", info.Action)
	if platformtext.StringsContains(constant.TaskPricePatches, info.OriginModelName) {
		logContent = fmt.Sprintf("%s，按次计费", logContent)
	} else if len(info.PriceData.OtherRatios) > 0 {
		var contents []string
		for key, ratio := range info.PriceData.OtherRatios {
			if ratio != 1.0 {
				contents = append(contents, fmt.Sprintf("%s: %.2f", key, ratio))
			}
		}
		if len(contents) > 0 {
			logContent = fmt.Sprintf("%s, 计算参数：%s", logContent, strings.Join(contents, ", "))
		}
	}

	other := map[string]interface{}{
		"is_task":      true,
		"request_path": c.Request.URL.Path,
		"model_price":  info.PriceData.ModelPrice,
		"group_ratio":  info.PriceData.GroupRatioInfo.GroupRatio,
	}
	if info.PriceData.ModelRatio > 0 {
		other["model_ratio"] = info.PriceData.ModelRatio
	}
	if info.PriceData.GroupRatioInfo.HasSpecialRatio {
		other["user_group_ratio"] = info.PriceData.GroupRatioInfo.GroupSpecialRatio
	}
	if info.IsModelMapped {
		other["is_model_mapped"] = true
		other["upstream_model_name"] = info.UpstreamModelName
	}

	auditapp.RecordConsumeLog(c, info.UserId, auditschema.RecordConsumeLogParams{
		ChannelId: info.ChannelId,
		ModelName: info.OriginModelName,
		TokenName: tokenName,
		Quota:     info.PriceData.Quota,
		Content:   logContent,
		TokenId:   info.TokenId,
		Group:     info.UsingGroup,
		Other:     other,
	})
	RecordUsageStats(info.UserId, info.ChannelId, info.PriceData.Quota)
}
