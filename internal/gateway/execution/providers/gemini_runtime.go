package providers

import (
	"github.com/sh2001sh/CodeGo-Api/dto"
	relaycommon "github.com/sh2001sh/CodeGo-Api/internal/gateway/runtime"
)

func ThinkingAdaptor(request *dto.GeminiChatRequest, info *relaycommon.RelayInfo, oaiRequest ...dto.GeneralOpenAIRequest) {
	applyGeminiThinkingAdaptor(request, info, oaiRequest...)
}

func FetchGeminiModels(baseURL string, apiKey string, proxyURL string) ([]string, error) {
	return fetchGeminiModels(baseURL, apiKey, proxyURL)
}
