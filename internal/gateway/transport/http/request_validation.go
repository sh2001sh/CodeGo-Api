package http

import (
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"github.com/sh2001sh/CodeGo-Api/dto"
	gatewaycontract "github.com/sh2001sh/CodeGo-Api/internal/gateway/contract"
	platformencoding "github.com/sh2001sh/CodeGo-Api/internal/platform/encodingx"
	platformhttpx "github.com/sh2001sh/CodeGo-Api/internal/platform/httpx"
	"github.com/sh2001sh/CodeGo-Api/internal/platform/logger"
	platformmath "github.com/sh2001sh/CodeGo-Api/internal/platform/mathx"
	platformruntime "github.com/sh2001sh/CodeGo-Api/internal/platform/runtime"
	platformtext "github.com/sh2001sh/CodeGo-Api/internal/platform/textx"
	"github.com/sh2001sh/CodeGo-Api/types"
	"math"
	"net/url"
	"strconv"
	"strings"
)

func validateMaxTokenField(name string, value *uint) error {
	if value == nil || *value == 0 {
		return nil
	}
	if !platformmath.ValidateOptionalUintWithinRange(value, platformmath.SafeMaxRequestTokens) {
		return fmt.Errorf("%s is invalid", name)
	}
	return nil
}

func validatePositiveCountField(name string, value *int, max int) error {
	if value == nil {
		return nil
	}
	if *value < 0 || *value > max {
		return fmt.Errorf("%s is invalid", name)
	}
	return nil
}

func getAndValidateRequest(c *gin.Context, format types.RelayFormat) (dto.Request, error) {
	relayMode := gatewaycontract.Path2RelayMode(c.Request.URL.Path)

	switch format {
	case types.RelayFormatOpenAI:
		return getAndValidateTextRequest(c, relayMode)
	case types.RelayFormatGemini:
		if strings.Contains(c.Request.URL.Path, ":embedContent") {
			return getAndValidateGeminiEmbeddingRequest(c)
		}
		if strings.Contains(c.Request.URL.Path, ":batchEmbedContents") {
			return getAndValidateGeminiBatchEmbeddingRequest(c)
		}
		return getAndValidateGeminiRequest(c)
	case types.RelayFormatClaude:
		return getAndValidateClaudeRequest(c)
	case types.RelayFormatOpenAIResponses:
		return getAndValidateResponsesRequest(c)
	case types.RelayFormatOpenAIResponsesCompaction:
		return getAndValidateResponsesCompactionRequest(c)
	case types.RelayFormatOpenAIImage:
		return getAndValidOpenAIImageRequest(c, relayMode)
	case types.RelayFormatEmbedding:
		return getAndValidateEmbeddingRequest(c, relayMode)
	case types.RelayFormatRerank:
		return getAndValidateRerankRequest(c)
	case types.RelayFormatOpenAIAudio:
		return getAndValidAudioRequest(c, relayMode)
	case types.RelayFormatOpenAIRealtime:
		return &dto.BaseRequest{}, nil
	default:
		return nil, fmt.Errorf("unsupported relay format: %s", format)
	}
}

func getAndValidAudioRequest(c *gin.Context, relayMode int) (*dto.AudioRequest, error) {
	audioRequest := &dto.AudioRequest{}
	if err := platformhttpx.UnmarshalBodyReusable(c, audioRequest); err != nil {
		return nil, err
	}
	switch relayMode {
	case gatewaycontract.RelayModeAudioSpeech:
		if audioRequest.Model == "" {
			return nil, errors.New("model is required")
		}
	default:
		if audioRequest.Model == "" {
			return nil, errors.New("model is required")
		}
		if audioRequest.ResponseFormat == "" {
			audioRequest.ResponseFormat = "json"
		}
	}
	return audioRequest, nil
}

