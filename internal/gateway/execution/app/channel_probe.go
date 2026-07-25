package app

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"github.com/sh2001sh/CodeGo-Api/constant"
	"github.com/sh2001sh/CodeGo-Api/dto"
	auditapp "github.com/sh2001sh/CodeGo-Api/internal/audit/app"
	auditschema "github.com/sh2001sh/CodeGo-Api/internal/audit/schema"
	gatewaycontract "github.com/sh2001sh/CodeGo-Api/internal/gateway/contract"
	gatewayproviders "github.com/sh2001sh/CodeGo-Api/internal/gateway/execution/providers"
	relaycommon "github.com/sh2001sh/CodeGo-Api/internal/gateway/runtime"
	gatewayschema "github.com/sh2001sh/CodeGo-Api/internal/gateway/schema"
	gatewaystore "github.com/sh2001sh/CodeGo-Api/internal/gateway/store"
	platformencoding "github.com/sh2001sh/CodeGo-Api/internal/platform/encodingx"
	platformhttpx "github.com/sh2001sh/CodeGo-Api/internal/platform/httpx"
	platformobservability "github.com/sh2001sh/CodeGo-Api/internal/platform/observability"
	"github.com/sh2001sh/CodeGo-Api/types"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"
)

func testChannel(channel *gatewayschema.Channel, testModel string, endpointType string, isStream bool) channelTestResult {
	tik := time.Now()

	unsupportedTypes := []int{
		constant.ChannelTypeSunoAPI,
		constant.ChannelTypeKling,
		constant.ChannelTypeJimeng,
		constant.ChannelTypeDoubaoVideo,
		constant.ChannelTypeVidu,
	}
	if lo.Contains(unsupportedTypes, channel.Type) {
		channelTypeName := constant.GetChannelTypeName(channel.Type)
		return channelTestResult{
			localErr: fmt.Errorf("%s channel test is not supported", channelTypeName),
		}
	}

	writer := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(writer)

	testModel = normalizeChannelTestModel(channel, testModel)
	endpointType = normalizeChannelTestEndpoint(channel, testModel, endpointType)
	requestPath := resolveChannelTestRequestPath(channel, testModel, endpointType)
	if strings.HasPrefix(requestPath, "/v1/responses/compact") {
		testModel = gatewaystore.WithCompactModelSuffix(testModel)
	}

	ctx.Request = &http.Request{
		Method: "POST",
		URL:    buildChannelTestRequestURL(requestPath),
		Body:   nil,
		Header: make(http.Header),
	}

	if err := writeGatewayUserCacheToContext(ctx, 1); err != nil {
		return channelTestResult{localErr: err}
	}
	ctx.Set("id", 1)
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set("channel", channel.Type)
	ctx.Set("base_url", channel.GetBaseURL())

	group, _ := loadGatewayUserGroup(1, false)
	ctx.Set("group", group)

	apiError := SetupContextForSelectedChannel(ctx, channel, testModel)
	if apiError != nil {
		return channelTestResult{
			context:  ctx,
			localErr: apiError,
			apiError: apiError,
		}
	}

	relayFormat := resolveChannelTestRelayFormat(endpointType, ctx.Request.URL.Path)
	request := buildTestRequest(testModel, endpointType, channel, isStream)
	info, err := relaycommon.GenRelayInfo(ctx, relayFormat, request, nil)
	if err != nil {
		return channelTestResult{
			context:  ctx,
			localErr: err,
			apiError: types.NewError(err, types.ErrorCodeGenRelayInfoFailed),
		}
	}

	info.IsChannelTest = true
	info.InitChannelMeta(ctx)

	if err = attachTestBillingRequestInput(info, request); err != nil {
		return channelTestResult{
			context:  ctx,
			localErr: err,
			apiError: types.NewError(err, types.ErrorCodeJsonMarshalFailed),
		}
	}
	if err = relaycommon.ModelMappedHelper(ctx, info, request); err != nil {
		return channelTestResult{
			context:  ctx,
			localErr: err,
			apiError: types.NewError(err, types.ErrorCodeChannelModelMappedError),
		}
	}

	testModel = info.UpstreamModelName
	request.SetModelName(testModel)

	apiType, _ := constant.ChannelTypeToAPIType(channel.Type)
	if info.RelayMode == gatewaycontract.RelayModeResponsesCompact &&
		apiType != constant.APITypeOpenAI &&
		apiType != constant.APITypeCodex {
		err = fmt.Errorf("responses compaction test only supports openai/codex channels, got api type %d", apiType)
		return channelTestResult{
			context:  ctx,
			localErr: err,
			apiError: types.NewError(err, types.ErrorCodeInvalidApiType),
		}
	}

	adaptor := gatewayproviders.NewSyncAdaptor(apiType)
	if adaptor == nil {
		err = fmt.Errorf("invalid api type: %d, adaptor is nil", apiType)
		return channelTestResult{
			context:  ctx,
			localErr: err,
			apiError: types.NewError(err, types.ErrorCodeInvalidApiType),
		}
	}

	platformobservability.SysLog(fmt.Sprintf("testing channel %d with model %s , info %+v ", channel.Id, testModel, info.ToString()))

	priceData, err := relaycommon.ModelPriceHelper(ctx, info, 0, request.GetTokenCountMeta())
	if err != nil {
		return channelTestResult{
			context:  ctx,
			localErr: err,
			apiError: types.NewError(err, types.ErrorCodeModelPriceError, types.ErrOptionWithStatusCode(http.StatusBadRequest)),
		}
	}

	adaptor.Init(info)
	convertedRequest, convertErr := convertChannelTestRequest(ctx, info, adaptor, request)
	if convertErr != nil {
		return channelTestResult{
			context:  ctx,
			localErr: convertErr,
			apiError: types.NewError(convertErr, types.ErrorCodeConvertRequestFailed),
		}
	}

	jsonData, err := platformencoding.Marshal(convertedRequest)
	if err != nil {
		return channelTestResult{
			context:  ctx,
			localErr: err,
			apiError: types.NewError(err, types.ErrorCodeJsonMarshalFailed),
		}
	}

	if len(info.ParamOverride) > 0 {
		jsonData, err = relaycommon.ApplyParamOverrideWithRelayInfo(jsonData, info)
		if err != nil {
			if fixedErr, ok := relaycommon.AsParamOverrideReturnError(err); ok {
				return channelTestResult{
					context:  ctx,
					localErr: fixedErr,
					apiError: relaycommon.APIErrorFromParamOverride(fixedErr),
				}
			}
			return channelTestResult{
				context:  ctx,
				localErr: err,
				apiError: types.NewError(err, types.ErrorCodeChannelParamOverrideInvalid),
			}
		}
	}

	requestBody := bytes.NewBuffer(jsonData)
	ctx.Request.Body = io.NopCloser(bytes.NewBuffer(jsonData))
	resp, err := adaptor.DoRequest(ctx, info, requestBody)
	if err != nil {
		return channelTestResult{
			context:  ctx,
			localErr: err,
			apiError: types.NewOpenAIError(err, types.ErrorCodeDoRequestFailed, http.StatusInternalServerError),
		}
	}

	var httpResp *http.Response
	if resp != nil {
		httpResp = resp.(*http.Response)
		if httpResp.StatusCode != http.StatusOK {
			err = platformhttpx.RelayErrorHandler(ctx.Request.Context(), httpResp, true)
			platformobservability.SysError(fmt.Sprintf(
				"channel test bad response: channel_id=%d name=%s type=%d model=%s endpoint_type=%s status=%d err=%v",
				channel.Id,
				channel.Name,
				channel.Type,
				testModel,
				endpointType,
				httpResp.StatusCode,
				err,
			))
			return channelTestResult{
				context:  ctx,
				localErr: err,
				apiError: types.NewOpenAIError(err, types.ErrorCodeBadResponse, http.StatusInternalServerError),
			}
		}
	}

	usageAny, respErr := adaptor.DoResponse(ctx, httpResp, info)
	if respErr != nil {
		return channelTestResult{
			context:  ctx,
			localErr: respErr,
			apiError: respErr,
		}
	}
	usage, usageErr := coerceTestUsage(usageAny, isStream, info.GetEstimatePromptTokens())
	if usageErr != nil {
		return channelTestResult{
			context:  ctx,
			localErr: usageErr,
			apiError: types.NewOpenAIError(usageErr, types.ErrorCodeBadResponseBody, http.StatusInternalServerError),
		}
	}

	result := writer.Result()
	respBody, err := readTestResponseBody(result.Body, isStream)
	if err != nil {
		return channelTestResult{
			context:  ctx,
			localErr: err,
			apiError: types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError),
		}
	}
	if bodyErr := validateTestResponseBody(respBody, isStream); bodyErr != nil {
		return channelTestResult{
			context:  ctx,
			localErr: bodyErr,
			apiError: types.NewOpenAIError(bodyErr, types.ErrorCodeBadResponseBody, http.StatusInternalServerError),
		}
	}

	info.SetEstimatePromptTokens(usage.PromptTokens)
	quota, tieredResult := settleTestQuota(info, priceData, usage)
	tok := time.Now()
	milliseconds := tok.Sub(tik).Milliseconds()
	consumedTime := float64(milliseconds) / 1000.0
	other := buildTestLogOther(ctx, info, priceData, usage, tieredResult)
	auditapp.RecordConsumeLog(ctx, 1, auditschema.RecordConsumeLogParams{
		ChannelId:        channel.Id,
		PromptTokens:     usage.PromptTokens,
		CompletionTokens: usage.CompletionTokens,
		ModelName:        info.OriginModelName,
		TokenName:        "模型测试",
		Quota:            quota,
		Content:          "模型测试",
		UseTimeSeconds:   int(consumedTime),
		IsStream:         info.IsStream,
		Group:            info.UsingGroup,
		Other:            other,
	})
	platformobservability.SysLog(fmt.Sprintf("testing channel #%d, response: \n%s", channel.Id, string(respBody)))
	return channelTestResult{context: ctx}
}

