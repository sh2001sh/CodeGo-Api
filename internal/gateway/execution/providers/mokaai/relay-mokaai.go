package mokaai

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/sh2001sh/CodeGo-Api/dto"
	relaycommon "github.com/sh2001sh/CodeGo-Api/internal/gateway/runtime"
	platformencoding "github.com/sh2001sh/CodeGo-Api/internal/platform/encodingx"
	"github.com/sh2001sh/CodeGo-Api/internal/platform/httpx"
	"github.com/sh2001sh/CodeGo-Api/types"
	"io"
	"net/http"
)

func embeddingRequestOpenAI2Moka(request dto.GeneralOpenAIRequest) *dto.EmbeddingRequest {
	var input []string

	switch v := request.Input.(type) {
	case string:
		input = []string{v}
	case []string:
		input = v
	case []interface{}:
		for _, part := range v {
			if str, ok := part.(string); ok {
				input = append(input, str)
			}
		}
	}
	return &dto.EmbeddingRequest{
		Input: input,
		Model: request.Model,
	}
}

func embeddingResponseMokaToOpenAI(response *dto.EmbeddingResponse) *dto.OpenAIEmbeddingResponse {
	openAIEmbeddingResponse := dto.OpenAIEmbeddingResponse{
		Object: "list",
		Data:   make([]dto.OpenAIEmbeddingResponseItem, 0, len(response.Data)),
		Model:  "baidu-embedding",
		Usage:  response.Usage,
	}
	for _, item := range response.Data {
		openAIEmbeddingResponse.Data = append(openAIEmbeddingResponse.Data, dto.OpenAIEmbeddingResponseItem{
			Object:    item.Object,
			Index:     item.Index,
			Embedding: item.Embedding,
		})
	}
	return &openAIEmbeddingResponse
}

func mokaEmbeddingHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.APIError) {
	var mokaResponse dto.EmbeddingResponse
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeBadResponseBody)
	}
	httpx.CloseResponseBodyGracefully(resp)
	if err := json.Unmarshal(responseBody, &mokaResponse); err != nil {
		return nil, types.NewError(err, types.ErrorCodeBadResponseBody)
	}

	fullTextResponse := embeddingResponseMokaToOpenAI(&mokaResponse)
	jsonResponse, err := platformencoding.Marshal(fullTextResponse)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeBadResponseBody)
	}
	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.WriteHeader(resp.StatusCode)
	httpx.IOCopyBytesGracefully(c, resp, jsonResponse)
	return &fullTextResponse.Usage, nil
}
