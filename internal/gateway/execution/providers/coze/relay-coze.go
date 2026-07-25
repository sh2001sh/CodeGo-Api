package coze

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"github.com/sh2001sh/CodeGo-Api/dto"
	"github.com/sh2001sh/CodeGo-Api/internal/gateway/execution/providers/synchttp"
	relaycommon "github.com/sh2001sh/CodeGo-Api/internal/gateway/runtime"
	gatewaystream "github.com/sh2001sh/CodeGo-Api/internal/gateway/stream"
	platformhttpx "github.com/sh2001sh/CodeGo-Api/internal/platform/httpx"
	platformobservability "github.com/sh2001sh/CodeGo-Api/internal/platform/observability"
	platformruntime "github.com/sh2001sh/CodeGo-Api/internal/platform/runtime"
	"github.com/sh2001sh/CodeGo-Api/internal/platform/tokenx"
	"github.com/sh2001sh/CodeGo-Api/types"
	"io"
	"net/http"
	"strings"
)

func convertCozeChatRequest(c *gin.Context, request dto.GeneralOpenAIRequest) *CozeChatRequest {
	var messages []CozeEnterMessage
	for _, message := range request.Messages {
		if message.Role == "user" {
			messages = append(messages, CozeEnterMessage{
				Role:        "user",
				Content:     message.Content,
				ContentType: "text",
			})
		}
	}

	user := request.User
	if len(user) == 0 {
		user = json.RawMessage(gatewaystream.GetResponseID(c))
	}
	return &CozeChatRequest{
		BotId:              c.GetString("bot_id"),
		UserId:             user,
		AdditionalMessages: messages,
		Stream:             lo.FromPtrOr(request.Stream, false),
	}
}

func cozeChatHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.APIError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeBadResponseBody)
	}
	platformhttpx.CloseResponseBodyGracefully(resp)

	var (
		response     dto.TextResponse
		cozeResponse CozeChatDetailResponse
	)
	response.Model = info.UpstreamModelName
	if err := json.Unmarshal(responseBody, &cozeResponse); err != nil {
		return nil, types.NewError(err, types.ErrorCodeBadResponseBody)
	}
	if cozeResponse.Code != 0 {
		return nil, types.NewError(errors.New(cozeResponse.Msg), types.ErrorCodeBadResponseBody)
	}

	usage := dto.Usage{
		PromptTokens:     c.GetInt("coze_input_count"),
		CompletionTokens: c.GetInt("coze_output_count"),
		TotalTokens:      c.GetInt("coze_token_count"),
	}
	response.Usage = usage
	response.Id = gatewaystream.GetResponseID(c)

	var responseContent json.RawMessage
	for _, data := range cozeResponse.Data {
		if data.Type == "answer" {
			responseContent = data.Content
			response.Created = data.CreatedAt
		}
	}
	response.Choices = []dto.OpenAITextResponseChoice{{
		Index:        0,
		Message:      dto.Message{Role: "assistant", Content: responseContent},
		FinishReason: "stop",
	}}

	jsonResponse, err := json.Marshal(response)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeBadResponseBody)
	}
	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.WriteHeader(resp.StatusCode)
	_, _ = c.Writer.Write(jsonResponse)
	return &usage, nil
}

func cozeChatStreamHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.APIError) {
	defer platformhttpx.CloseResponseBodyGracefully(resp)

	scanner := gatewaystream.NewStreamScanner(resp.Body)
	scanner.Split(bufio.ScanLines)
	gatewaystream.SetEventStreamHeaders(c)
	id := gatewaystream.GetResponseID(c)
	var responseText strings.Builder
	usage := &dto.Usage{}

	var currentEvent string
	var currentData string
	for scanner.Scan() {
		if gatewaystream.IsClientGone(c) {
			break
		}

		line := scanner.Text()
		if line == "" {
			if currentEvent != "" && currentData != "" {
				if !handleCozeEvent(c, currentEvent, currentData, &responseText, usage, id, info) {
					break
				}
				currentEvent = ""
				currentData = ""
			}
			continue
		}
		if strings.HasPrefix(line, "event:") {
			currentEvent = strings.TrimSpace(line[6:])
			continue
		}
		if strings.HasPrefix(line, "data:") {
			currentData = strings.TrimSpace(line[5:])
			continue
		}
	}

	if currentEvent != "" && currentData != "" {
		_ = handleCozeEvent(c, currentEvent, currentData, &responseText, usage, id, info)
	}
	if err := scanner.Err(); err != nil {
		return nil, types.NewError(err, types.ErrorCodeBadResponseBody)
	}

	gatewaystream.Done(c)
	if usage.TotalTokens == 0 {
		usage = tokenx.ResponseText2Usage(c, responseText.String(), info.UpstreamModelName, c.GetInt("coze_input_count"))
	}
	return usage, nil
}

