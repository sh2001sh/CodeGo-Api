package tokenx

import (
	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"github.com/sh2001sh/CodeGo-Api/constant"
	"github.com/sh2001sh/CodeGo-Api/dto"
	httpctx "github.com/sh2001sh/CodeGo-Api/internal/platform/transport/http/httpctx"
)

// ResponseText2Usage estimates usage from response text when upstream usage is unavailable.
func ResponseText2Usage(c *gin.Context, responseText string, modelName string, promptTokens int) *dto.Usage {
	httpctx.SetContextKey(c, constant.ContextKeyLocalCountTokens, true)
	usage := &dto.Usage{}
	usage.PromptTokens = promptTokens
	usage.CompletionTokens = EstimateTokenByModel(modelName, responseText)
	usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	return usage
}

// ValidUsage reports whether a usage payload contains non-zero token data.
func ValidUsage(usage *dto.Usage) bool {
	return usage != nil && (usage.PromptTokens != 0 || usage.CompletionTokens != 0)
}

// NormalizeCacheCreationSplit backfills missing 5m cache-creation tokens from the aggregate total.
func NormalizeCacheCreationSplit(totalTokens int, tokens5m int, tokens1h int) (int, int) {
	remainder := lo.Max([]int{totalTokens - tokens5m - tokens1h, 0})
	return tokens5m + remainder, tokens1h
}
