package http

import (
	"context"
	"encoding/base64"
	"errors"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/sh2001sh/CodeGo-Api/constant"
	gatewayexecutionapp "github.com/sh2001sh/CodeGo-Api/internal/gateway/execution/app"
	gatewayschema "github.com/sh2001sh/CodeGo-Api/internal/gateway/schema"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func newCodexEngine() *gin.Engine {
	store := cookie.NewStore([]byte("secret"))
	engine := gin.New()
	engine.Use(sessions.Sessions("session", store))
	return engine
}

func TestStartCodexOAuthForChannelStoresSession(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	baseURL := "https://chatgpt.com"
	channel := gatewayschema.Channel{
		Id:      1,
		Type:    constant.ChannelTypeCodex,
		Key:     `{"access_token":"a","account_id":"b"}`,
		Status:  constant.ChannelStatusEnabled,
		Name:    "codex",
		BaseURL: &baseURL,
		Group:   "default",
		Models:  "codex-mini",
	}
	require.NoError(t, db.Create(&channel).Error)

	restore := gatewayexecutionapp.SetCreateCodexOAuthAuthorizationFlowForTest(func() (*gatewayexecutionapp.CodexOAuthAuthorizationFlow, error) {
		return &gatewayexecutionapp.CodexOAuthAuthorizationFlow{
			State:        "state-1",
			Verifier:     "verifier-1",
			AuthorizeURL: "https://auth.example/start",
		}, nil
	})
	defer restore()

	engine := newCodexEngine()
	engine.POST("/channel/:id/codex/oauth/start", StartCodexOAuthForChannel)
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/channel/1/codex/oauth/start", nil)
	engine.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"authorize_url":"https://auth.example/start"`)
	require.NotEmpty(t, recorder.Header().Get("Set-Cookie"))
}

func TestCompleteCodexOAuthReturnsGeneratedKey(t *testing.T) {
	restoreExchange := gatewayexecutionapp.SetExchangeCodexAuthorizationCodeForTest(func(ctx context.Context, code string, verifier string, proxyURL string) (*gatewayexecutionapp.CodexOAuthTokenResult, error) {
		token := buildJWT(`{"email":"user@example.com","https://api.openai.com/auth":{"chatgpt_account_id":"acct_123"}}`)
		return &gatewayexecutionapp.CodexOAuthTokenResult{
			AccessToken:  token,
			RefreshToken: "refresh-123",
			ExpiresAt:    time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		}, nil
	})
	defer restoreExchange()

	restoreStart := gatewayexecutionapp.SetCreateCodexOAuthAuthorizationFlowForTest(func() (*gatewayexecutionapp.CodexOAuthAuthorizationFlow, error) {
		return &gatewayexecutionapp.CodexOAuthAuthorizationFlow{
			State:        "state-2",
			Verifier:     "verifier-2",
			AuthorizeURL: "https://auth.example/start",
		}, nil
	})
	defer restoreStart()

	engine := newCodexEngine()
	engine.POST("/channel/codex/oauth/start", StartCodexOAuth)
	engine.POST("/channel/codex/oauth/complete", CompleteCodexOAuth)

	startRecorder := httptest.NewRecorder()
	startReq := httptest.NewRequest(http.MethodPost, "/channel/codex/oauth/start", nil)
	engine.ServeHTTP(startRecorder, startReq)
	cookieHeader := startRecorder.Header().Get("Set-Cookie")
	require.NotEmpty(t, cookieHeader)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/channel/codex/oauth/complete", strings.NewReader(`{"input":"https://localhost/callback?code=abc&state=state-2"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Cookie", cookieHeader)
	engine.ServeHTTP(recorder, req)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"message":"generated"`)
	require.Contains(t, recorder.Body.String(), `"account_id":"acct_123"`)
	require.Contains(t, recorder.Body.String(), `"key":"{`)
}

func TestGetCodexChannelUsageReturnsFailureMessage(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	baseURL := "https://chatgpt.com"
	channel := gatewayschema.Channel{
		Id:      1,
		Type:    constant.ChannelTypeCodex,
		Key:     `{"access_token":"token-1","refresh_token":"refresh-1","account_id":"acct_1","type":"codex"}`,
		Status:  constant.ChannelStatusEnabled,
		Name:    "codex",
		BaseURL: &baseURL,
		Group:   "default",
		Models:  "codex-mini",
	}
	require.NoError(t, db.Create(&channel).Error)

	restoreClient := gatewayexecutionapp.SetCodexProxyHTTPClientFactoryForTest(func(proxyURL string) (*http.Client, error) {
		return &http.Client{}, nil
	})
	defer restoreClient()

	restoreFetch := gatewayexecutionapp.SetFetchCodexWhamUsageForTest(func(ctx context.Context, client *http.Client, baseURL string, accessToken string, accountID string) (int, []byte, error) {
		return http.StatusForbidden, []byte(`{"error":"forbidden"}`), nil
	})
	defer restoreFetch()

	restoreRefresh := gatewayexecutionapp.SetRefreshCodexOAuthTokenForTest(func(ctx context.Context, refreshToken string, proxyURL string) (*gatewayexecutionapp.CodexOAuthTokenResult, error) {
		return nil, errors.New("refresh failed")
	})
	defer restoreRefresh()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = gin.Params{{Key: "id", Value: "1"}}
	ctx.Request = httptest.NewRequest(http.MethodGet, "/channel/1/codex/usage", nil)

	GetCodexChannelUsage(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{"success":false,"message":"upstream status: 403","upstream_status":403,"data":{"error":"forbidden"}}`, recorder.Body.String())
}

func buildJWT(payload string) string {
	return "header." + base64.RawURLEncoding.EncodeToString([]byte(payload)) + ".sig"
}
