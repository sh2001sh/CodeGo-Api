package http

import (
	"github.com/sh2001sh/CodeGo-Api/constant"
	"github.com/sh2001sh/CodeGo-Api/dto"
	identitydomain "github.com/sh2001sh/CodeGo-Api/internal/identity/domain"
	identityschema "github.com/sh2001sh/CodeGo-Api/internal/identity/schema"
	"net/http"
	"testing"
)

func TestUpdateUserSettingPersistsWebhookSettings(t *testing.T) {
	db := setupIdentityHTTPTestDB(t)
	user := &identityschema.User{
		Id:          1,
		Username:    "settings-user",
		Password:    "password123",
		DisplayName: "Settings User",
		Role:        constant.RoleCommonUser,
		Status:      constant.UserStatusEnabled,
		Group:       "default",
	}
	identitydomain.SetSetting(user, dto.UserSetting{
		SidebarModules:                   `{"chat":{"enabled":true}}`,
		BillingPreference:                "wallet",
		UpstreamModelUpdateNotifyEnabled: true,
	})
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/user/setting", map[string]any{
		"notify_type":                          dto.NotifyTypeWebhook,
		"quota_warning_threshold":              12.5,
		"webhook_url":                          "https://example.com/hook",
		"webhook_secret":                       "secret-token",
		"accept_unset_model_ratio_model":       true,
		"record_ip_log":                        true,
		"upstream_model_update_notify_enabled": false,
	}, user.Id)
	ctx.Set("role", constant.RoleCommonUser)
	UpdateUserSetting(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected update success, got message: %s", response.Message)
	}

	reloaded, err := loadUserByIDForTest(user.Id, true)
	if err != nil {
		t.Fatalf("failed to reload user: %v", err)
	}
	settings := identitydomain.GetSetting(reloaded)
	if settings.NotifyType != dto.NotifyTypeWebhook || settings.WebhookUrl != "https://example.com/hook" || settings.WebhookSecret != "secret-token" {
		t.Fatalf("expected webhook settings to persist, got %#v", settings)
	}
	if !settings.AcceptUnsetRatioModel || !settings.RecordIpLog {
		t.Fatalf("expected boolean settings to persist, got %#v", settings)
	}
	if settings.SidebarModules != `{"chat":{"enabled":true}}` || settings.BillingPreference != "wallet" {
		t.Fatalf("expected unrelated settings to be preserved, got %#v", settings)
	}
	if !settings.UpstreamModelUpdateNotifyEnabled {
		t.Fatalf("expected non-admin upstream notify flag to remain unchanged, got %#v", settings)
	}
}

func TestUpdateUserSettingAppliesAdminOnlyAndGotifyDefaults(t *testing.T) {
	db := setupIdentityHTTPTestDB(t)
	user := &identityschema.User{
		Id:          1,
		Username:    "admin-settings-user",
		Password:    "password123",
		DisplayName: "Admin Settings User",
		Role:        constant.RoleAdminUser,
		Status:      constant.UserStatusEnabled,
		Group:       "default",
	}
	identitydomain.SetSetting(user, dto.UserSetting{
		UpstreamModelUpdateNotifyEnabled: false,
	})
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to seed admin user: %v", err)
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/user/setting", map[string]any{
		"notify_type":                          dto.NotifyTypeGotify,
		"quota_warning_threshold":              3,
		"gotify_url":                           "https://gotify.example.com",
		"gotify_token":                         "gotify-token",
		"gotify_priority":                      99,
		"upstream_model_update_notify_enabled": true,
	}, user.Id)
	ctx.Set("role", constant.RoleAdminUser)
	UpdateUserSetting(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected admin update success, got message: %s", response.Message)
	}

	reloaded, err := loadUserByIDForTest(user.Id, true)
	if err != nil {
		t.Fatalf("failed to reload admin user: %v", err)
	}
	settings := identitydomain.GetSetting(reloaded)
	if !settings.UpstreamModelUpdateNotifyEnabled {
		t.Fatalf("expected admin-only flag to update, got %#v", settings)
	}
	if settings.GotifyPriority != 5 {
		t.Fatalf("expected gotify priority default 5, got %d", settings.GotifyPriority)
	}
}

func TestUpdateUserSettingRejectsInvalidTransportSpecificValues(t *testing.T) {
	setupIdentityHTTPTestDB(t)

	tests := []struct {
		name string
		body map[string]any
	}{
		{
			name: "invalid notify type",
			body: map[string]any{
				"notify_type":             "sms",
				"quota_warning_threshold": 1,
			},
		},
		{
			name: "invalid bark url",
			body: map[string]any{
				"notify_type":             dto.NotifyTypeBark,
				"quota_warning_threshold": 1,
				"bark_url":                "ftp://example.com",
			},
		},
		{
			name: "missing gotify token",
			body: map[string]any{
				"notify_type":             dto.NotifyTypeGotify,
				"quota_warning_threshold": 1,
				"gotify_url":              "https://gotify.example.com",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/user/setting", tc.body, 1)
			UpdateUserSetting(ctx)

			response := decodeAPIResponse(t, recorder)
			if response.Success {
				t.Fatalf("expected invalid request to fail")
			}
		})
	}
}
