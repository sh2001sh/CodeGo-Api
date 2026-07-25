package app

import (
	"context"
	"errors"
	"fmt"
	"github.com/sh2001sh/CodeGo-Api/constant"
	gatewaydomain "github.com/sh2001sh/CodeGo-Api/internal/gateway/domain"
	gatewayproviders "github.com/sh2001sh/CodeGo-Api/internal/gateway/execution/providers"
	gatewayschema "github.com/sh2001sh/CodeGo-Api/internal/gateway/schema"
	gatewaystore "github.com/sh2001sh/CodeGo-Api/internal/gateway/store"
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
	platformencoding "github.com/sh2001sh/CodeGo-Api/internal/platform/encodingx"
	platformhttpx "github.com/sh2001sh/CodeGo-Api/internal/platform/httpx"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type codexOAuthFlowFactory func() (*CodexOAuthAuthorizationFlow, error)
type codexAuthorizationCodeExchanger func(ctx context.Context, code string, verifier string, proxyURL string) (*CodexOAuthTokenResult, error)
type codexUsageFetcher func(ctx context.Context, client *http.Client, baseURL string, accessToken string, accountID string) (int, []byte, error)
type codexTokenRefresher func(ctx context.Context, refreshToken string, proxyURL string) (*CodexOAuthTokenResult, error)
type codexHTTPClientFactory func(proxyURL string) (*http.Client, error)

var (
	createCodexOAuthAuthorizationFlow = CreateCodexOAuthAuthorizationFlow
	exchangeCodexAuthorizationCode    = ExchangeCodexAuthorizationCodeWithProxy
	fetchCodexWhamUsage               = FetchCodexWhamUsage
	refreshCodexOAuthToken            = RefreshCodexOAuthTokenWithProxy
	newCodexProxyHTTPClient           = platformhttpx.NewProxyHTTPClient
)

type CodexOAuthStartResult struct {
	AuthorizeURL string `json:"authorize_url"`
	State        string `json:"-"`
	Verifier     string `json:"-"`
}

type CodexOAuthCompleteResult struct {
	Key         string `json:"key,omitempty"`
	AccountID   string `json:"account_id"`
	Email       string `json:"email"`
	ExpiresAt   string `json:"expires_at"`
	LastRefresh string `json:"last_refresh"`
	ChannelID   int    `json:"channel_id,omitempty"`
}

type CodexChannelUsageResult struct {
	Success        bool `json:"success"`
	UpstreamStatus int  `json:"upstream_status"`
	Data           any  `json:"data"`
}

func SetCreateCodexOAuthAuthorizationFlowForTest(factory func() (*CodexOAuthAuthorizationFlow, error)) func() {
	original := createCodexOAuthAuthorizationFlow
	createCodexOAuthAuthorizationFlow = factory
	return func() { createCodexOAuthAuthorizationFlow = original }
}

func SetExchangeCodexAuthorizationCodeForTest(exchanger func(ctx context.Context, code string, verifier string, proxyURL string) (*CodexOAuthTokenResult, error)) func() {
	original := exchangeCodexAuthorizationCode
	exchangeCodexAuthorizationCode = exchanger
	return func() { exchangeCodexAuthorizationCode = original }
}

func SetFetchCodexWhamUsageForTest(fetcher func(ctx context.Context, client *http.Client, baseURL string, accessToken string, accountID string) (int, []byte, error)) func() {
	original := fetchCodexWhamUsage
	fetchCodexWhamUsage = fetcher
	return func() { fetchCodexWhamUsage = original }
}

func SetRefreshCodexOAuthTokenForTest(refresher func(ctx context.Context, refreshToken string, proxyURL string) (*CodexOAuthTokenResult, error)) func() {
	original := refreshCodexOAuthToken
	refreshCodexOAuthToken = refresher
	return func() { refreshCodexOAuthToken = original }
}

func SetCodexProxyHTTPClientFactoryForTest(factory func(proxyURL string) (*http.Client, error)) func() {
	original := newCodexProxyHTTPClient
	newCodexProxyHTTPClient = factory
	return func() { newCodexProxyHTTPClient = original }
}

func CodexOAuthSessionKey(channelID int, field string) string {
	return fmt.Sprintf("codex_oauth_%s_%d", field, channelID)
}

func ParseCodexAuthorizationInput(input string) (code string, state string, err error) {
	value := strings.TrimSpace(input)
	if value == "" {
		return "", "", errors.New("empty input")
	}
	if strings.Contains(value, "#") {
		parts := strings.SplitN(value, "#", 2)
		return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), nil
	}
	if strings.Contains(value, "code=") {
		u, parseErr := url.Parse(value)
		if parseErr == nil {
			query := u.Query()
			return strings.TrimSpace(query.Get("code")), strings.TrimSpace(query.Get("state")), nil
		}
		query, parseErr := url.ParseQuery(value)
		if parseErr == nil {
			return strings.TrimSpace(query.Get("code")), strings.TrimSpace(query.Get("state")), nil
		}
	}
	return value, "", nil
}

func validateCodexChannel(channelID int, selectAll bool) (*gatewayschema.Channel, error) {
	if channelID <= 0 {
		return nil, nil
	}
	channel, err := gatewaystore.LoadChannelByID(channelID, selectAll)
	if err != nil {
		return nil, err
	}
	if channel == nil {
		return nil, errors.New("channel not found")
	}
	if channel.Type != constant.ChannelTypeCodex {
		return nil, errors.New("channel type is not Codex")
	}
	return channel, nil
}

