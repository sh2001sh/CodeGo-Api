package cloudflare

import (
	"bufio"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"github.com/sh2001sh/CodeGo-Api/dto"
	relaycommon "github.com/sh2001sh/CodeGo-Api/internal/gateway/runtime"
	gatewaystream "github.com/sh2001sh/CodeGo-Api/internal/gateway/stream"
	"github.com/sh2001sh/CodeGo-Api/internal/platform/httpx"
	"github.com/sh2001sh/CodeGo-Api/internal/platform/logger"
	"github.com/sh2001sh/CodeGo-Api/internal/platform/tokenx"
	"github.com/sh2001sh/CodeGo-Api/types"
)

func convertCFToCompletionsRequest(textRequest dto.GeneralOpenAIRequest) *CFRequest {
	prompt, _ := textRequest.Prompt.(string)
	return &CFRequest{
		Prompt:      prompt,
		MaxTokens:   textRequest.GetMaxTokens(),
		Stream:      lo.FromPtrOr(textRequest.Stream, false),
		Temperature: textRequest.Temperature,
	}
}

func cfStreamHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*types.APIError, *dto.Usage) {
	scanner := gatewaystream.NewStreamScanner(resp.Body)
	scanner.Split(bufio.ScanLines)

	gatewaystream.SetEventStreamHeaders(c)
	id := gatewaystream.GetResponseID(c)
	var responseText strings.Builder
	isFirst := true

	for scanner.Scan() {
		data := scanner.Text()
		if len(data) < len("data: ") {
			continue
		}
		data = strings.TrimPrefix(data, "data: ")
		data = strings.TrimSuffix(data, "\r")
		if data == "[DONE]" {
			break
		}

		var response dto.ChatCompletionsStreamResponse
		if err := json.Unmarshal([]byte(data), &response); err != nil {
			logger.LogError(c, "error_unmarshalling_stream_response: "+err.Error())
			continue
		}
		for _, choice := range response.Choices {
			choice.Delta.Role = "assistant"
			responseText.WriteString(choice.Delta.GetContentString())
		}
		response.Id = id
		response.Model = info.UpstreamModelName
		if err := gatewaystream.ObjectData(c, response); err != nil {
			logger.LogError(c, "error_rendering_stream_response: "+err.Error())
		}
		if isFirst {
			isFirst = false
			info.FirstResponseTime = time.Now()
		}
	}

	if err := scanner.Err(); err != nil {
		logger.LogError(c, "error_scanning_stream_response: "+err.Error())
	}
	usage := tokenx.ResponseText2Usage(c, responseText.String(), info.UpstreamModelName, info.GetEstimatePromptTokens())
	if info.ShouldIncludeUsage {
		response := gatewaystream.GenerateFinalUsageResponse(id, info.StartTime.Unix(), info.UpstreamModelName, *usage)
		if err := gatewaystream.ObjectData(c, response); err != nil {
			logger.LogError(c, "error_rendering_final_usage_response: "+err.Error())
		}
	}
	gatewaystream.Done(c)
	httpx.CloseResponseBodyGracefully(resp)
	return nil, usage
}

func cfHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*types.APIError, *dto.Usage) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return types.NewError(err, types.ErrorCodeBadResponseBody), nil
	}
	httpx.CloseResponseBodyGracefully(resp)

	var response dto.TextResponse
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return types.NewError(err, types.ErrorCodeBadResponseBody), nil
	}
	response.Model = info.UpstreamModelName
	var responseText strings.Builder
	for _, choice := range response.Choices {
		responseText.WriteString(choice.Message.StringContent())
	}
	usage := tokenx.ResponseText2Usage(c, responseText.String(), info.UpstreamModelName, info.GetEstimatePromptTokens())
	response.Usage = *usage
	response.Id = gatewaystream.GetResponseID(c)

	jsonResponse, err := json.Marshal(response)
	if err != nil {
		return types.NewError(err, types.ErrorCodeBadResponseBody), nil
	}
	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.WriteHeader(resp.StatusCode)
	_, _ = c.Writer.Write(jsonResponse)
	return nil, usage
}

func cfSTTHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*types.APIError, *dto.Usage) {
	var cfResp CFAudioResponse
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return types.NewError(err, types.ErrorCodeBadResponseBody), nil
	}
	httpx.CloseResponseBodyGracefully(resp)
	if err := json.Unmarshal(responseBody, &cfResp); err != nil {
		return types.NewError(err, types.ErrorCodeBadResponseBody), nil
	}

	audioResp := &dto.AudioResponse{Text: cfResp.Result.Text}
	jsonResponse, err := json.Marshal(audioResp)
	if err != nil {
		return types.NewError(err, types.ErrorCodeBadResponseBody), nil
	}
	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.WriteHeader(resp.StatusCode)
	_, _ = c.Writer.Write(jsonResponse)

	usage := tokenx.ResponseText2Usage(c, cfResp.Result.Text, info.UpstreamModelName, info.GetEstimatePromptTokens())
	return nil, usage
}
