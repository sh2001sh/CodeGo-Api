package palm

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/sh2001sh/CodeGo-Api/constant"
	"github.com/sh2001sh/CodeGo-Api/dto"
	relaycommon "github.com/sh2001sh/CodeGo-Api/internal/gateway/runtime"
	gatewaystream "github.com/sh2001sh/CodeGo-Api/internal/gateway/stream"
	platformencoding "github.com/sh2001sh/CodeGo-Api/internal/platform/encodingx"
	"github.com/sh2001sh/CodeGo-Api/internal/platform/httpx"
	platformruntime "github.com/sh2001sh/CodeGo-Api/internal/platform/runtime"
	"github.com/sh2001sh/CodeGo-Api/internal/platform/tokenx"
	"github.com/sh2001sh/CodeGo-Api/types"
	"io"
	"net/http"
)

func responsePaLMToOpenAI(response *PaLMChatResponse) *dto.OpenAITextResponse {
	fullTextResponse := dto.OpenAITextResponse{
		Choices: make([]dto.OpenAITextResponseChoice, 0, len(response.Candidates)),
	}
	for i, candidate := range response.Candidates {
		choice := dto.OpenAITextResponseChoice{
			Index: i,
			Message: dto.Message{
				Role:    "assistant",
				Content: candidate.Content,
			},
			FinishReason: "stop",
		}
		fullTextResponse.Choices = append(fullTextResponse.Choices, choice)
	}
	return &fullTextResponse
}

func streamResponsePaLMToOpenAI(palmResponse *PaLMChatResponse) *dto.ChatCompletionsStreamResponse {
	var choice dto.ChatCompletionsStreamResponseChoice
	if len(palmResponse.Candidates) > 0 {
		choice.Delta.SetContentString(palmResponse.Candidates[0].Content)
	}
	choice.FinishReason = &constant.FinishReasonStop
	return &dto.ChatCompletionsStreamResponse{
		Object:  "chat.completion.chunk",
		Model:   "palm2",
		Choices: []dto.ChatCompletionsStreamResponseChoice{choice},
	}
}

func palmStreamHandler(c *gin.Context, resp *http.Response) (*types.APIError, string) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		httpx.CloseResponseBodyGracefully(resp)
		return types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError), ""
	}
	httpx.CloseResponseBodyGracefully(resp)

	var palmResponse PaLMChatResponse
	if err := json.Unmarshal(responseBody, &palmResponse); err != nil {
		return types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError), ""
	}

	responseID := gatewaystream.GetResponseID(c)
	createdTime := platformruntime.GetTimestamp()
	fullTextResponse := streamResponsePaLMToOpenAI(&palmResponse)
	fullTextResponse.Id = responseID
	fullTextResponse.Created = createdTime

	responseText := ""
	if len(palmResponse.Candidates) > 0 {
		responseText = palmResponse.Candidates[0].Content
	}

	jsonResponse, err := json.Marshal(fullTextResponse)
	if err != nil {
		return types.NewError(err, types.ErrorCodeBadResponseBody), ""
	}
	gatewaystream.SetEventStreamHeaders(c)
	if err := gatewaystream.StringData(c, string(jsonResponse)); err != nil {
		return types.NewOpenAIError(err, types.ErrorCodeBadResponse, http.StatusInternalServerError), responseText
	}
	gatewaystream.Done(c)
	return nil, responseText
}

func palmHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.APIError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}
	httpx.CloseResponseBodyGracefully(resp)

	var palmResponse PaLMChatResponse
	if err := json.Unmarshal(responseBody, &palmResponse); err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	if palmResponse.Error.Code != 0 || len(palmResponse.Candidates) == 0 {
		return nil, types.WithOpenAIError(types.OpenAIError{
			Message: palmResponse.Error.Message,
			Type:    palmResponse.Error.Status,
			Param:   "",
			Code:    palmResponse.Error.Code,
		}, resp.StatusCode)
	}

	fullTextResponse := responsePaLMToOpenAI(&palmResponse)
	usage := tokenx.ResponseText2Usage(c, palmResponse.Candidates[0].Content, info.UpstreamModelName, info.GetEstimatePromptTokens())
	fullTextResponse.Usage = *usage

	jsonResponse, err := platformencoding.Marshal(fullTextResponse)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeBadResponseBody)
	}
	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.WriteHeader(resp.StatusCode)
	httpx.IOCopyBytesGracefully(c, resp, jsonResponse)
	return usage, nil
}
