package openai

import "encoding/json"

var ai360ModelList = []string{
	"360gpt-turbo",
	"360gpt-turbo-responsibility-8k",
	"360gpt-pro",
	"360gpt2-pro",
	"360GPT_S2_V9",
	"embedding-bert-512-v1",
	"embedding_s1_v1",
	"semantic_similarity_s1_v1",
}

const ai360ChannelName = "ai360"

var lingyiwanwuModelList = []string{
	"yi-large",
	"yi-medium",
	"yi-vision",
	"yi-medium-200k",
	"yi-spark",
	"yi-large-rag",
	"yi-large-turbo",
	"yi-large-preview",
	"yi-large-rag-preview",
}

const lingyiwanwuChannelName = "lingyiwanwu"

var xinferenceModelList = []string{
	"bge-reranker-v2-m3",
	"jina-reranker-v2",
}

const xinferenceChannelName = "xinference"
const openrouterChannelName = "openrouter"

type RequestReasoning struct {
	Enabled   bool   `json:"enabled"`
	Effort    string `json:"effort,omitempty"`
	MaxTokens int    `json:"max_tokens,omitempty"`
	Exclude   bool   `json:"exclude,omitempty"`
}

type OpenRouterEnterpriseResponse struct {
	Data    json.RawMessage `json:"data"`
	Success bool            `json:"success"`
}
