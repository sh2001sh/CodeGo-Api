package http

import (
	"github.com/sh2001sh/CodeGo-Api/constant"
	"github.com/sh2001sh/CodeGo-Api/dto"
	gatewaygroups "github.com/sh2001sh/CodeGo-Api/internal/gateway/groupsettings"
	gatewayschema "github.com/sh2001sh/CodeGo-Api/internal/gateway/schema"
	identitydomain "github.com/sh2001sh/CodeGo-Api/internal/identity/domain"
	identityschema "github.com/sh2001sh/CodeGo-Api/internal/identity/schema"
	platformencoding "github.com/sh2001sh/CodeGo-Api/internal/platform/encodingx"
	"testing"
)

func TestGetUserSelfReturnsProfilePermissionsAndSidebarModules(t *testing.T) {
	db := setupIdentityHTTPTestDB(t)

	user := &identityschema.User{
		Id:             1,
		ExternalId:     "7KM4QZ",
		Username:       "profile-user",
		Password:       "password123",
		DisplayName:    "Profile User",
		Role:           constant.RoleAdminUser,
		Status:         constant.UserStatusEnabled,
		Email:          "profile@example.com",
		Group:          "default",
		Quota:          42,
		UsedQuota:      5,
		RequestCount:   11,
		Setting:        "",
		StripeCustomer: "cus_profile",
	}
	identitydomain.SetSetting(user, dto.UserSetting{SidebarModules: `{"chat":{"enabled":true}}`})
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}

	ctx, recorder := newAuthenticatedContext(t, "GET", "/api/user/self", nil, user.Id)
	ctx.Set("role", constant.RoleAdminUser)
	GetUserSelf(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}

	var payload struct {
		Id             int            `json:"id"`
		ExternalId     string         `json:"external_id"`
		Username       string         `json:"username"`
		DisplayName    string         `json:"display_name"`
		SidebarModules string         `json:"sidebar_modules"`
		Permissions    map[string]any `json:"permissions"`
		StripeCustomer string         `json:"stripe_customer"`
	}
	if err := platformencoding.Unmarshal(response.Data, &payload); err != nil {
		t.Fatalf("failed to decode self profile: %v", err)
	}

	if payload.Id != user.Id || payload.Username != user.Username || payload.DisplayName != user.DisplayName {
		t.Fatalf("unexpected self profile payload: %#v", payload)
	}
	if payload.ExternalId != user.ExternalId {
		t.Fatalf("expected public user ID %q, got %q", user.ExternalId, payload.ExternalId)
	}
	if payload.SidebarModules != `{"chat":{"enabled":true}}` {
		t.Fatalf("expected sidebar modules to be extracted, got %q", payload.SidebarModules)
	}
	if payload.StripeCustomer != user.StripeCustomer {
		t.Fatalf("expected persisted profile fields, got %#v", payload)
	}
	if sidebarSettings, ok := payload.Permissions["sidebar_settings"].(bool); !ok || !sidebarSettings {
		t.Fatalf("expected admin sidebar_settings permission, got %#v", payload.Permissions)
	}
	permissionModules, ok := payload.Permissions["sidebar_modules"].(map[string]any)
	if !ok {
		t.Fatalf("expected sidebar_modules permission map, got %#v", payload.Permissions["sidebar_modules"])
	}
	adminPermission, ok := permissionModules["admin"].(map[string]any)
	if !ok {
		t.Fatalf("expected admin permission map, got %#v", permissionModules["admin"])
	}
	if settingPermission, ok := adminPermission["setting"].(bool); !ok || settingPermission {
		t.Fatalf("expected admin setting permission to be false, got %#v", adminPermission["setting"])
	}
}

func TestGetUserModelsReturnsSortedDeduplicatedModels(t *testing.T) {
	db := setupIdentityHTTPTestDB(t)

	user := &identityschema.User{Id: 1, Username: "models-user", Password: "password123", DisplayName: "Models User", Role: constant.RoleCommonUser, Status: constant.UserStatusEnabled, Group: "default"}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}

	abilities := []gatewayschema.Ability{
		{Group: "default", Model: "gpt-5", ChannelId: 1, Enabled: true},
		{Group: "default", Model: "claude-3-7-sonnet", ChannelId: 2, Enabled: true},
		{Group: "default", Model: "gpt-5", ChannelId: 3, Enabled: true},
	}
	for _, ability := range abilities {
		record := ability
		if err := db.Create(&record).Error; err != nil {
			t.Fatalf("failed to seed ability: %v", err)
		}
	}

	ctx, recorder := newAuthenticatedContext(t, "GET", "/api/user/models", nil, user.Id)
	GetUserModels(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}

	var models []string
	if err := platformencoding.Unmarshal(response.Data, &models); err != nil {
		t.Fatalf("failed to decode user models: %v", err)
	}
	expected := []string{"claude-3-7-sonnet", "gpt-5"}
	if len(models) != len(expected) {
		t.Fatalf("expected %d models, got %#v", len(expected), models)
	}
	for i := range expected {
		if models[i] != expected[i] {
			t.Fatalf("expected sorted deduplicated models %v, got %v", expected, models)
		}
	}
}

func TestGetUserModelsFiltersByAutoGroupChain(t *testing.T) {
	db := setupIdentityHTTPTestDB(t)

	originalAutoGroups := gatewaygroups.AutoGroups2JsonString()
	originalUsableGroups := gatewaygroups.UserUsableGroups2JSONString()
	t.Cleanup(func() {
		_ = gatewaygroups.UpdateAutoGroupsByJsonString(originalAutoGroups)
		_ = gatewaygroups.UpdateUserUsableGroupsByJSONString(originalUsableGroups)
	})
	if err := gatewaygroups.UpdateAutoGroupsByJsonString(`["default","claude"]`); err != nil {
		t.Fatalf("failed to configure auto groups: %v", err)
	}
	if err := gatewaygroups.UpdateUserUsableGroupsByJSONString(`{"default":"默认","claude":"Claude","archive":"归档"}`); err != nil {
		t.Fatalf("failed to configure usable groups: %v", err)
	}

	user := &identityschema.User{Id: 1, Username: "auto-models-user", Password: "password123", Role: constant.RoleCommonUser, Status: constant.UserStatusEnabled, Group: "default"}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}
	for _, ability := range []gatewayschema.Ability{
		{Group: "default", Model: "gpt-5", ChannelId: 1, Enabled: true},
		{Group: "claude", Model: "claude-sonnet-4-5", ChannelId: 2, Enabled: true},
		{Group: "archive", Model: "legacy-model", ChannelId: 3, Enabled: true},
	} {
		record := ability
		if err := db.Create(&record).Error; err != nil {
			t.Fatalf("failed to seed ability: %v", err)
		}
	}

	ctx, recorder := newAuthenticatedContext(t, "GET", "/api/user/models?group=auto", nil, user.Id)
	GetUserModels(ctx)
	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}

	var models []string
	if err := platformencoding.Unmarshal(response.Data, &models); err != nil {
		t.Fatalf("failed to decode user models: %v", err)
	}
	expected := []string{"claude-sonnet-4-5", "gpt-5"}
	if len(models) != len(expected) {
		t.Fatalf("expected auto models %v, got %v", expected, models)
	}
	for index := range expected {
		if models[index] != expected[index] {
			t.Fatalf("expected auto models %v, got %v", expected, models)
		}
	}
}
