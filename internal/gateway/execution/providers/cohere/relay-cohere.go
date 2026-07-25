package cohere

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"github.com/sh2001sh/CodeGo-Api/dto"
	relaycommon "github.com/sh2001sh/CodeGo-Api/internal/gateway/runtime"
	gatewaystream "github.com/sh2001sh/CodeGo-Api/internal/gateway/stream"
	platformconfig "github.com/sh2001sh/CodeGo-Api/internal/platform/config"
	"github.com/sh2001sh/CodeGo-Api/internal/platform/httpx"
	platformobservability "github.com/sh2001sh/CodeGo-Api/internal/platform/observability"
	platformruntime "github.com/sh2001sh/CodeGo-Api/internal/platform/runtime"
	"github.com/sh2001sh/CodeGo-Api/internal/platform/tokenx"
	"github.com/sh2001sh/CodeGo-Api/types"
	"io"
	"net/http"
	"strings"
	"time"
)

func requestOpenAI2Cohere(textRequest dto.GeneralOpenAIRequest) *CohereRequest {
	cohereReq := CohereRequest{
		Model:       textRequest.Model,
		ChatHistory: []ChatHistory{},
		Message:     "",
		Stream:      lo.FromPtrOr(textRequest.Stream, false),
		MaxTokens:   textRequest.GetMaxTokens(),
	}
	if platformconfig.CohereSafetySetting != "NONE" {
		cohereReq.SafetyMode = platformconfig.CohereSafetySetting
	}
	if cohereReq.MaxTokens == 0 {
		cohereReq.MaxTokens = 4000
	}
	for _, msg := range textRequest.Messages {
		if msg.Role == "user" {
			cohereReq.Message = msg.StringContent()
		} else {
			role := "USER"
			if msg.Role == "assistant" {
				role = "CHATBOT"
			} else if msg.Role == "system" {
				role = "SYSTEM"
			}
			cohereReq.ChatHistory = append(cohereReq.ChatHistory, ChatHistory{
				Role:    role,
				Message: msg.StringContent(),
			})
		}
	}
	return &cohereReq
}

func requestConvertRerank2Cohere(rerankRequest dto.RerankRequest) *CohereRerankRequest {
	topN := lo.FromPtrOr(rerankRequest.TopN, 1)
	if topN <= 0 {
		topN = 1
	}
	return &CohereRerankRequest{
		Query:           rerankRequest.Query,
		Documents:       rerankRequest.Documents,
		Model:           rerankRequest.Model,
		TopN:            topN,
		ReturnDocuments: true,
	}
}

func stopReasonCohereToOpenAI(reason string) string {
	switch reason {
	case "COMPLETE":
		return "stop"
	case "MAX_TOKENS":
		return "max_tokens"
	default:
		return reason
	}
}

func cohereStreamHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.APIError) {
	defer httpx.CloseResponseBodyGracefully(resp)

	responseID := gatewaystream.GetResponseID(c)
	createdTime := platformruntime.GetTimestamp()
	usage := &dto.Usage{}
	var responseText strings.Builder
	scanner := gatewaystream.NewStreamScanner(resp.Body)
	scanner.Split(func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		if atEOF && len(data) == 0 {
			return 0, nil, nil
		}
		if i := strings.Index(string(data), "\n"); i >= 0 {
			return i + 1, data[0:i], nil
		}
		if atEOF {
			return len(data), data, nil
		}
		return 0, nil, nil
	})

	gatewaystream.SetEventStreamHeaders(c)
	isFirst := true
	c.Stream(func(w io.Writer) bool {
		if !scanner.Scan() {
			if scanner.Err() != nil {
				platformobservability.SysLog("error reading stream: " + scanner.Err().Error())
			}
			gatewaystream.Done(c)
			return false
		}

		if isFirst {
			isFirst = false
			info.FirstResponseTime = time.Now()
		}

		data := strings.TrimSuffix(scanner.Text(), "\r")
		var cohereResp CohereResponse
		if err := json.Unmarshal([]byte(data), &cohereResp); err != nil {
			platformobservability.SysLog("error unmarshalling stream response: " + err.Error())
			return true
		}

		openaiResp := dto.ChatCompletionsStreamResponse{
			Id:      responseID,
			Created: createdTime,
			Object:  "chat.completion.chunk",
			Model:   info.UpstreamModelName,
		}
		if cohereResp.IsFinished {
			finishReason := stopReasonCohereToOpenAI(cohereResp.FinishReason)
			openaiResp.Choices = []dto.ChatCompletionsStreamResponseChoice{{
				Delta:        dto.ChatCompletionsStreamResponseChoiceDelta{},
				Index:        0,
				FinishReason: &finishReason,
			}}
			if cohereResp.Response != nil {
				usage.PromptTokens = cohereResp.Response.Meta.BilledUnits.InputTokens
				usage.CompletionTokens = cohereResp.Response.Meta.BilledUnits.OutputTokens
				usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
			}
		} else {
			openaiResp.Choices = []dto.ChatCompletionsStreamResponseChoice{{
				Delta: dto.ChatCompletionsStreamResponseChoiceDelta{
					Role:    "assistant",
					Content: &cohereResp.Text,
				},
				Index: 0,
			}}
			responseText.WriteString(cohereResp.Text)
		}

		jsonStr, err := json.Marshal(openaiResp)
		if err != nil {
			platformobservability.SysLog("error marshalling stream response: " + err.Error())
			return true
		}
		if err := gatewaystream.StringData(c, string(jsonStr)); err != nil {
			platformobservability.SysLog("error writing stream response: " + err.Error())
			return false
		}
		return !cohereResp.IsFinished
	})

	if usage.PromptTokens == 0 {
		usage = tokenx.ResponseText2Usage(c, responseText.String(), info.UpstreamModelName, info.GetEstimatePromptTokens())
	}
	return usage, nil
}

func cohereHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.APIError) {
	createdTime := platformruntime.GetTimestamp()
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeBadResponseBody)
	}
	httpx.CloseResponseBodyGracefully(resp)

	var cohereResp CohereResponseResult
	if err := json.Unmarshal(responseBody, &cohereResp); err != nil {
		return nil, types.NewError(err, types.ErrorCodeBadResponseBody)
	}

	usage := dto.Usage{
		PromptTokens:     cohereResp.Meta.BilledUnits.InputTokens,
		CompletionTokens: cohereResp.Meta.BilledUnits.OutputTokens,
		TotalTokens:      cohereResp.Meta.BilledUnits.InputTokens + cohereResp.Meta.BilledUnits.OutputTokens,
	}
	openaiResp := dto.TextResponse{
		Id:      cohereResp.ResponseID,
		Created: createdTime,
		Object:  "chat.completion",
		Model:   info.UpstreamModelName,
		Usage:   usage,
		Choices: []dto.OpenAITextResponseChoice{{
			Index:        0,
			Message:      dto.Message{Content: cohereResp.Text, Role: "assistant"},
			FinishReason: stopReasonCohereToOpenAI(cohereResp.FinishReason),
		}},
	}

	jsonResponse, err := json.Marshal(openaiResp)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeBadResponseBody)
	}
	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.WriteHeader(resp.StatusCode)
	_, _ = c.Writer.Write(jsonResponse)
	return &usage, nil
}

func cohereRerankHandler(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (*dto.Usage, *types.APIError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeBadResponseBody)
	}
	httpx.CloseResponseBodyGracefully(resp)

	var cohereResp CohereRerankResponseResult
	if err := json.Unmarshal(responseBody, &cohereResp); err != nil {
		return nil, types.NewError(err, types.ErrorCodeBadResponseBody)
	}

	usage := dto.Usage{}
	if cohereResp.Meta.BilledUnits.InputTokens == 0 {
		usage.PromptTokens = info.GetEstimatePromptTokens()
		usage.CompletionTokens = 0
		usage.TotalTokens = info.GetEstimatePromptTokens()
	} else {
		usage.PromptTokens = cohereResp.Meta.BilledUnits.InputTokens
		usage.CompletionTokens = cohereResp.Meta.BilledUnits.OutputTokens
		usage.TotalTokens = cohereResp.Meta.BilledUnits.InputTokens + cohereResp.Meta.BilledUnits.OutputTokens
	}

	rerankResp := dto.RerankResponse{
		Results: cohereResp.Results,
		Usage:   usage,
	}
	jsonResponse, err := json.Marshal(rerankResp)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeBadResponseBody)
	}
	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.WriteHeader(resp.StatusCode)
	_, _ = c.Writer.Write(jsonResponse)
	return &usage, nil
}
