package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	bedrockruntimeTypes "github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
	"github.com/aws/smithy-go/auth/bearer"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"github.com/sh2001sh/CodeGo-Api/dto"
	"github.com/sh2001sh/CodeGo-Api/internal/gateway/execution/providers/claude"
	"github.com/sh2001sh/CodeGo-Api/internal/gateway/execution/providers/synchttp"
	relaycommon "github.com/sh2001sh/CodeGo-Api/internal/gateway/runtime"
	gatewaystore "github.com/sh2001sh/CodeGo-Api/internal/gateway/store"
	gatewaystream "github.com/sh2001sh/CodeGo-Api/internal/gateway/stream"
	platformconfig "github.com/sh2001sh/CodeGo-Api/internal/platform/config"
	platformencoding "github.com/sh2001sh/CodeGo-Api/internal/platform/encodingx"
	platformhttpx "github.com/sh2001sh/CodeGo-Api/internal/platform/httpx"
	platformruntime "github.com/sh2001sh/CodeGo-Api/internal/platform/runtime"
	"github.com/sh2001sh/CodeGo-Api/types"
	"io"
	"net/http"
	"strings"
	"time"
)

func getAwsErrorStatusCode(err error) int {
	var httpErr interface{ HTTPStatusCode() int }
	if errors.As(err, &httpErr) {
		return httpErr.HTTPStatusCode()
	}
	return http.StatusInternalServerError
}

func newAwsInvokeContext() (context.Context, context.CancelFunc) {
	if platformconfig.RelayTimeout <= 0 {
		return context.Background(), func() {}
	}
	return context.WithTimeout(context.Background(), time.Duration(platformconfig.RelayTimeout)*time.Second)
}

func newAwsClient(_ *gin.Context, info *relaycommon.RelayInfo) (*bedrockruntime.Client, error) {
	var (
		httpClient *http.Client
		err        error
	)
	if info.ChannelSetting.Proxy != "" {
		httpClient, err = platformhttpx.NewProxyHTTPClient(info.ChannelSetting.Proxy)
		if err != nil {
			return nil, fmt.Errorf("new proxy http client failed: %w", err)
		}
	} else {
		httpClient = platformhttpx.GetHTTPClient()
	}

	awsSecret := strings.Split(info.ApiKey, "|")
	switch len(awsSecret) {
	case 2:
		apiKey := awsSecret[0]
		region := awsSecret[1]
		return bedrockruntime.New(bedrockruntime.Options{
			Region:                  region,
			BearerAuthTokenProvider: bearer.StaticTokenProvider{Token: bearer.Token{Value: apiKey}},
			HTTPClient:              httpClient,
		}), nil
	case 3:
		ak := awsSecret[0]
		sk := awsSecret[1]
		region := awsSecret[2]
		return bedrockruntime.New(bedrockruntime.Options{
			Region:      region,
			Credentials: aws.NewCredentialsCache(credentials.NewStaticCredentialsProvider(ak, sk, "")),
			HTTPClient:  httpClient,
		}), nil
	default:
		return nil, errors.New("invalid aws secret key")
	}
}

