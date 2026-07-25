package app

import (
	"context"
	"errors"
	"fmt"
	identitydomain "github.com/sh2001sh/CodeGo-Api/internal/identity/domain"
	"github.com/sh2001sh/CodeGo-Api/internal/identity/oauth"
	platformencoding "github.com/sh2001sh/CodeGo-Api/internal/platform/encodingx"
	platformobservability "github.com/sh2001sh/CodeGo-Api/internal/platform/observability"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

var (
	ErrCustomOAuthProviderIDInvalid   = errors.New("无效的 ID")
	ErrCustomOAuthProviderNotFound    = errors.New("未找到该 OAuth 提供商")
	ErrCustomOAuthProviderSlugTaken   = errors.New("该 Slug 已被使用")
	ErrCustomOAuthProviderSlugBuiltin = errors.New("该 Slug 与内置 OAuth 提供商冲突")
	ErrCustomOAuthProviderHasBindings = errors.New("该 OAuth 提供商还有用户绑定，无法删除。请先解除所有用户绑定。")
	ErrCustomOAuthBindingsCheckFailed = errors.New("检查用户绑定时发生错误，请稍后重试")
	ErrCustomOAuthDiscoveryURLMissing = errors.New("请先填写 Discovery URL 或 Issuer URL")
	ErrCustomOAuthDiscoveryURLInvalid = errors.New("Discovery URL 无效，仅支持 http/https")
)

// CustomOAuthProviderInput carries create/update fields for a custom OAuth provider.
type CustomOAuthProviderInput struct {
	Name                  string
	Slug                  string
	Icon                  *string
	Enabled               *bool
	ClientID              string
	ClientSecret          string
	AuthorizationEndpoint string
	TokenEndpoint         string
	UserInfoEndpoint      string
	Scopes                string
	UserIDField           string
	UsernameField         string
	DisplayNameField      string
	EmailField            string
	WellKnown             *string
	AuthStyle             *int
	AccessPolicy          *string
	AccessDeniedMessage   *string
}

// CustomOAuthDiscoveryResponse carries the fetched OIDC discovery document.
type CustomOAuthDiscoveryResponse struct {
	WellKnownURL string         `json:"well_known_url"`
	Discovery    map[string]any `json:"discovery"`
}

// ListCustomOAuthProviders returns every configured custom OAuth provider.
func ListCustomOAuthProviders() ([]*identitydomain.CustomOAuthProvider, error) {
	return listCustomOAuthProviderRecords()
}

// GetCustomOAuthProvider loads a provider by numeric ID.
func GetCustomOAuthProvider(id int) (*identitydomain.CustomOAuthProvider, error) {
	if id <= 0 {
		return nil, ErrCustomOAuthProviderIDInvalid
	}
	provider, err := getCustomOAuthProviderRecordByID(id)
	if err != nil {
		return nil, ErrCustomOAuthProviderNotFound
	}
	return provider, nil
}

// FetchCustomOAuthDiscovery resolves and fetches an OIDC discovery document.
func FetchCustomOAuthDiscovery(ctx context.Context, wellKnownURL string, issuerURL string) (*CustomOAuthDiscoveryResponse, error) {
	targetURL := strings.TrimSpace(wellKnownURL)
	issuerURL = strings.TrimSpace(issuerURL)
	if targetURL == "" && issuerURL == "" {
		return nil, ErrCustomOAuthDiscoveryURLMissing
	}
	if targetURL == "" {
		targetURL = strings.TrimRight(issuerURL, "/") + "/.well-known/openid-configuration"
	}
	targetURL = strings.TrimSpace(targetURL)

	parsedURL, err := url.Parse(targetURL)
	if err != nil || parsedURL.Host == "" || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
		return nil, ErrCustomOAuthDiscoveryURLInvalid
	}

	discoveryCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(discoveryCtx, http.MethodGet, targetURL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建 Discovery 请求失败: %w", err)
	}
	httpReq.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("获取 Discovery 配置失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		message := strings.TrimSpace(string(body))
		if message == "" {
			message = resp.Status
		}
		return nil, fmt.Errorf("获取 Discovery 配置失败: %s", message)
	}

	var discovery map[string]any
	if err := platformencoding.DecodeJSON(resp.Body, &discovery); err != nil {
		return nil, fmt.Errorf("解析 Discovery 配置失败: %w", err)
	}

	return &CustomOAuthDiscoveryResponse{
		WellKnownURL: targetURL,
		Discovery:    discovery,
	}, nil
}