func StartCodexOAuth(channelID int) (*CodexOAuthStartResult, error) {
	if _, err := validateCodexChannel(channelID, false); err != nil {
		return nil, err
	}
	flow, err := createCodexOAuthAuthorizationFlow()
	if err != nil {
		return nil, err
	}
	return &CodexOAuthStartResult{
		AuthorizeURL: flow.AuthorizeURL,
		State:        flow.State,
		Verifier:     flow.Verifier,
	}, nil
}

func GetCodexChannelForOAuth(channelID int) (*gatewayschema.Channel, error) {
	return validateCodexChannel(channelID, false)
}

func CompleteCodexOAuth(channelID int, code string, verifier string, proxyURL string) (*CodexOAuthCompleteResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	tokenRes, err := exchangeCodexAuthorizationCode(ctx, code, verifier, proxyURL)
	if err != nil {
		return nil, err
	}

	accountID, ok := ExtractCodexAccountIDFromJWT(tokenRes.AccessToken)
	if !ok {
		return nil, errors.New("failed to extract account_id from access_token")
	}
	email, _ := ExtractEmailFromJWT(tokenRes.AccessToken)

	key := gatewayproviders.OAuthKey{
		AccessToken:  tokenRes.AccessToken,
		RefreshToken: tokenRes.RefreshToken,
		AccountID:    accountID,
		LastRefresh:  time.Now().Format(time.RFC3339),
		Expired:      tokenRes.ExpiresAt.Format(time.RFC3339),
		Email:        email,
		Type:         "codex",
	}
	encoded, err := platformencoding.Marshal(key)
	if err != nil {
		return nil, err
	}

	result := &CodexOAuthCompleteResult{
		Key:         string(encoded),
		AccountID:   accountID,
		Email:       email,
		ExpiresAt:   key.Expired,
		LastRefresh: key.LastRefresh,
		ChannelID:   channelID,
	}

	if channelID <= 0 {
		return result, nil
	}

	if _, err = validateCodexChannel(channelID, false); err != nil {
		return nil, err
	}
	if err = platformdb.DB.Model(&gatewayschema.Channel{}).Where("id = ?", channelID).Update("key", string(encoded)).Error; err != nil {
		return nil, err
	}
	gatewaystore.InitChannelCache()
	platformhttpx.ResetProxyClientCache()
	result.Key = ""
	return result, nil
}

func GetCodexChannelUsage(channelID int) (*CodexChannelUsageResult, error) {
	channel, err := validateCodexChannel(channelID, true)
	if err != nil {
		return nil, err
	}
	if channel.ChannelInfo.IsMultiKey {
		return nil, errors.New("multi-key channel is not supported")
	}

	oauthKey, err := gatewayproviders.ParseOAuthKey(strings.TrimSpace(channel.Key))
	if err != nil {
		return nil, errors.New("解析凭证失败，请检查渠道配置")
	}

	accessToken := strings.TrimSpace(oauthKey.AccessToken)
	accountID := strings.TrimSpace(oauthKey.AccountID)
	if accessToken == "" {
		return nil, errors.New("codex channel: access_token is required")
	}
	if accountID == "" {
		return nil, errors.New("codex channel: account_id is required")
	}

	client, err := newCodexProxyHTTPClient(gatewaydomain.GetSettings(channel).Proxy)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	statusCode, body, err := fetchCodexWhamUsage(ctx, client, channel.GetBaseURL(), accessToken, accountID)
	if err != nil {
		return nil, err
	}

	if (statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden) && strings.TrimSpace(oauthKey.RefreshToken) != "" {
		refreshCtx, refreshCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer refreshCancel()

		refreshRes, refreshErr := refreshCodexOAuthToken(refreshCtx, oauthKey.RefreshToken, gatewaydomain.GetSettings(channel).Proxy)
		if refreshErr == nil {
			oauthKey.AccessToken = refreshRes.AccessToken
			oauthKey.RefreshToken = refreshRes.RefreshToken
			oauthKey.LastRefresh = time.Now().Format(time.RFC3339)
			oauthKey.Expired = refreshRes.ExpiresAt.Format(time.RFC3339)
			if strings.TrimSpace(oauthKey.Type) == "" {
				oauthKey.Type = "codex"
			}

			if encoded, encodeErr := platformencoding.Marshal(oauthKey); encodeErr == nil {
				_ = platformdb.DB.Model(&gatewayschema.Channel{}).Where("id = ?", channel.Id).Update("key", string(encoded)).Error
				gatewaystore.InitChannelCache()
				platformhttpx.ResetProxyClientCache()
			}

			ctx2, cancel2 := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel2()
			statusCode, body, err = fetchCodexWhamUsage(ctx2, client, channel.GetBaseURL(), oauthKey.AccessToken, accountID)
			if err != nil {
				return nil, err
			}
		}
	}

	var payload any
	if platformencoding.Unmarshal(body, &payload) != nil {
		payload = string(body)
	}

	return &CodexChannelUsageResult{
		Success:        statusCode >= 200 && statusCode < 300,
		UpstreamStatus: statusCode,
		Data:           payload,
	}, nil
}
