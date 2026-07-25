package providers

import (
	"github.com/gin-gonic/gin"
	"github.com/sh2001sh/CodeGo-Api/dto"
	relaycommon "github.com/sh2001sh/CodeGo-Api/internal/gateway/runtime"
	gatewaystream "github.com/sh2001sh/CodeGo-Api/internal/gateway/stream"
	gatewaytranslation "github.com/sh2001sh/CodeGo-Api/internal/gateway/translation"
	platformencoding "github.com/sh2001sh/CodeGo-Api/internal/platform/encodingx"
	"github.com/sh2001sh/CodeGo-Api/types"
)

func handleNonOpenAIStreamFormat(c *gin.Context, info *relaycommon.RelayInfo, chunk *dto.ChatCompletionsStreamResponse, streamErr **types.APIError) bool {
	info.SendResponseCount++

	switch info.RelayFormat {
	case types.RelayFormatClaude:
		if chunk.Usage != nil {
			info.ClaudeConvertInfo.Usage = chunk.Usage
		}
		claudeResponses := gatewaytranslation.StreamResponseOpenAI2Claude(chunk, info)
		for _, resp := range claudeResponses {
			if err := gatewaystream.ClaudeData(c, *resp); err != nil {
				*streamErr = types.NewOpenAIError(err, types.ErrorCodeBadResponse, 500)
				return false
			}
		}
		return true
	case types.RelayFormatGemini:
		geminiResponse := gatewaytranslation.StreamResponseOpenAI2Gemini(chunk, info)
		if geminiResponse == nil {
			return true
		}
		geminiResponseStr, err := platformencoding.Marshal(geminiResponse)
		if err != nil {
			*streamErr = types.NewOpenAIError(err, types.ErrorCodeJsonMarshalFailed, 500)
			return false
		}
		if err := gatewaystream.StringData(c, string(geminiResponseStr)); err != nil {
			*streamErr = types.NewOpenAIError(err, types.ErrorCodeBadResponse, 500)
			return false
		}
		return true
	default:
		return true
	}
}

func handleNonOpenAIFinalResponse(c *gin.Context, info *relaycommon.RelayInfo, responseID string, createAt int64, model string, usage *dto.Usage) {
	if usage == nil {
		return
	}
	finalChunk := gatewaystream.GenerateFinalUsageResponse(responseID, createAt, model, *usage)

	switch info.RelayFormat {
	case types.RelayFormatClaude:
		info.ClaudeConvertInfo.Usage = usage
		claudeResponses := gatewaytranslation.StreamResponseOpenAI2Claude(finalChunk, info)
		for _, resp := range claudeResponses {
			_ = gatewaystream.ClaudeData(c, *resp)
		}
		info.ClaudeConvertInfo.Done = true
	case types.RelayFormatGemini:
		geminiResponse := gatewaytranslation.StreamResponseOpenAI2Gemini(finalChunk, info)
		if geminiResponse == nil {
			return
		}
		geminiResponseStr, err := platformencoding.Marshal(geminiResponse)
		if err != nil {
			return
		}
		_ = gatewaystream.StringData(c, string(geminiResponseStr))
	}
}
