package http

import (
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/sh2001sh/CodeGo-Api/constant"
	platformencoding "github.com/sh2001sh/CodeGo-Api/internal/platform/encodingx"
	platformtext "github.com/sh2001sh/CodeGo-Api/internal/platform/textx"
	httpctx "github.com/sh2001sh/CodeGo-Api/internal/platform/transport/http/httpctx"
	"github.com/sh2001sh/CodeGo-Api/types"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"testing"
)

type relayErrorEnvelope struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

func TestShouldRetryGatewayTimeoutBeforeResponseDelivery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	err := types.NewOpenAIError(errors.New("upstream timeout"), types.ErrorCodeBadResponseStatusCode, 524)

	require.True(t, shouldRetry(ctx, err, 1))
	httpctx.SetContextKey(ctx, constant.ContextKeyResponseBodyDelivered, true)
	require.False(t, shouldRetry(ctx, err, 1))
}

func TestFinalizeRelayErrorMasksChineseUpstreamQuotaLeak(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	apiErr := types.NewOpenAIError(
		errors.New("用户额度不足, 剩余额度: -＄0.038392 (request id: upstream)"),
		types.ErrorCodeBadResponseStatusCode,
		http.StatusForbidden,
	)

	finalizeRelayError(ctx, types.RelayFormatOpenAI, nil, apiErr, "downstream")

	var response relayErrorEnvelope
	require.NoError(t, platformencoding.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, platformtext.UpstreamQuotaGenericMessage+" (request id: downstream)", response.Error.Message)
}

func TestFinalizeRelayErrorKeepsLocalQuotaMessage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	apiErr := types.NewErrorWithStatusCode(
		errors.New("用户额度不足, 剩余额度: ＄0.002290"),
		types.ErrorCodeInsufficientUserQuota,
		http.StatusForbidden,
	)

	finalizeRelayError(ctx, types.RelayFormatOpenAI, nil, apiErr, "downstream")

	var response relayErrorEnvelope
	require.NoError(t, platformencoding.Unmarshal(recorder.Body.Bytes(), &response))
	require.Contains(t, response.Error.Message, "用户额度不足, 剩余额度: ＄0.002290")
	require.NotEqual(t, platformtext.UpstreamQuotaGenericMessage, response.Error.Message)
}

func TestFinalizeRelayErrorHidesLocalChannelSelectionDetails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	apiErr := types.NewError(
		errors.New("分组 plus高不稳定分组 下模型 gpt-5.6-luna 的可用渠道不存在（retry）"),
		types.ErrorCodeGetChannelFailed,
	)

	finalizeRelayError(ctx, types.RelayFormatOpenAI, nil, apiErr, "downstream")

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	var response relayErrorEnvelope
	require.NoError(t, platformencoding.Unmarshal(recorder.Body.Bytes(), &response))
	require.Contains(t, response.Error.Message, types.ModelUnavailableMessage)
	require.NotContains(t, response.Error.Message, "plus高不稳定分组")
}

func TestFinalizeRelayErrorHidesUpstreamChannelAvailabilityDetails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	apiErr := types.NewOpenAIError(
		errors.New("No available channel for model gpt-5.6-luna under group plus高不稳定分组 (distributor) (request id: upstream)"),
		types.ErrorCodeBadResponseStatusCode,
		http.StatusBadGateway,
	)

	finalizeRelayError(ctx, types.RelayFormatOpenAI, nil, apiErr, "downstream")

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	var response relayErrorEnvelope
	require.NoError(t, platformencoding.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, types.ModelUnavailableMessage+" (request id: downstream)", response.Error.Message)
	require.Equal(t, string(types.ErrorCodeGetChannelFailed), response.Error.Code)
	require.NotContains(t, recorder.Body.String(), "plus高不稳定分组")
	require.NotContains(t, recorder.Body.String(), "upstream")
}

func TestFinalizeRelayErrorHidesAnyUpstreamServiceUnavailableMessage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	apiErr := types.NewOpenAIError(
		errors.New("provider capacity temporarily exhausted (trace: upstream)"),
		types.ErrorCodeBadResponseStatusCode,
		http.StatusServiceUnavailable,
	)

	finalizeRelayError(ctx, types.RelayFormatOpenAI, nil, apiErr, "downstream")

	var response relayErrorEnvelope
	require.NoError(t, platformencoding.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, types.ModelUnavailableMessage+" (request id: downstream)", response.Error.Message)
	require.NotContains(t, recorder.Body.String(), "upstream")
}

func TestRelayNotImplementedReturnsOpenAIStyleError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/files", nil)

	RelayNotImplemented(ctx)

	require.Equal(t, http.StatusNotImplemented, recorder.Code)
	var response relayErrorEnvelope
	require.NoError(t, platformencoding.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, "API not implemented", response.Error.Message)
	require.Equal(t, "codego_api_error", response.Error.Type)
	require.Equal(t, "api_not_implemented", response.Error.Code)
}

func TestRelayNotFoundIncludesMethodAndPath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v1/unknown", nil)

	RelayNotFound(ctx)

	require.Equal(t, http.StatusNotFound, recorder.Code)
	var response relayErrorEnvelope
	require.NoError(t, platformencoding.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, "Invalid URL (GET /v1/unknown)", response.Error.Message)
	require.Equal(t, "invalid_request_error", response.Error.Type)
}

func TestPlaygroundRejectsAccessToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/pg/chat/completions", nil)
	ctx.Set("use_access_token", true)

	Playground(ctx)

	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	var response relayErrorEnvelope
	require.NoError(t, platformencoding.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, "暂不支持使用 access token", response.Error.Message)
}

func TestPlaygroundImageRejectsAccessToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/pg/images/generations", nil)
	ctx.Set("use_access_token", true)

	PlaygroundImage(ctx)

	require.Equal(t, http.StatusInternalServerError, recorder.Code)
	var response relayErrorEnvelope
	require.NoError(t, platformencoding.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, "暂不支持使用 access token", response.Error.Message)
}
