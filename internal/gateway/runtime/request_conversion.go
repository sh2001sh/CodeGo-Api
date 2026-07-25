package runtime

import (
	"github.com/sh2001sh/CodeGo-Api/dto"
	"github.com/sh2001sh/CodeGo-Api/types"
)

// GuessRelayFormatFromRequest infers the protocol format from a typed request payload.
func GuessRelayFormatFromRequest(req any) (types.RelayFormat, bool) {
	switch req.(type) {
	case *dto.GeneralOpenAIRequest, dto.GeneralOpenAIRequest:
		return types.RelayFormatOpenAI, true
	case *dto.OpenAIResponsesRequest, dto.OpenAIResponsesRequest:
		return types.RelayFormatOpenAIResponses, true
	case *dto.ClaudeRequest, dto.ClaudeRequest:
		return types.RelayFormatClaude, true
	case *dto.GeminiChatRequest, dto.GeminiChatRequest:
		return types.RelayFormatGemini, true
	case *dto.EmbeddingRequest, dto.EmbeddingRequest:
		return types.RelayFormatEmbedding, true
	case *dto.RerankRequest, dto.RerankRequest:
		return types.RelayFormatRerank, true
	case *dto.ImageRequest, dto.ImageRequest:
		return types.RelayFormatOpenAIImage, true
	case *dto.AudioRequest, dto.AudioRequest:
		return types.RelayFormatOpenAIAudio, true
	default:
		return "", false
	}
}

// AppendRequestConversionFromRequest records a request-format conversion onto relay info.
func AppendRequestConversionFromRequest(info *RelayInfo, req any) {
	if info == nil {
		return
	}
	format, ok := GuessRelayFormatFromRequest(req)
	if !ok {
		return
	}
	info.AppendRequestConversion(format)
}