func getAndValidateRerankRequest(c *gin.Context) (*dto.RerankRequest, error) {
	var rerankRequest *dto.RerankRequest
	if err := platformhttpx.UnmarshalBodyReusable(c, &rerankRequest); err != nil {
		logger.LogError(c, fmt.Sprintf("getAndValidateTextRequest failed: %s", err.Error()))
		return nil, types.NewError(err, types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}
	if rerankRequest.Query == "" {
		return nil, types.NewError(fmt.Errorf("query is empty"), types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}
	if len(rerankRequest.Documents) == 0 {
		return nil, types.NewError(fmt.Errorf("documents is empty"), types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}
	return rerankRequest, nil
}

func getAndValidateEmbeddingRequest(c *gin.Context, relayMode int) (*dto.EmbeddingRequest, error) {
	var embeddingRequest *dto.EmbeddingRequest
	if err := platformhttpx.UnmarshalBodyReusable(c, &embeddingRequest); err != nil {
		logger.LogError(c, fmt.Sprintf("getAndValidateTextRequest failed: %s", err.Error()))
		return nil, types.NewError(err, types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}
	if embeddingRequest.Input == nil {
		return nil, fmt.Errorf("input is empty")
	}
	if relayMode == gatewaycontract.RelayModeModerations && embeddingRequest.Model == "" {
		embeddingRequest.Model = "omni-moderation-latest"
	}
	if relayMode == gatewaycontract.RelayModeEmbeddings && embeddingRequest.Model == "" {
		embeddingRequest.Model = c.Param("model")
	}
	return embeddingRequest, nil
}

func getAndValidateResponsesRequest(c *gin.Context) (*dto.OpenAIResponsesRequest, error) {
	request := &dto.OpenAIResponsesRequest{}
	if err := platformhttpx.UnmarshalBodyReusable(c, request); err != nil {
		return nil, err
	}
	if request.Model == "" {
		return nil, errors.New("model is required")
	}
	if request.Input == nil {
		return nil, errors.New("input is required")
	}
	if err := validateMaxTokenField("max_output_tokens", request.MaxOutputTokens); err != nil {
		return nil, err
	}
	if !platformmath.ValidateOptionalUintWithinRange(request.MaxToolCalls, platformmath.SafeMaxToolCallCount) {
		return nil, errors.New("max_tool_calls is invalid")
	}
	request.Stream = platformruntime.GetPointer(true)
	return request, nil
}

func getAndValidateResponsesCompactionRequest(c *gin.Context) (*dto.OpenAIResponsesCompactionRequest, error) {
	request := &dto.OpenAIResponsesCompactionRequest{}
	if err := platformhttpx.UnmarshalBodyReusable(c, request); err != nil {
		return nil, err
	}
	if request.Model == "" {
		return nil, errors.New("model is required")
	}
	return request, nil
}

func getAndValidOpenAIImageRequest(c *gin.Context, relayMode int) (*dto.ImageRequest, error) {
	imageRequest := &dto.ImageRequest{}

	switch relayMode {
	case gatewaycontract.RelayModeImagesEdits:
		if strings.Contains(c.Request.Header.Get("Content-Type"), "multipart/form-data") {
			form, err := platformhttpx.ParseMultipartFormReusable(c)
			if err != nil {
				return nil, fmt.Errorf("failed to parse image edit form request: %w", err)
			}
			formData := url.Values(form.Value)
			c.Request.MultipartForm = form
			c.Request.PostForm = formData
			imageRequest.Prompt = formData.Get("prompt")
			imageRequest.Model = formData.Get("model")
			imageRequest.N = platformruntime.GetPointer(uint(platformtext.String2Int(formData.Get("n"))))
			imageRequest.Quality = formData.Get("quality")
			imageRequest.Size = formData.Get("size")
			if streamValue := strings.TrimSpace(formData.Get("stream")); streamValue != "" {
				stream, err := strconv.ParseBool(streamValue)
				if err != nil {
					return nil, fmt.Errorf("invalid stream value: %w", err)
				}
				imageRequest.Stream = stream
			}
			if imageValue := formData.Get("image"); imageValue != "" {
				imageRequest.Image, _ = platformencoding.Marshal(imageValue)
			}
			if imageRequest.Model == "gpt-image-1" && imageRequest.Quality == "" {
				imageRequest.Quality = "standard"
			}
			if imageRequest.N == nil || *imageRequest.N == 0 {
				imageRequest.N = platformruntime.GetPointer(uint(1))
			}
			if formData.Has("watermark") {
				watermark := formData.Get("watermark") == "true"
				imageRequest.Watermark = &watermark
			}
			break
		}
		fallthrough
	default:
		if err := platformhttpx.UnmarshalBodyReusable(c, imageRequest); err != nil {
			return nil, err
		}
		if imageRequest.Model == "" {
			return nil, errors.New("model is required")
		}
		if strings.Contains(imageRequest.Size, "脳") {
			return nil, errors.New("size an unexpected error occurred in the parameter, please use 'x' instead of the multiplication sign '脳'")
		}
		if imageRequest.Model == "dall-e-2" || imageRequest.Model == "dall-e" {
			if imageRequest.Size != "" && imageRequest.Size != "256x256" && imageRequest.Size != "512x512" && imageRequest.Size != "1024x1024" {
				return nil, errors.New("size must be one of 256x256, 512x512, or 1024x1024 for dall-e-2 or dall-e")
			}
			if imageRequest.Size == "" {
				imageRequest.Size = "1024x1024"
			}
		} else if imageRequest.Model == "dall-e-3" {
			if imageRequest.Size != "" && imageRequest.Size != "1024x1024" && imageRequest.Size != "1024x1792" && imageRequest.Size != "1792x1024" {
				return nil, errors.New("size must be one of 1024x1024, 1024x1792 or 1792x1024 for dall-e-3")
			}
			if imageRequest.Quality == "" {
				imageRequest.Quality = "standard"
			}
			if imageRequest.Size == "" {
				imageRequest.Size = "1024x1024"
			}
		} else if imageRequest.Model == "gpt-image-1" && imageRequest.Quality == "" {
			imageRequest.Quality = "auto"
		}
		if imageRequest.N == nil || *imageRequest.N == 0 {
			imageRequest.N = platformruntime.GetPointer(uint(1))
		}
		if imageRequest.N != nil && (*imageRequest.N == 0 || *imageRequest.N > dto.MaxImageRequestCount) {
			return nil, errors.New("n must be between 1 and 16")
		}
	}

	return imageRequest, nil
}

func getAndValidateClaudeRequest(c *gin.Context) (*dto.ClaudeRequest, error) {
	textRequest := &dto.ClaudeRequest{}
	if err := platformhttpx.UnmarshalBodyReusable(c, textRequest); err != nil {
		return nil, err
	}
	if textRequest.Messages == nil || len(textRequest.Messages) == 0 {
		return nil, errors.New("field messages is required")
	}
	if textRequest.Model == "" {
		return nil, errors.New("field model is required")
	}
	if err := validateMaxTokenField("max_tokens", textRequest.MaxTokens); err != nil {
		return nil, err
	}
	if err := validateMaxTokenField("max_tokens_to_sample", textRequest.MaxTokensToSample); err != nil {
		return nil, err
	}
	if err := validatePositiveCountField("top_k", textRequest.TopK, platformmath.SafeMaxCandidateCount); err != nil {
		return nil, err
	}
	return textRequest, nil
}

func getAndValidateTextRequest(c *gin.Context, relayMode int) (*dto.GeneralOpenAIRequest, error) {
	textRequest := &dto.GeneralOpenAIRequest{}
	if err := platformhttpx.UnmarshalBodyReusable(c, textRequest); err != nil {
		return nil, err
	}
	if relayMode == gatewaycontract.RelayModeModerations && textRequest.Model == "" {
		textRequest.Model = "text-moderation-latest"
	}
	if relayMode == gatewaycontract.RelayModeEmbeddings && textRequest.Model == "" {
		textRequest.Model = c.Param("model")
	}
	if lo.FromPtrOr(textRequest.MaxTokens, uint(0)) > math.MaxInt32/2 {
		return nil, errors.New("max_tokens is invalid")
	}
	if err := validateMaxTokenField("max_tokens", textRequest.MaxTokens); err != nil {
		return nil, err
	}
	if err := validateMaxTokenField("max_completion_tokens", textRequest.MaxCompletionTokens); err != nil {
		return nil, err
	}
	if err := validatePositiveCountField("n", textRequest.N, platformmath.SafeMaxCandidateCount); err != nil {
		return nil, err
	}
	if err := validatePositiveCountField("top_k", textRequest.TopK, platformmath.SafeMaxCandidateCount); err != nil {
		return nil, err
	}
	if textRequest.Model == "" {
		return nil, errors.New("model is required")
	}
	if textRequest.WebSearchOptions != nil {
		if textRequest.WebSearchOptions.SearchContextSize != "" {
			validSizes := map[string]bool{"high": true, "medium": true, "low": true}
			if !validSizes[textRequest.WebSearchOptions.SearchContextSize] {
				return nil, errors.New("invalid search_context_size, must be one of: high, medium, low")
			}
		} else {
			textRequest.WebSearchOptions.SearchContextSize = "medium"
		}
	}
	switch relayMode {
	case gatewaycontract.RelayModeCompletions:
		if textRequest.Prompt == "" {
			return nil, errors.New("field prompt is required")
		}
	case gatewaycontract.RelayModeChatCompletions:
		if len(textRequest.Messages) == 0 && textRequest.Prefix == nil && textRequest.Suffix == nil {
			return nil, errors.New("field messages is required")
		}
	case gatewaycontract.RelayModeModerations:
		if textRequest.Input == nil || textRequest.Input == "" {
			return nil, errors.New("field input is required")
		}
	case gatewaycontract.RelayModeEdits:
		if textRequest.Instruction == "" {
			return nil, errors.New("field instruction is required")
		}
	}
	return textRequest, nil
}

func getAndValidateGeminiRequest(c *gin.Context) (*dto.GeminiChatRequest, error) {
	request := &dto.GeminiChatRequest{}
	if err := platformhttpx.UnmarshalBodyReusable(c, request); err != nil {
		return nil, err
	}
	if len(request.Contents) == 0 && len(request.Requests) == 0 {
		return nil, errors.New("contents is required")
	}
	if request.GenerationConfig.MaxOutputTokens != nil && !platformmath.ValidateOptionalUintWithinRange(request.GenerationConfig.MaxOutputTokens, platformmath.SafeMaxRequestTokens) {
		return nil, errors.New("max_output_tokens is invalid")
	}
	if err := validatePositiveCountField("candidate_count", request.GenerationConfig.CandidateCount, platformmath.SafeMaxCandidateCount); err != nil {
		return nil, err
	}
	return request, nil
}

func getAndValidateGeminiEmbeddingRequest(c *gin.Context) (*dto.GeminiEmbeddingRequest, error) {
	request := &dto.GeminiEmbeddingRequest{}
	if err := platformhttpx.UnmarshalBodyReusable(c, request); err != nil {
		return nil, err
	}
	return request, nil
}

func getAndValidateGeminiBatchEmbeddingRequest(c *gin.Context) (*dto.GeminiBatchEmbeddingRequest, error) {
	request := &dto.GeminiBatchEmbeddingRequest{}
	if err := platformhttpx.UnmarshalBodyReusable(c, request); err != nil {
		return nil, err
	}
	return request, nil
}
