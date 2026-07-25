package contract

import "strings"

var (
	openAIResponseOnlyModels = []string{
		"o3-pro",
		"o3-deep-research",
		"o4-mini-deep-research",
	}
	imageGenerationModels = []string{
		"dall-e-3",
		"dall-e-2",
		"gpt-image-1",
		"gpt-image-2",
		"prefix:imagen-",
		"flux-",
		"flux.1-",
	}
	openAITextModels = []string{
		"gpt-",
		"o1",
		"o3",
		"o4",
		"chatgpt",
	}
)

// IsOpenAIResponseOnlyModel reports whether a model is only available through the OpenAI Responses API.
func IsOpenAIResponseOnlyModel(modelName string) bool {
	for _, model := range openAIResponseOnlyModels {
		if strings.Contains(modelName, model) {
			return true
		}
	}
	return false
}

// IsImageGenerationModel reports whether a model should use the image-generation endpoint.
func IsImageGenerationModel(modelName string) bool {
	modelName = strings.ToLower(modelName)
	for _, model := range imageGenerationModels {
		if strings.Contains(modelName, model) {
			return true
		}
		if strings.HasPrefix(model, "prefix:") && strings.HasPrefix(modelName, strings.TrimPrefix(model, "prefix:")) {
			return true
		}
	}
	return false
}

// IsOpenAITextModel reports whether a model should use the OpenAI tokenizer path.
func IsOpenAITextModel(modelName string) bool {
	modelName = strings.ToLower(modelName)
	for _, model := range openAITextModels {
		if strings.Contains(modelName, model) {
			return true
		}
	}
	return false
}