func doAwsClientRequest(c *gin.Context, info *relaycommon.RelayInfo, a *Adaptor, requestBody io.Reader) (any, error) {
	awsClient, err := newAwsClient(c, info)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeChannelAwsClientError)
	}
	a.AwsClient = awsClient

	awsModelID := getAwsModelID(info.UpstreamModelName)
	awsRegionPrefix := getAwsRegionPrefix(awsClient.Options().Region)
	if awsModelCanCrossRegion(awsModelID, awsRegionPrefix) {
		awsModelID = awsModelCrossRegion(awsModelID, awsRegionPrefix)
	}

	requestHeader := http.Header{}
	a.SetupRequestHeader(c, &requestHeader, info)
	headerOverride, err := synchttp.ResolveHeaderOverride(info, c)
	if err != nil {
		return nil, err
	}
	for key, value := range headerOverride {
		requestHeader.Set(key, value)
	}

	if isNovaModel(awsModelID) {
		var novaReq *NovaRequest
		if err := platformencoding.DecodeJSON(requestBody, &novaReq); err != nil {
			return nil, types.NewError(errors.Wrap(err, "decode nova request fail"), types.ErrorCodeBadRequestBody)
		}

		awsReq := &bedrockruntime.InvokeModelInput{
			ModelId:     aws.String(awsModelID),
			Accept:      aws.String("application/json"),
			ContentType: aws.String("application/json"),
		}
		reqBody, err := platformencoding.Marshal(novaReq)
		if err != nil {
			return nil, types.NewError(errors.Wrap(err, "marshal nova request"), types.ErrorCodeBadResponseBody)
		}
		awsReq.Body = reqBody
		a.AwsReq = awsReq
		return nil, nil
	}

	awsClaudeReq, err := formatRequest(requestBody, requestHeader)
	if err != nil {
		return nil, types.NewError(errors.Wrap(err, "format aws request fail"), types.ErrorCodeBadRequestBody)
	}

	body, err := buildAWSRequestBody(c, info, awsClaudeReq)
	if err != nil {
		return nil, types.NewError(errors.Wrap(err, "marshal aws request fail"), types.ErrorCodeBadRequestBody)
	}

	if info.IsStream {
		a.AwsReq = &bedrockruntime.InvokeModelWithResponseStreamInput{
			ModelId:     aws.String(awsModelID),
			Accept:      aws.String("application/json"),
			ContentType: aws.String("application/json"),
			Body:        body,
		}
		return nil, nil
	}

	a.AwsReq = &bedrockruntime.InvokeModelInput{
		ModelId:     aws.String(awsModelID),
		Accept:      aws.String("application/json"),
		ContentType: aws.String("application/json"),
		Body:        body,
	}
	return nil, nil
}

func buildAWSRequestBody(c *gin.Context, info *relaycommon.RelayInfo, awsClaudeReq any) ([]byte, error) {
	if gatewaystore.GetGlobalSettings().PassThroughRequestEnabled || info.ChannelSetting.PassThroughBodyEnabled {
		storage, err := platformhttpx.GetBodyStorage(c)
		if err != nil {
			return nil, errors.Wrap(err, "get request body for pass-through fail")
		}
		body, err := storage.Bytes()
		if err != nil {
			return nil, errors.Wrap(err, "get request body bytes fail")
		}
		var data map[string]interface{}
		if err := platformencoding.Unmarshal(body, &data); err != nil {
			return nil, errors.Wrap(err, "pass-through unmarshal request body fail")
		}
		delete(data, "model")
		delete(data, "stream")
		return platformencoding.Marshal(data)
	}
	return platformencoding.Marshal(awsClaudeReq)
}

func getAwsRegionPrefix(awsRegionID string) string {
	parts := strings.Split(awsRegionID, "-")
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}

func awsModelCanCrossRegion(awsModelID, awsRegionPrefix string) bool {
	regionSet, exists := awsModelCanCrossRegionMap[awsModelID]
	return exists && regionSet[awsRegionPrefix]
}

func awsModelCrossRegion(awsModelID, awsRegionPrefix string) string {
	modelPrefix, ok := awsRegionCrossModelPrefixMap[awsRegionPrefix]
	if !ok {
		return awsModelID
	}
	return modelPrefix + "." + awsModelID
}

func getAwsModelID(requestModel string) string {
	if awsModelIDName, ok := awsModelIDMap[requestModel]; ok {
		return awsModelIDName
	}
	return requestModel
}

func awsHandler(c *gin.Context, info *relaycommon.RelayInfo, a *Adaptor) (*types.APIError, *dto.Usage) {
	ctx, cancel := newAwsInvokeContext()
	defer cancel()

	awsResp, err := a.AwsClient.InvokeModel(ctx, a.AwsReq.(*bedrockruntime.InvokeModelInput))
	if err != nil {
		statusCode := getAwsErrorStatusCode(err)
		return types.NewOpenAIError(errors.Wrap(err, "InvokeModel"), types.ErrorCodeAwsInvokeError, statusCode), nil
	}

	claudeInfo := &claude.ClaudeResponseInfo{
		ResponseID:   gatewaystream.GetResponseID(c),
		Created:      platformruntime.GetTimestamp(),
		Model:        info.UpstreamModelName,
		ResponseText: strings.Builder{},
		Usage:        &dto.Usage{},
	}

	if awsResp.ContentType != nil && *awsResp.ContentType != "" {
		c.Writer.Header().Set("Content-Type", *awsResp.ContentType)
	}

	handlerErr := claude.HandleClaudeResponseData(c, info, claudeInfo, nil, awsResp.Body)
	if handlerErr != nil {
		return handlerErr, nil
	}
	return nil, claudeInfo.Usage
}

