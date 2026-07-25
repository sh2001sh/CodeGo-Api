package providers

import (
	"github.com/sh2001sh/CodeGo-Api/constant"
	internalali "github.com/sh2001sh/CodeGo-Api/internal/gateway/execution/providers/ali"
	internalaws "github.com/sh2001sh/CodeGo-Api/internal/gateway/execution/providers/aws"
	internalbaidu "github.com/sh2001sh/CodeGo-Api/internal/gateway/execution/providers/baidu"
	internalbaiduv2 "github.com/sh2001sh/CodeGo-Api/internal/gateway/execution/providers/baidu_v2"
	internalclaude "github.com/sh2001sh/CodeGo-Api/internal/gateway/execution/providers/claude"
	internalcloudflare "github.com/sh2001sh/CodeGo-Api/internal/gateway/execution/providers/cloudflare"
	internalcodex "github.com/sh2001sh/CodeGo-Api/internal/gateway/execution/providers/codex"
	internalcohere "github.com/sh2001sh/CodeGo-Api/internal/gateway/execution/providers/cohere"
	internalcoze "github.com/sh2001sh/CodeGo-Api/internal/gateway/execution/providers/coze"
	internaldeepseek "github.com/sh2001sh/CodeGo-Api/internal/gateway/execution/providers/deepseek"
	internaldify "github.com/sh2001sh/CodeGo-Api/internal/gateway/execution/providers/dify"
	internalgemini "github.com/sh2001sh/CodeGo-Api/internal/gateway/execution/providers/gemini"
	internaljimeng "github.com/sh2001sh/CodeGo-Api/internal/gateway/execution/providers/jimeng"
	internaljina "github.com/sh2001sh/CodeGo-Api/internal/gateway/execution/providers/jina"
	internalminimax "github.com/sh2001sh/CodeGo-Api/internal/gateway/execution/providers/minimax"
	internalmistral "github.com/sh2001sh/CodeGo-Api/internal/gateway/execution/providers/mistral"
	internalmokaai "github.com/sh2001sh/CodeGo-Api/internal/gateway/execution/providers/mokaai"
	internalmoonshot "github.com/sh2001sh/CodeGo-Api/internal/gateway/execution/providers/moonshot"
	internalollama "github.com/sh2001sh/CodeGo-Api/internal/gateway/execution/providers/ollama"
	internalopenai "github.com/sh2001sh/CodeGo-Api/internal/gateway/execution/providers/openai"
	internalpalm "github.com/sh2001sh/CodeGo-Api/internal/gateway/execution/providers/palm"
	internalperplexity "github.com/sh2001sh/CodeGo-Api/internal/gateway/execution/providers/perplexity"
	internalreplicate "github.com/sh2001sh/CodeGo-Api/internal/gateway/execution/providers/replicate"
	internalsiliconflow "github.com/sh2001sh/CodeGo-Api/internal/gateway/execution/providers/siliconflow"
	internalsubmodel "github.com/sh2001sh/CodeGo-Api/internal/gateway/execution/providers/submodel"
	internaltencent "github.com/sh2001sh/CodeGo-Api/internal/gateway/execution/providers/tencent"
	internalvertex "github.com/sh2001sh/CodeGo-Api/internal/gateway/execution/providers/vertex"
	internalvolcengine "github.com/sh2001sh/CodeGo-Api/internal/gateway/execution/providers/volcengine"
	internalxai "github.com/sh2001sh/CodeGo-Api/internal/gateway/execution/providers/xai"
	internalxunfei "github.com/sh2001sh/CodeGo-Api/internal/gateway/execution/providers/xunfei"
	internalzhipu "github.com/sh2001sh/CodeGo-Api/internal/gateway/execution/providers/zhipu"
	internalzhipu4v "github.com/sh2001sh/CodeGo-Api/internal/gateway/execution/providers/zhipu_4v"
)

type SyncAdaptor = Adaptor

func NewSyncAdaptor(apiType int) SyncAdaptor {
	switch apiType {
	case constant.APITypeAli:
		return &internalali.Adaptor{}
	case constant.APITypeAnthropic:
		return &internalclaude.Adaptor{}
	case constant.APITypeBaidu:
		return &internalbaidu.Adaptor{}
	case constant.APITypeGemini:
		return &internalgemini.Adaptor{}
	case constant.APITypeOpenAI:
		return &internalopenai.Adaptor{}
	case constant.APITypePaLM:
		return &internalpalm.Adaptor{}
	case constant.APITypeTencent:
		return &internaltencent.Adaptor{}
	case constant.APITypeXunfei:
		return &internalxunfei.Adaptor{}
	case constant.APITypeZhipu:
		return &internalzhipu.Adaptor{}
	case constant.APITypeZhipuV4:
		return &internalzhipu4v.Adaptor{}
	case constant.APITypeOllama:
		return &internalollama.Adaptor{}
	case constant.APITypePerplexity:
		return &internalperplexity.Adaptor{}
	case constant.APITypeAws:
		return &internalaws.Adaptor{}
	case constant.APITypeCohere:
		return &internalcohere.Adaptor{}
	case constant.APITypeDify:
		return &internaldify.Adaptor{}
	case constant.APITypeJina:
		return &internaljina.Adaptor{}
	case constant.APITypeCloudflare:
		return &internalcloudflare.Adaptor{}
	case constant.APITypeSiliconFlow:
		return &internalsiliconflow.Adaptor{}
	case constant.APITypeVertexAi:
		return &internalvertex.Adaptor{}
	case constant.APITypeMistral:
		return &internalmistral.Adaptor{}
	case constant.APITypeDeepSeek:
		return &internaldeepseek.Adaptor{}
	case constant.APITypeMokaAI:
		return &internalmokaai.Adaptor{}
	case constant.APITypeVolcEngine:
		return &internalvolcengine.Adaptor{}
	case constant.APITypeBaiduV2:
		return &internalbaiduv2.Adaptor{}
	case constant.APITypeOpenRouter:
		return &internalopenai.Adaptor{}
	case constant.APITypeXinference:
		return &internalopenai.Adaptor{}
	case constant.APITypeXai:
		return &internalxai.Adaptor{}
	case constant.APITypeCoze:
		return &internalcoze.Adaptor{}
	case constant.APITypeJimeng:
		return &internaljimeng.Adaptor{}
	case constant.APITypeMoonshot:
		return &internalmoonshot.Adaptor{}
	case constant.APITypeSubmodel:
		return &internalsubmodel.Adaptor{}
	case constant.APITypeMiniMax:
		return &internalminimax.Adaptor{}
	case constant.APITypeReplicate:
		return &internalreplicate.Adaptor{}
	case constant.APITypeCodex:
		return &internalcodex.Adaptor{}
	default:
		return nil
	}
}
