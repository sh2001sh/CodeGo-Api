package xunfei

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/samber/lo"
	"github.com/sh2001sh/CodeGo-Api/constant"
	"github.com/sh2001sh/CodeGo-Api/dto"
	gatewaystream "github.com/sh2001sh/CodeGo-Api/internal/gateway/stream"
	platformobservability "github.com/sh2001sh/CodeGo-Api/internal/platform/observability"
	platformruntime "github.com/sh2001sh/CodeGo-Api/internal/platform/runtime"
	"github.com/sh2001sh/CodeGo-Api/types"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func requestOpenAI2Xunfei(request dto.GeneralOpenAIRequest, xunfeiAppID string, domain string) *XunfeiChatRequest {
	messages := make([]XunfeiMessage, 0, len(request.Messages))
	shouldCovertSystemMessage := !strings.HasSuffix(request.Model, "3.5")
	for _, message := range request.Messages {
		if message.Role == "system" && shouldCovertSystemMessage {
			messages = append(messages, XunfeiMessage{
				Role:    "user",
				Content: message.StringContent(),
			})
			messages = append(messages, XunfeiMessage{
				Role:    "assistant",
				Content: "Okay",
			})
		} else {
			messages = append(messages, XunfeiMessage{
				Role:    message.Role,
				Content: message.StringContent(),
			})
		}
	}
	xunfeiRequest := XunfeiChatRequest{}
	xunfeiRequest.Header.AppId = xunfeiAppID
	xunfeiRequest.Parameter.Chat.Domain = domain
	xunfeiRequest.Parameter.Chat.Temperature = request.Temperature
	xunfeiRequest.Parameter.Chat.TopK = lo.FromPtrOr(request.N, 0)
	xunfeiRequest.Parameter.Chat.MaxTokens = request.GetMaxTokens()
	xunfeiRequest.Payload.Message.Text = messages
	return &xunfeiRequest
}

func responseXunfei2OpenAI(response *XunfeiChatResponse) *dto.OpenAITextResponse {
	if len(response.Payload.Choices.Text) == 0 {
		response.Payload.Choices.Text = []XunfeiChatResponseTextItem{{Content: ""}}
	}
	choice := dto.OpenAITextResponseChoice{
		Index: 0,
		Message: dto.Message{
			Role:    "assistant",
			Content: response.Payload.Choices.Text[0].Content,
		},
		FinishReason: constant.FinishReasonStop,
	}
	fullTextResponse := dto.OpenAITextResponse{
		Object:  "chat.completion",
		Created: platformruntime.GetTimestamp(),
		Choices: []dto.OpenAITextResponseChoice{choice},
		Usage:   response.Payload.Usage.Text,
	}
	return &fullTextResponse
}

func streamResponseXunfei2OpenAI(xunfeiResponse *XunfeiChatResponse) *dto.ChatCompletionsStreamResponse {
	if len(xunfeiResponse.Payload.Choices.Text) == 0 {
		xunfeiResponse.Payload.Choices.Text = []XunfeiChatResponseTextItem{{Content: ""}}
	}
	var choice dto.ChatCompletionsStreamResponseChoice
	choice.Delta.SetContentString(xunfeiResponse.Payload.Choices.Text[0].Content)
	if xunfeiResponse.Payload.Choices.Status == 2 {
		choice.FinishReason = &constant.FinishReasonStop
	}
	response := dto.ChatCompletionsStreamResponse{
		Object:  "chat.completion.chunk",
		Created: platformruntime.GetTimestamp(),
		Model:   "SparkDesk",
		Choices: []dto.ChatCompletionsStreamResponseChoice{choice},
	}
	return &response
}

func buildXunfeiAuthURL(hostURL string, apiKey, apiSecret string) string {
	hmacWithSHA256ToBase64 := func(data, key string) string {
		mac := hmac.New(sha256.New, []byte(key))
		mac.Write([]byte(data))
		return base64.StdEncoding.EncodeToString(mac.Sum(nil))
	}
	parsedURL, err := url.Parse(hostURL)
	if err != nil {
		fmt.Println(err)
	}
	date := time.Now().UTC().Format(time.RFC1123)
	sign := strings.Join([]string{
		"host: " + parsedURL.Host,
		"date: " + date,
		"GET " + parsedURL.Path + " HTTP/1.1",
	}, "\n")
	sha := hmacWithSHA256ToBase64(sign, apiSecret)
	authURL := fmt.Sprintf(
		"hmac username=\"%s\", algorithm=\"%s\", headers=\"%s\", signature=\"%s\"",
		apiKey,
		"hmac-sha256",
		"host date request-line",
		sha,
	)
	authorization := base64.StdEncoding.EncodeToString([]byte(authURL))
	values := url.Values{}
	values.Add("host", parsedURL.Host)
	values.Add("date", date)
	values.Add("authorization", authorization)
	return hostURL + "?" + values.Encode()
}

