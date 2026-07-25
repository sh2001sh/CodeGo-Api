package http

import (
	httpapi "github.com/sh2001sh/CodeGo-Api/internal/platform/transport/http/httpapi"
	stdhttp "net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	adminopsapp "github.com/sh2001sh/CodeGo-Api/internal/adminops/app"
	identitydomain "github.com/sh2001sh/CodeGo-Api/internal/identity/domain"
)

type customOAuthProviderResponse struct {
	ID                    int    `json:"id"`
	Name                  string `json:"name"`
	Slug                  string `json:"slug"`
	Icon                  string `json:"icon"`
	Enabled               bool   `json:"enabled"`
	ClientID              string `json:"client_id"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	UserInfoEndpoint      string `json:"user_info_endpoint"`
	Scopes                string `json:"scopes"`
	UserIDField           string `json:"user_id_field"`
	UsernameField         string `json:"username_field"`
	DisplayNameField      string `json:"display_name_field"`
	EmailField            string `json:"email_field"`
	WellKnown             string `json:"well_known"`
	AuthStyle             int    `json:"auth_style"`
	AccessPolicy          string `json:"access_policy"`
	AccessDeniedMessage   string `json:"access_denied_message"`
}

type customOAuthCreateRequest struct {
	Name                  string `json:"name" binding:"required"`
	Slug                  string `json:"slug" binding:"required"`
	Icon                  string `json:"icon"`
	Enabled               bool   `json:"enabled"`
	ClientID              string `json:"client_id" binding:"required"`
	ClientSecret          string `json:"client_secret" binding:"required"`
	AuthorizationEndpoint string `json:"authorization_endpoint" binding:"required"`
	TokenEndpoint         string `json:"token_endpoint" binding:"required"`
	UserInfoEndpoint      string `json:"user_info_endpoint" binding:"required"`
	Scopes                string `json:"scopes"`
	UserIDField           string `json:"user_id_field"`
	UsernameField         string `json:"username_field"`
	DisplayNameField      string `json:"display_name_field"`
	EmailField            string `json:"email_field"`
	WellKnown             string `json:"well_known"`
	AuthStyle             int    `json:"auth_style"`
	AccessPolicy          string `json:"access_policy"`
	AccessDeniedMessage   string `json:"access_denied_message"`
}

type customOAuthUpdateRequest struct {
	Name                  string  `json:"name"`
	Slug                  string  `json:"slug"`
	Icon                  *string `json:"icon"`
	Enabled               *bool   `json:"enabled"`
	ClientID              string  `json:"client_id"`
	ClientSecret          string  `json:"client_secret"`
	AuthorizationEndpoint string  `json:"authorization_endpoint"`
	TokenEndpoint         string  `json:"token_endpoint"`
	UserInfoEndpoint      string  `json:"user_info_endpoint"`
	Scopes                string  `json:"scopes"`
	UserIDField           string  `json:"user_id_field"`
	UsernameField         string  `json:"username_field"`
	DisplayNameField      string  `json:"display_name_field"`
	EmailField            string  `json:"email_field"`
	WellKnown             *string `json:"well_known"`
	AuthStyle             *int    `json:"auth_style"`
	AccessPolicy          *string `json:"access_policy"`
	AccessDeniedMessage   *string `json:"access_denied_message"`
}

type customOAuthDiscoveryRequest struct {
	WellKnownURL string `json:"well_known_url"`
	IssuerURL    string `json:"issuer_url"`
}

// FetchCustomOAuthDiscovery loads an OIDC discovery document for a custom provider.
func FetchCustomOAuthDiscovery(c *gin.Context) {
	var req customOAuthDiscoveryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpapi.ApiErrorMsg(c, "无效的请求参数: "+err.Error())
		return
	}

	response, err := adminopsapp.FetchCustomOAuthDiscovery(c.Request.Context(), req.WellKnownURL, req.IssuerURL)
	if err != nil {
		handleCustomOAuthError(c, err)
		return
	}
	httpapi.ApiSuccess(c, response)
}

// GetCustomOAuthProviders returns every configured custom OAuth provider.
func GetCustomOAuthProviders(c *gin.Context) {
	providers, err := adminopsapp.ListCustomOAuthProviders()
	if err != nil {
		httpapi.ApiError(c, err)
		return
	}

	response := make([]customOAuthProviderResponse, 0, len(providers))
	for _, provider := range providers {
		response = append(response, toCustomOAuthProviderResponse(provider))
	}
	httpapi.ApiSuccess(c, response)
}

// GetCustomOAuthProvider returns a single provider by ID.
func GetCustomOAuthProvider(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		httpapi.ApiErrorMsg(c, adminopsapp.ErrCustomOAuthProviderIDInvalid.Error())
		return
	}

	provider, err := adminopsapp.GetCustomOAuthProvider(id)
	if err != nil {
		handleCustomOAuthError(c, err)
		return
	}
	httpapi.ApiSuccess(c, toCustomOAuthProviderResponse(provider))
}

// CreateCustomOAuthProvider creates and registers a new custom provider.
func CreateCustomOAuthProvider(c *gin.Context) {
	var req customOAuthCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpapi.ApiErrorMsg(c, "无效的请求参数: "+err.Error())
		return
	}

	enabled := req.Enabled
	icon := req.Icon
	wellKnown := req.WellKnown
	authStyle := req.AuthStyle
	accessPolicy := req.AccessPolicy
	accessDeniedMessage := req.AccessDeniedMessage

	provider, err := adminopsapp.CreateCustomOAuthProvider(adminopsapp.CustomOAuthProviderInput{
		Name:                  req.Name,
		Slug:                  req.Slug,
		Icon:                  &icon,
		Enabled:               &enabled,
		ClientID:              req.ClientID,
		ClientSecret:          req.ClientSecret,
		AuthorizationEndpoint: req.AuthorizationEndpoint,
		TokenEndpoint:         req.TokenEndpoint,
		UserInfoEndpoint:      req.UserInfoEndpoint,
		Scopes:                req.Scopes,
		UserIDField:           req.UserIDField,
		UsernameField:         req.UsernameField,
		DisplayNameField:      req.DisplayNameField,
		EmailField:            req.EmailField,
		WellKnown:             &wellKnown,
		AuthStyle:             &authStyle,
		AccessPolicy:          &accessPolicy,
		AccessDeniedMessage:   &accessDeniedMessage,
	})
	if err != nil {
		handleCustomOAuthError(c, err)
		return
	}

	c.JSON(stdhttp.StatusOK, gin.H{
		"success": true,
		"message": "创建成功",
		"data":    toCustomOAuthProviderResponse(provider),
	})
}

// UpdateCustomOAuthProvider updates an existing custom provider.
func UpdateCustomOAuthProvider(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		httpapi.ApiErrorMsg(c, adminopsapp.ErrCustomOAuthProviderIDInvalid.Error())
		return
	}

	var req customOAuthUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpapi.ApiErrorMsg(c, "无效的请求参数: "+err.Error())
		return
	}

	provider, err := adminopsapp.UpdateCustomOAuthProvider(id, adminopsapp.CustomOAuthProviderInput{
		Name:                  req.Name,
		Slug:                  req.Slug,
		Icon:                  req.Icon,
		Enabled:               req.Enabled,
		ClientID:              req.ClientID,
		ClientSecret:          req.ClientSecret,
		AuthorizationEndpoint: req.AuthorizationEndpoint,
		TokenEndpoint:         req.TokenEndpoint,
		UserInfoEndpoint:      req.UserInfoEndpoint,
		Scopes:                req.Scopes,
		UserIDField:           req.UserIDField,
		UsernameField:         req.UsernameField,
		DisplayNameField:      req.DisplayNameField,
		EmailField:            req.EmailField,
		WellKnown:             req.WellKnown,
		AuthStyle:             req.AuthStyle,
		AccessPolicy:          req.AccessPolicy,
		AccessDeniedMessage:   req.AccessDeniedMessage,
	})
	if err != nil {
		handleCustomOAuthError(c, err)
		return
	}

	c.JSON(stdhttp.StatusOK, gin.H{
		"success": true,
		"message": "更新成功",
		"data":    toCustomOAuthProviderResponse(provider),
	})
}

// DeleteCustomOAuthProvider deletes an unbound custom provider.
func DeleteCustomOAuthProvider(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		httpapi.ApiErrorMsg(c, adminopsapp.ErrCustomOAuthProviderIDInvalid.Error())
		return
	}

	if err := adminopsapp.DeleteCustomOAuthProvider(id); err != nil {
		handleCustomOAuthError(c, err)
		return
	}

	c.JSON(stdhttp.StatusOK, gin.H{
		"success": true,
		"message": "删除成功",
	})
}

func toCustomOAuthProviderResponse(provider *identitydomain.CustomOAuthProvider) customOAuthProviderResponse {
	return customOAuthProviderResponse{
		ID:                    provider.Id,
		Name:                  provider.Name,
		Slug:                  provider.Slug,
		Icon:                  provider.Icon,
		Enabled:               provider.Enabled,
		ClientID:              provider.ClientId,
		AuthorizationEndpoint: provider.AuthorizationEndpoint,
		TokenEndpoint:         provider.TokenEndpoint,
		UserInfoEndpoint:      provider.UserInfoEndpoint,
		Scopes:                provider.Scopes,
		UserIDField:           provider.UserIdField,
		UsernameField:         provider.UsernameField,
		DisplayNameField:      provider.DisplayNameField,
		EmailField:            provider.EmailField,
		WellKnown:             provider.WellKnown,
		AuthStyle:             provider.AuthStyle,
		AccessPolicy:          provider.AccessPolicy,
		AccessDeniedMessage:   provider.AccessDeniedMessage,
	}
}

func handleCustomOAuthError(c *gin.Context, err error) {
	switch err {
	case nil:
		return
	case adminopsapp.ErrCustomOAuthProviderIDInvalid,
		adminopsapp.ErrCustomOAuthProviderNotFound,
		adminopsapp.ErrCustomOAuthProviderSlugTaken,
		adminopsapp.ErrCustomOAuthProviderSlugBuiltin,
		adminopsapp.ErrCustomOAuthProviderHasBindings,
		adminopsapp.ErrCustomOAuthBindingsCheckFailed,
		adminopsapp.ErrCustomOAuthDiscoveryURLMissing,
		adminopsapp.ErrCustomOAuthDiscoveryURLInvalid:
		httpapi.ApiErrorMsg(c, err.Error())
	default:
		httpapi.ApiErrorMsg(c, err.Error())
	}
}