func convertChannelTestRequest(ctx *gin.Context, info *relaycommon.RelayInfo, adaptor gatewayproviders.SyncAdaptor, request dto.Request) (any, error) {
	switch info.RelayMode {
	case gatewaycontract.RelayModeEmbeddings:
		embeddingReq, ok := request.(*dto.EmbeddingRequest)
		if !ok {
			return nil, errors.New("invalid embedding request type")
		}
		return adaptor.ConvertEmbeddingRequest(ctx, info, *embeddingReq)
	case gatewaycontract.RelayModeImagesGenerations:
		imageReq, ok := request.(*dto.ImageRequest)
		if !ok {
			return nil, errors.New("invalid image request type")
		}
		return adaptor.ConvertImageRequest(ctx, info, *imageReq)
	case gatewaycontract.RelayModeRerank:
		rerankReq, ok := request.(*dto.RerankRequest)
		if !ok {
			return nil, errors.New("invalid rerank request type")
		}
		return adaptor.ConvertRerankRequest(ctx, info.RelayMode, *rerankReq)
	case gatewaycontract.RelayModeResponses:
		responseReq, ok := request.(*dto.OpenAIResponsesRequest)
		if !ok {
			return nil, errors.New("invalid response request type")
		}
		return adaptor.ConvertOpenAIResponsesRequest(ctx, info, *responseReq)
	case gatewaycontract.RelayModeResponsesCompact:
		switch req := request.(type) {
		case *dto.OpenAIResponsesCompactionRequest:
			return adaptor.ConvertOpenAIResponsesRequest(ctx, info, dto.OpenAIResponsesRequest{
				Model:              req.Model,
				Input:              req.Input,
				Instructions:       req.Instructions,
				PreviousResponseID: req.PreviousResponseID,
			})
		case *dto.OpenAIResponsesRequest:
			return adaptor.ConvertOpenAIResponsesRequest(ctx, info, *req)
		default:
			return nil, errors.New("invalid response compaction request type")
		}
	default:
		generalReq, ok := request.(*dto.GeneralOpenAIRequest)
		if !ok {
			return nil, errors.New("invalid general request type")
		}
		return adaptor.ConvertOpenAIRequest(ctx, info, generalReq)
	}
}