func xunfeiStreamHandler(c *gin.Context, textRequest dto.GeneralOpenAIRequest, appID string, apiSecret string, apiKey string) (*dto.Usage, *types.APIError) {
	domain, authURL := getXunfeiAuthURL(c, apiKey, apiSecret, textRequest.Model)
	streamStopped := make(chan struct{})
	defer close(streamStopped)
	dataChan, stopChan, err := xunfeiMakeRequest(textRequest, domain, authURL, appID, streamStopped)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeDoRequestFailed)
	}
	gatewaystream.SetEventStreamHeaders(c)
	var usage dto.Usage
	c.Stream(func(w io.Writer) bool {
		select {
		case xunfeiResponse := <-dataChan:
			usage.PromptTokens += xunfeiResponse.Payload.Usage.Text.PromptTokens
			usage.CompletionTokens += xunfeiResponse.Payload.Usage.Text.CompletionTokens
			usage.TotalTokens += xunfeiResponse.Payload.Usage.Text.TotalTokens
			response := streamResponseXunfei2OpenAI(&xunfeiResponse)
			jsonResponse, err := json.Marshal(response)
			if err != nil {
				platformobservability.SysLog("error marshalling stream response: " + err.Error())
				return true
			}
			if err := gatewaystream.StringData(c, string(jsonResponse)); err != nil {
				platformobservability.SysLog("error writing stream response: " + err.Error())
				return false
			}
			return true
		case <-stopChan:
			gatewaystream.Done(c)
			return false
		case <-c.Request.Context().Done():
			return false
		}
	})
	return &usage, nil
}

func xunfeiHandler(c *gin.Context, textRequest dto.GeneralOpenAIRequest, appID string, apiSecret string, apiKey string) (*dto.Usage, *types.APIError) {
	domain, authURL := getXunfeiAuthURL(c, apiKey, apiSecret, textRequest.Model)
	dataChan, stopChan, err := xunfeiMakeRequest(textRequest, domain, authURL, appID, nil)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeDoRequestFailed)
	}
	var usage dto.Usage
	var content string
	var xunfeiResponse XunfeiChatResponse
	stop := false
	for !stop {
		select {
		case xunfeiResponse = <-dataChan:
			if len(xunfeiResponse.Payload.Choices.Text) == 0 {
				continue
			}
			content += xunfeiResponse.Payload.Choices.Text[0].Content
			usage.PromptTokens += xunfeiResponse.Payload.Usage.Text.PromptTokens
			usage.CompletionTokens += xunfeiResponse.Payload.Usage.Text.CompletionTokens
			usage.TotalTokens += xunfeiResponse.Payload.Usage.Text.TotalTokens
		case stop = <-stopChan:
		}
	}
	if len(xunfeiResponse.Payload.Choices.Text) == 0 {
		xunfeiResponse.Payload.Choices.Text = []XunfeiChatResponseTextItem{{Content: ""}}
	}
	xunfeiResponse.Payload.Choices.Text[0].Content = content

	response := responseXunfei2OpenAI(&xunfeiResponse)
	jsonResponse, err := json.Marshal(response)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeBadResponseBody)
	}
	c.Writer.Header().Set("Content-Type", "application/json")
	_, _ = c.Writer.Write(jsonResponse)
	return &usage, nil
}

func xunfeiMakeRequest(textRequest dto.GeneralOpenAIRequest, domain, authURL, appID string, done <-chan struct{}) (chan XunfeiChatResponse, chan bool, error) {
	dialer := websocket.Dialer{HandshakeTimeout: 5 * time.Second}
	conn, resp, err := dialer.Dial(authURL, nil)
	if err != nil || resp.StatusCode != http.StatusSwitchingProtocols {
		return nil, nil, err
	}

	data := requestOpenAI2Xunfei(textRequest, appID, domain)
	if err := conn.WriteJSON(data); err != nil {
		return nil, nil, err
	}

	dataChan := make(chan XunfeiChatResponse, 1)
	stopChan := make(chan bool, 1)
	go func() {
		defer conn.Close()
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				platformobservability.SysLog("error reading stream response: " + err.Error())
				break
			}
			var response XunfeiChatResponse
			if err := json.Unmarshal(msg, &response); err != nil {
				platformobservability.SysLog("error unmarshalling stream response: " + err.Error())
				break
			}
			select {
			case dataChan <- response:
			case <-done:
				return
			}
			if response.Payload.Choices.Status == 2 {
				break
			}
		}
		select {
		case stopChan <- true:
		case <-done:
		}
	}()

	return dataChan, stopChan, nil
}

func apiVersion2Domain(apiVersion string) string {
	switch apiVersion {
	case "v1.1":
		return "lite"
	case "v2.1":
		return "generalv2"
	case "v3.1":
		return "generalv3"
	case "v3.5":
		return "generalv3.5"
	case "v4.0":
		return "4.0Ultra"
	}
	return "general" + apiVersion
}

func getXunfeiAuthURL(c *gin.Context, apiKey string, apiSecret string, modelName string) (string, string) {
	apiVersion := getAPIVersion(c, modelName)
	domain := apiVersion2Domain(apiVersion)
	authURL := buildXunfeiAuthURL(fmt.Sprintf("wss://spark-api.xf-yun.com/%s/chat", apiVersion), apiKey, apiSecret)
	return domain, authURL
}

func getAPIVersion(c *gin.Context, modelName string) string {
	query := c.Request.URL.Query()
	apiVersion := query.Get("api-version")
	if apiVersion != "" {
		return apiVersion
	}
	parts := strings.Split(modelName, "-")
	if len(parts) == 2 {
		return parts[1]
	}
	apiVersion = c.GetString("api_version")
	if apiVersion != "" {
		return apiVersion
	}
	apiVersion = "v1.1"
	platformobservability.SysLog("api_version not found, using default: " + apiVersion)
	return apiVersion
}