// CreateCustomOAuthProvider persists and registers a new custom OAuth provider.
func CreateCustomOAuthProvider(input CustomOAuthProviderInput) (*identitydomain.CustomOAuthProvider, error) {
	slug := normalizeProviderSlug(input.Slug)
	if isCustomOAuthProviderSlugTaken(slug, 0) {
		return nil, ErrCustomOAuthProviderSlugTaken
	}
	if oauth.IsProviderRegistered(slug) && !oauth.IsCustomProvider(slug) {
		return nil, ErrCustomOAuthProviderSlugBuiltin
	}

	provider := &identitydomain.CustomOAuthProvider{
		Name:                  input.Name,
		Slug:                  slug,
		ClientId:              input.ClientID,
		ClientSecret:          input.ClientSecret,
		AuthorizationEndpoint: input.AuthorizationEndpoint,
		TokenEndpoint:         input.TokenEndpoint,
		UserInfoEndpoint:      input.UserInfoEndpoint,
		Scopes:                input.Scopes,
		UserIdField:           input.UserIDField,
		UsernameField:         input.UsernameField,
		DisplayNameField:      input.DisplayNameField,
		EmailField:            input.EmailField,
	}
	applyOptionalProviderFields(provider, input)

	if err := createCustomOAuthProviderRecord(provider); err != nil {
		return nil, err
	}
	oauth.RegisterOrUpdateCustomProvider(provider)
	return provider, nil
}

// UpdateCustomOAuthProvider mutates an existing provider and refreshes registry entries.
func UpdateCustomOAuthProvider(id int, input CustomOAuthProviderInput) (*identitydomain.CustomOAuthProvider, error) {
	if id <= 0 {
		return nil, ErrCustomOAuthProviderIDInvalid
	}

	provider, err := getCustomOAuthProviderRecordByID(id)
	if err != nil {
		return nil, ErrCustomOAuthProviderNotFound
	}
	oldSlug := provider.Slug
	newSlug := normalizeProviderSlug(input.Slug)
	if newSlug != "" && newSlug != provider.Slug {
		if isCustomOAuthProviderSlugTaken(newSlug, id) {
			return nil, ErrCustomOAuthProviderSlugTaken
		}
		if oauth.IsProviderRegistered(newSlug) && !oauth.IsCustomProvider(newSlug) {
			return nil, ErrCustomOAuthProviderSlugBuiltin
		}
		provider.Slug = newSlug
	}

	if input.Name != "" {
		provider.Name = input.Name
	}
	if input.ClientID != "" {
		provider.ClientId = input.ClientID
	}
	if input.ClientSecret != "" {
		provider.ClientSecret = input.ClientSecret
	}
	if input.AuthorizationEndpoint != "" {
		provider.AuthorizationEndpoint = input.AuthorizationEndpoint
	}
	if input.TokenEndpoint != "" {
		provider.TokenEndpoint = input.TokenEndpoint
	}
	if input.UserInfoEndpoint != "" {
		provider.UserInfoEndpoint = input.UserInfoEndpoint
	}
	if input.Scopes != "" {
		provider.Scopes = input.Scopes
	}
	if input.UserIDField != "" {
		provider.UserIdField = input.UserIDField
	}
	if input.UsernameField != "" {
		provider.UsernameField = input.UsernameField
	}
	if input.DisplayNameField != "" {
		provider.DisplayNameField = input.DisplayNameField
	}
	if input.EmailField != "" {
		provider.EmailField = input.EmailField
	}
	applyOptionalProviderFields(provider, input)

	if err := updateCustomOAuthProviderRecord(provider); err != nil {
		return nil, err
	}

	if oldSlug != provider.Slug {
		oauth.UnregisterCustomProvider(oldSlug)
	}
	oauth.RegisterOrUpdateCustomProvider(provider)
	return provider, nil
}

// DeleteCustomOAuthProvider removes an unbound provider and unregisters its slug.
func DeleteCustomOAuthProvider(id int) error {
	if id <= 0 {
		return ErrCustomOAuthProviderIDInvalid
	}

	provider, err := getCustomOAuthProviderRecordByID(id)
	if err != nil {
		return ErrCustomOAuthProviderNotFound
	}

	count, err := countUserOAuthBindingsByProviderID(id)
	if err != nil {
		platformobservability.SysError("Failed to get binding count for provider " + strconv.Itoa(id) + ": " + err.Error())
		return ErrCustomOAuthBindingsCheckFailed
	}
	if count > 0 {
		return ErrCustomOAuthProviderHasBindings
	}

	if err := deleteCustomOAuthProviderRecord(id); err != nil {
		return err
	}
	oauth.UnregisterCustomProvider(provider.Slug)
	return nil
}

func applyOptionalProviderFields(provider *identitydomain.CustomOAuthProvider, input CustomOAuthProviderInput) {
	if input.Icon != nil {
		provider.Icon = *input.Icon
	}
	if input.Enabled != nil {
		provider.Enabled = *input.Enabled
	}
	if input.WellKnown != nil {
		provider.WellKnown = *input.WellKnown
	}
	if input.AuthStyle != nil {
		provider.AuthStyle = *input.AuthStyle
	}
	if input.AccessPolicy != nil {
		provider.AccessPolicy = *input.AccessPolicy
	}
	if input.AccessDeniedMessage != nil {
		provider.AccessDeniedMessage = *input.AccessDeniedMessage
	}
}

func normalizeProviderSlug(slug string) string {
	return strings.ToLower(strings.TrimSpace(slug))
}
