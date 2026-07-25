package http

import (
	"github.com/gin-gonic/gin"
	"github.com/sh2001sh/CodeGo-Api/constant"
	identitydomain "github.com/sh2001sh/CodeGo-Api/internal/identity/domain"
	identityschema "github.com/sh2001sh/CodeGo-Api/internal/identity/schema"
	platformencoding "github.com/sh2001sh/CodeGo-Api/internal/platform/encodingx"
	stdhttp "net/http"
	"strconv"
	"testing"
)

func TestGetUserOAuthBindingsReturnsProviderMetadata(t *testing.T) {
	db := setupIdentityHTTPTestDB(t)

	user := &identityschema.User{Id: 1, Username: "oauth-self", Password: "password123", DisplayName: "OAuth Self", Role: constant.RoleCommonUser, Status: constant.UserStatusEnabled, Group: "default"}
	provider := &identitydomain.CustomOAuthProvider{
		Name:                  "Custom Provider",
		Slug:                  "custom-provider",
		Icon:                  "custom-icon",
		ClientId:              "client-id",
		ClientSecret:          "secret-id",
		AuthorizationEndpoint: "https://provider.example.com/auth",
		TokenEndpoint:         "https://provider.example.com/token",
		UserInfoEndpoint:      "https://provider.example.com/userinfo",
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}
	if err := db.Create(provider).Error; err != nil {
		t.Fatalf("failed to seed provider: %v", err)
	}
	if err := db.Create(&identitydomain.UserOAuthBinding{UserId: user.Id, ProviderId: provider.Id, ProviderUserId: "provider-user-1"}).Error; err != nil {
		t.Fatalf("failed to seed binding: %v", err)
	}

	ctx, recorder := newAuthenticatedContext(t, stdhttp.MethodGet, "/api/user/oauth/bindings", nil, user.Id)
	GetUserOAuthBindings(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected user bindings to succeed, got %#v", response)
	}

	var payload []struct {
		ProviderID     int    `json:"provider_id"`
		ProviderName   string `json:"provider_name"`
		ProviderSlug   string `json:"provider_slug"`
		ProviderIcon   string `json:"provider_icon"`
		ProviderUserID string `json:"provider_user_id"`
	}
	if err := platformencoding.Unmarshal(response.Data, &payload); err != nil {
		t.Fatalf("failed to decode bindings response: %v", err)
	}
	if len(payload) != 1 {
		t.Fatalf("expected 1 binding, got %#v", payload)
	}
	if payload[0].ProviderID != provider.Id || payload[0].ProviderSlug != provider.Slug || payload[0].ProviderUserID != "provider-user-1" {
		t.Fatalf("unexpected binding payload: %#v", payload[0])
	}
}

func TestGetUserOAuthBindingsByAdminRequiresHigherRole(t *testing.T) {
	db := setupIdentityHTTPTestDB(t)

	admin := &identityschema.User{Id: 1, Username: "oauth-admin", Password: "password123", DisplayName: "OAuth Admin", Role: constant.RoleAdminUser, Status: constant.UserStatusEnabled, Group: "default"}
	target := &identityschema.User{Id: 2, Username: "oauth-target-admin", Password: "password123", DisplayName: "OAuth Target Admin", Role: constant.RoleAdminUser, Status: constant.UserStatusEnabled, Group: "default"}
	for _, user := range []*identityschema.User{admin, target} {
		if err := db.Create(user).Error; err != nil {
			t.Fatalf("failed to seed user: %v", err)
		}
	}

	ctx, recorder := newAuthenticatedContext(t, stdhttp.MethodGet, "/api/user/2/oauth/bindings", nil, admin.Id)
	ctx.Set("role", constant.RoleAdminUser)
	ctx.Params = gin.Params{{Key: "id", Value: "2"}}
	GetUserOAuthBindingsByAdmin(ctx)

	response := decodeAPIResponse(t, recorder)
	if response.Success {
		t.Fatalf("expected same-role admin binding lookup to fail")
	}
	if response.Message != "no permission" {
		t.Fatalf("expected no permission message, got %q", response.Message)
	}
}

