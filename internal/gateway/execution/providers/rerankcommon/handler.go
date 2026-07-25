package rerankcommon

import (
	"github.com/gin-gonic/gin"
	"github.com/sh2001sh/CodeGo-Api/constant"
	"github.com/sh2001sh/CodeGo-Api/dto"
	relaycommon "github.com/sh2001sh/CodeGo-Api/internal/gateway/runtime"
	platformconfig "github.com/sh2001sh/CodeGo-Api/internal/platform/config"
	platformencoding "github.com/sh2001sh/CodeGo-Api/internal/platform/encodingx"
	platformhttpx "github.com/sh2001sh/CodeGo-Api/internal/platform/httpx"
	"github.com/sh2001sh/CodeGo-Api/types"
	"io"
	"net/http"
)

type xinRerankResponseDocument struct {
	Document       any     `json:"document,omitempty"`
	Index          int     `json:"index"`
	RelevanceScore float64 `json:"relevance_score"`
}

type xinRerankResponse struct {
	Results []xinRerankResponseDocument `json:"results"`
}

func Handle(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.APIError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}
	platformhttpx.CloseResponseBodyGracefully(resp)
	if platformconfig.DebugEnabled {
		println("reranker response body: ", string(responseBody))
	}

	var rerankResp dto.RerankResponse
	if info.ChannelType == constant.ChannelTypeXinference {
		var upstream xinRerankResponse
		if err = platformencoding.Unmarshal(responseBody, &upstream); err != nil {
			return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
		}
		results := make([]dto.RerankResponseResult, len(upstream.Results))
		for i, result := range upstream.Results {
			respResult := dto.RerankResponseResult{
				Index:          result.Index,
				RelevanceScore: result.RelevanceScore,
			}
			if info.ReturnDocuments {
				var document any
				if result.Document != nil {
					if doc, ok := result.Document.(string); ok {
						if doc == "" {
							document = info.Documents[result.Index]
						} else {
							document = doc
						}
					} else {
						document = result.Document
					}
				}
				respResult.Document = document
			}
			results[i] = respResult
		}
		rerankResp = dto.RerankResponse{
			Results: results,
			Usage: dto.Usage{
				PromptTokens: info.GetEstimatePromptTokens(),
				TotalTokens:  info.GetEstimatePromptTokens(),
			},
		}
	} else {
		if err = platformencoding.Unmarshal(responseBody, &rerankResp); err != nil {
			return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
		}
		rerankResp.Usage.PromptTokens = rerankResp.Usage.TotalTokens
	}

	c.Writer.Header().Set("Content-Type", "application/json")
	c.JSON(http.StatusOK, rerankResp)
	return &rerankResp.Usage, nil
}
