package constant

func ChannelTypeToAPIType(channelType int) (int, bool) {
	apiType := -1
	switch channelType {
	case ChannelTypeOpenAI:
		apiType = APITypeOpenAI
	case ChannelTypeAnthropic:
		apiType = APITypeAnthropic
	case ChannelTypeBaidu:
		apiType = APITypeBaidu
	case ChannelTypePaLM:
		apiType = APITypePaLM
	case ChannelTypeZhipu:
		apiType = APITypeZhipu
	case ChannelTypeAli:
		apiType = APITypeAli
	case ChannelTypeXunfei:
		apiType = APITypeXunfei
	case ChannelTypeAIProxyLibrary:
		apiType = APITypeAIProxyLibrary
	case ChannelTypeTencent:
		apiType = APITypeTencent
	case ChannelTypeGemini:
		apiType = APITypeGemini
	case ChannelTypeZhipu_v4:
		apiType = APITypeZhipuV4
	case ChannelTypeOllama:
		apiType = APITypeOllama
	case ChannelTypePerplexity:
		apiType = APITypePerplexity
	case ChannelTypeAws:
		apiType = APITypeAws
	case ChannelTypeCohere:
		apiType = APITypeCohere
	case ChannelTypeDify:
		apiType = APITypeDify
	case ChannelTypeJina:
		apiType = APITypeJina
	case ChannelCloudflare:
		apiType = APITypeCloudflare
	case ChannelTypeSiliconFlow:
		apiType = APITypeSiliconFlow
	case ChannelTypeVertexAi:
		apiType = APITypeVertexAi
	case ChannelTypeMistral:
		apiType = APITypeMistral
	case ChannelTypeDeepSeek:
		apiType = APITypeDeepSeek
	case ChannelTypeMokaAI:
		apiType = APITypeMokaAI
	case ChannelTypeVolcEngine:
		apiType = APITypeVolcEngine
	case ChannelTypeBaiduV2:
		apiType = APITypeBaiduV2
	case ChannelTypeOpenRouter:
		apiType = APITypeOpenRouter
	case ChannelTypeXinference:
		apiType = APITypeXinference
	case ChannelTypeXai:
		apiType = APITypeXai
	case ChannelTypeCoze:
		apiType = APITypeCoze
	case ChannelTypeJimeng:
		apiType = APITypeJimeng
	case ChannelTypeMoonshot:
		apiType = APITypeMoonshot
	case ChannelTypeSubmodel:
		apiType = APITypeSubmodel
	case ChannelTypeMiniMax:
		apiType = APITypeMiniMax
	case ChannelTypeReplicate:
		apiType = APITypeReplicate
	case ChannelTypeCodex:
		apiType = APITypeCodex
	}
	if apiType == -1 {
		return APITypeOpenAI, false
	}
	return apiType, true
}