func TestUnbindCustomOAuthDeletesBinding(t *testing.T) {
	db := setupIdentityHTTPTestDB(t)

	user := &identityschema.User{Id: 1, Username: "oauth-unbind", Password: "password123", DisplayName: "OAuth Unbind", Role: constant.RoleCommonUser, Status: constant.UserStatusEnabled, Group: "default"}
	provider := &identitydomain.CustomOAuthProvider{
		Name:                  "Unbind Provider",
		Slug:                  "unbind-provider",
		ClientId:              "client-id",
		ClientSecret:          "secret-id",
		AuthorizationEndpoint: "https://provider.example.com/auth",
		TokenEndpoint:         "https://provider.example.com/token",
		UserInfoEndpoint:      "https://provider.example.com/userinfo",
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}
	if err := db.Create(provider).Error; err != nil {
		t.Fatalf("failed to seed provider: %v", err)
	}
	if err := db.Create(&identitydomain.UserOAuthBinding{UserId: user.Id, ProviderId: provider.Id, ProviderUserId: "provider-user-2"}).Error; err != nil {
		t.Fatalf("failed to seed binding: %v", err)
	}

	ctx, recorder := newAuthenticatedContext(t, stdhttp.MethodDelete, "/api/user/oauth/bindings/"+strconv.Itoa(provider.Id), nil, user.Id)
	ctx.Params = gin.Params{{Key: "provider_id", Value: strconv.Itoa(provider.Id)}}
	UnbindCustomOAuth(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected unbind to succeed, got %#v", response)
	}
	var deletedBinding identitydomain.UserOAuthBinding
	if err := db.Where("user_id = ? AND provider_id = ?", user.Id, provider.Id).First(&deletedBinding).Error; err == nil {
		t.Fatalf("expected binding to be deleted")
	}
}

func TestUnbindCustomOAuthByAdminDeletesBinding(t *testing.T) {
	db := setupIdentityHTTPTestDB(t)

	admin := &identityschema.User{Id: 1, Username: "oauth-admin-unbind", Password: "password123", DisplayName: "OAuth Admin Unbind", Role: constant.RoleAdminUser, Status: constant.UserStatusEnabled, Group: "default"}
	target := &identityschema.User{Id: 2, Username: "oauth-target-user", Password: "password123", DisplayName: "OAuth Target User", Role: constant.RoleCommonUser, Status: constant.UserStatusEnabled, Group: "default"}
	provider := &identitydomain.CustomOAuthProvider{
		Name:                  "Admin Unbind Provider",
		Slug:                  "admin-unbind-provider",
		ClientId:              "client-id",
		ClientSecret:          "secret-id",
		AuthorizationEndpoint: "https://provider.example.com/auth",
		TokenEndpoint:         "https://provider.example.com/token",
		UserInfoEndpoint:      "https://provider.example.com/userinfo",
	}
	for _, user := range []*identityschema.User{admin, target} {
		if err := db.Create(user).Error; err != nil {
			t.Fatalf("failed to seed user: %v", err)
		}
	}
	if err := db.Create(provider).Error; err != nil {
		t.Fatalf("failed to seed provider: %v", err)
	}
	if err := db.Create(&identitydomain.UserOAuthBinding{UserId: target.Id, ProviderId: provider.Id, ProviderUserId: "provider-user-3"}).Error; err != nil {
		t.Fatalf("failed to seed binding: %v", err)
	}

	ctx, recorder := newAuthenticatedContext(t, stdhttp.MethodDelete, "/api/user/2/oauth/bindings/"+strconv.Itoa(provider.Id), nil, admin.Id)
	ctx.Set("role", constant.RoleAdminUser)
	ctx.Params = gin.Params{
		{Key: "id", Value: strconv.Itoa(target.Id)},
		{Key: "provider_id", Value: strconv.Itoa(provider.Id)},
	}
	UnbindCustomOAuthByAdmin(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected admin unbind to succeed, got %#v", response)
	}
	var deletedBinding identitydomain.UserOAuthBinding
	if err := db.Where("user_id = ? AND provider_id = ?", target.Id, provider.Id).First(&deletedBinding).Error; err == nil {
		t.Fatalf("expected binding to be deleted")
	}
}