func handleCozeEvent(c *gin.Context, event string, data string, responseText *strings.Builder, usage *dto.Usage, id string, info *relaycommon.RelayInfo) bool {
	if gatewaystream.IsClientGone(c) {
		return false
	}

	switch event {
	case "conversation.chat.completed":
		var chatData CozeChatResponseData
		if err := json.Unmarshal([]byte(data), &chatData); err != nil {
			platformobservability.SysLog("error_unmarshalling_stream_response: " + err.Error())
			return true
		}

		usage.PromptTokens = chatData.Usage.InputCount
		usage.CompletionTokens = chatData.Usage.OutputCount
		usage.TotalTokens = chatData.Usage.TokenCount

		stopResponse := gatewaystream.GenerateStopResponse(id, platformruntime.GetTimestamp(), info.UpstreamModelName, "stop")
		if err := gatewaystream.ObjectData(c, stopResponse); err != nil {
			platformobservability.SysLog("error_writing_stream_response: " + err.Error())
			return false
		}

	case "conversation.message.delta":
		var messageData CozeChatV3MessageDetail
		if err := json.Unmarshal([]byte(data), &messageData); err != nil {
			platformobservability.SysLog("error_unmarshalling_stream_response: " + err.Error())
			return true
		}

		var content string
		if err := json.Unmarshal(messageData.Content, &content); err != nil {
			platformobservability.SysLog("error_unmarshalling_stream_response: " + err.Error())
			return true
		}
		responseText.WriteString(content)

		openAIResponse := dto.ChatCompletionsStreamResponse{
			Id:      id,
			Object:  "chat.completion.chunk",
			Created: platformruntime.GetTimestamp(),
			Model:   info.UpstreamModelName,
		}
		choice := dto.ChatCompletionsStreamResponseChoice{Index: 0}
		choice.Delta.SetContentString(content)
		openAIResponse.Choices = append(openAIResponse.Choices, choice)
		if err := gatewaystream.ObjectData(c, openAIResponse); err != nil {
			platformobservability.SysLog("error_writing_stream_response: " + err.Error())
			return false
		}

	case "error":
		var errorData CozeError
		if err := json.Unmarshal([]byte(data), &errorData); err != nil {
			platformobservability.SysLog("error_unmarshalling_stream_response: " + err.Error())
			return true
		}
		platformobservability.SysLog(fmt.Sprintf("stream event error: %v %v", errorData.Code, errorData.Message))
	}
	return true
}

func checkIfChatComplete(a *Adaptor, c *gin.Context, info *relaycommon.RelayInfo) (error, bool) {
	requestURL := fmt.Sprintf("%s/v3/chat/retrieve?conversation_id=%s&chat_id=%s", info.ChannelBaseUrl, c.GetString("coze_conversation_id"), c.GetString("coze_chat_id"))
	req, err := http.NewRequest(http.MethodGet, requestURL, nil)
	if err != nil {
		return err, false
	}
	if err := a.SetupRequestHeader(c, &req.Header, info); err != nil {
		return err, false
	}
	if err := applyPollingHeaderOverride(req, info, c); err != nil {
		return err, false
	}

	resp, err := doRequest(req, info)
	if err != nil {
		return err, false
	}
	if resp == nil {
		return fmt.Errorf("resp is nil"), false
	}
	defer resp.Body.Close()

	var cozeResponse CozeChatResponse
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body failed: %w", err), false
	}
	if err := json.Unmarshal(responseBody, &cozeResponse); err != nil {
		return fmt.Errorf("unmarshal response body failed: %w", err), false
	}
	if cozeResponse.Data.Status == "completed" {
		c.Set("coze_token_count", cozeResponse.Data.Usage.TokenCount)
		c.Set("coze_output_count", cozeResponse.Data.Usage.OutputCount)
		c.Set("coze_input_count", cozeResponse.Data.Usage.InputCount)
		return nil, true
	}
	if cozeResponse.Data.Status == "failed" || cozeResponse.Data.Status == "canceled" || cozeResponse.Data.Status == "requires_action" {
		return fmt.Errorf("chat status: %s", cozeResponse.Data.Status), false
	}
	return nil, false
}

func getChatDetail(a *Adaptor, c *gin.Context, info *relaycommon.RelayInfo) (*http.Response, error) {
	requestURL := fmt.Sprintf("%s/v3/chat/message/list?conversation_id=%s&chat_id=%s", info.ChannelBaseUrl, c.GetString("coze_conversation_id"), c.GetString("coze_chat_id"))
	req, err := http.NewRequest(http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("new request failed: %w", err)
	}
	if err := a.SetupRequestHeader(c, &req.Header, info); err != nil {
		return nil, fmt.Errorf("setup request header failed: %w", err)
	}
	if err := applyPollingHeaderOverride(req, info, c); err != nil {
		return nil, fmt.Errorf("apply header override failed: %w", err)
	}

	resp, err := doRequest(req, info)
	if err != nil {
		return nil, fmt.Errorf("do request failed: %w", err)
	}
	return resp, nil
}

func applyPollingHeaderOverride(req *http.Request, info *relaycommon.RelayInfo, c *gin.Context) error {
	headerOverride, err := synchttp.ResolveHeaderOverride(info, c)
	if err != nil {
		return err
	}
	synchttp.ApplyHeaderOverrideToRequest(req, headerOverride)
	return nil
}

func doRequest(req *http.Request, info *relaycommon.RelayInfo) (*http.Response, error) {
	var (
		client *http.Client
		err    error
	)
	if info.ChannelSetting.Proxy != "" {
		client, err = platformhttpx.NewProxyHTTPClient(info.ChannelSetting.Proxy)
		if err != nil {
			return nil, fmt.Errorf("new proxy http client failed: %w", err)
		}
	} else {
		client = platformhttpx.GetHTTPClient()
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("client.Do failed: %w", err)
	}
	return resp, nil
}