func awsStreamHandler(c *gin.Context, info *relaycommon.RelayInfo, a *Adaptor) (*types.APIError, *dto.Usage) {
	ctx, cancel := newAwsInvokeContext()
	defer cancel()

	awsResp, err := a.AwsClient.InvokeModelWithResponseStream(ctx, a.AwsReq.(*bedrockruntime.InvokeModelWithResponseStreamInput))
	if err != nil {
		statusCode := getAwsErrorStatusCode(err)
		return types.NewOpenAIError(errors.Wrap(err, "InvokeModelWithResponseStream"), types.ErrorCodeAwsInvokeError, statusCode), nil
	}
	stream := awsResp.GetStream()
	defer stream.Close()

	claudeInfo := &claude.ClaudeResponseInfo{
		ResponseID:   gatewaystream.GetResponseID(c),
		Created:      platformruntime.GetTimestamp(),
		Model:        info.UpstreamModelName,
		ResponseText: strings.Builder{},
		Usage:        &dto.Usage{},
	}

	for event := range stream.Events() {
		switch v := event.(type) {
		case *bedrockruntimeTypes.ResponseStreamMemberChunk:
			info.SetFirstResponseTime()
			respErr := claude.HandleStreamResponseData(c, info, claudeInfo, string(v.Value.Bytes))
			if respErr != nil {
				return respErr, nil
			}
		case *bedrockruntimeTypes.UnknownUnionMember:
			fmt.Println("unknown tag:", v.Tag)
			return types.NewError(errors.New("unknown response type"), types.ErrorCodeInvalidRequest), nil
		default:
			fmt.Println("union is nil or unknown type")
			return types.NewError(errors.New("nil or unknown response type"), types.ErrorCodeInvalidRequest), nil
		}
	}

	claude.HandleStreamFinalResponse(c, info, claudeInfo)
	return nil, claudeInfo.Usage
}

func handleNovaRequest(c *gin.Context, info *relaycommon.RelayInfo, a *Adaptor) (*types.APIError, *dto.Usage) {
	ctx, cancel := newAwsInvokeContext()
	defer cancel()

	awsResp, err := a.AwsClient.InvokeModel(ctx, a.AwsReq.(*bedrockruntime.InvokeModelInput))
	if err != nil {
		statusCode := getAwsErrorStatusCode(err)
		return types.NewOpenAIError(errors.Wrap(err, "InvokeModel"), types.ErrorCodeAwsInvokeError, statusCode), nil
	}

	var novaResp struct {
		Output struct {
			Message struct {
				Content []struct {
					Text string `json:"text"`
				} `json:"content"`
			} `json:"message"`
		} `json:"output"`
		Usage struct {
			InputTokens  int `json:"inputTokens"`
			OutputTokens int `json:"outputTokens"`
			TotalTokens  int `json:"totalTokens"`
		} `json:"usage"`
	}

	if err := json.Unmarshal(awsResp.Body, &novaResp); err != nil {
		return types.NewError(errors.Wrap(err, "unmarshal nova response"), types.ErrorCodeBadResponseBody), nil
	}

	responseText := ""
	if len(novaResp.Output.Message.Content) > 0 {
		responseText = novaResp.Output.Message.Content[0].Text
	}

	response := dto.OpenAITextResponse{
		Id:      gatewaystream.GetResponseID(c),
		Object:  "chat.completion",
		Created: platformruntime.GetTimestamp(),
		Model:   info.UpstreamModelName,
		Choices: []dto.OpenAITextResponseChoice{{
			Index: 0,
			Message: dto.Message{
				Role:    "assistant",
				Content: responseText,
			},
			FinishReason: "stop",
		}},
		Usage: dto.Usage{
			PromptTokens:     novaResp.Usage.InputTokens,
			CompletionTokens: novaResp.Usage.OutputTokens,
			TotalTokens:      novaResp.Usage.TotalTokens,
		},
	}

	c.JSON(http.StatusOK, response)
	return nil, &response.Usage
}
