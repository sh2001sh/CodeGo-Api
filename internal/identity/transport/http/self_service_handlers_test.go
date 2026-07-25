package http

import (
	"github.com/sh2001sh/CodeGo-Api/constant"
	"github.com/sh2001sh/CodeGo-Api/dto"
	identitydomain "github.com/sh2001sh/CodeGo-Api/internal/identity/domain"
	identityschema "github.com/sh2001sh/CodeGo-Api/internal/identity/schema"
	identitystore "github.com/sh2001sh/CodeGo-Api/internal/identity/store"
	platformencoding "github.com/sh2001sh/CodeGo-Api/internal/platform/encodingx"
	platformsecurity "github.com/sh2001sh/CodeGo-Api/internal/platform/security"
	"testing"
)

func TestUpdateSelfLanguagePersistsUserSetting(t *testing.T) {
	setupIdentityHTTPTestDB(t)

	user := &identityschema.User{
		Id:          1,
		Username:    "self-language-user",
		Password:    "password123",
		DisplayName: "Self Language User",
		Role:        constant.RoleCommonUser,
		Status:      constant.UserStatusEnabled,
		Group:       "default",
	}
	if err := identitystore.CreateUser(user); err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}

	ctx, recorder := newAuthenticatedContext(t, "PUT", "/api/user/self", map[string]any{
		"language": "en-US",
	}, user.Id)
	UpdateSelf(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got %#v", response)
	}

	reloaded, err := loadUserByIDForTest(user.Id, false)
	if err != nil {
		t.Fatalf("failed to reload user: %v", err)
	}
	if identitydomain.GetSetting(reloaded).Language != "en-US" {
		t.Fatalf("expected persisted language, got %#v", identitydomain.GetSetting(reloaded))
	}
}

func TestUpdateSelfSidebarModulesPersistsUserSetting(t *testing.T) {
	setupIdentityHTTPTestDB(t)

	user := &identityschema.User{
		Id:          1,
		Username:    "self-sidebar-user",
		Password:    "password123",
		DisplayName: "Self Sidebar User",
		Role:        constant.RoleCommonUser,
		Status:      constant.UserStatusEnabled,
		Group:       "default",
	}
	identitydomain.SetSetting(user, dto.UserSetting{Language: "zh-CN"})
	if err := identitystore.CreateUser(user); err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}

	sidebarModules := `{"chat":{"enabled":true,"chat":false}}`
	ctx, recorder := newAuthenticatedContext(t, "PUT", "/api/user/self", map[string]any{
		"sidebar_modules": sidebarModules,
	}, user.Id)
	UpdateSelf(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got %#v", response)
	}

	reloaded, err := loadUserByIDForTest(user.Id, false)
	if err != nil {
		t.Fatalf("failed to reload user: %v", err)
	}
	setting := identitydomain.GetSetting(reloaded)
	if setting.SidebarModules != sidebarModules || setting.Language != "zh-CN" {
		t.Fatalf("expected sidebar modules update to preserve other settings, got %#v", setting)
	}
}

func TestUpdateSelfProfileUpdatesDisplayNameAndPassword(t *testing.T) {
	setupIdentityHTTPTestDB(t)

	user := &identityschema.User{
		Id:          1,
		Username:    "self-profile-user",
		Password:    "password123",
		DisplayName: "Before Name",
		Role:        constant.RoleCommonUser,
		Status:      constant.UserStatusEnabled,
		Group:       "default",
	}
	if err := identitystore.CreateUser(user); err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}

	ctx, recorder := newAuthenticatedContext(t, "PUT", "/api/user/self", map[string]any{
		"display_name":      "After Name",
		"original_password": "password123",
		"password":          "password456",
	}, user.Id)
	UpdateSelf(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got %#v", response)
	}

	reloaded, err := loadUserByIDForTest(user.Id, true)
	if err != nil {
		t.Fatalf("failed to reload user: %v", err)
	}
	if reloaded.DisplayName != "After Name" {
		t.Fatalf("expected updated display name, got %#v", reloaded)
	}
	if !platformsecurity.ValidatePasswordAndHash("password456", reloaded.Password) {
		t.Fatalf("expected password hash to be rotated, got %q", reloaded.Password)
	}
}

func TestUpdateSelfPasswordRequiresMatchingOriginalPassword(t *testing.T) {
	setupIdentityHTTPTestDB(t)

	user := &identityschema.User{
		Id:          1,
		Username:    "self-password-user",
		Password:    "password123",
		DisplayName: "Self Password User",
		Role:        constant.RoleCommonUser,
		Status:      constant.UserStatusEnabled,
		Group:       "default",
	}
	if err := identitystore.CreateUser(user); err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}

	ctx, recorder := newAuthenticatedContext(t, "PUT", "/api/user/self", map[string]any{
		"original_password": "wrong-password",
		"password":          "password456",
	}, user.Id)
	UpdateSelf(ctx)

	response := decodeAPIResponse(t, recorder)
	if response.Success {
		t.Fatalf("expected password update failure, got %#v", response)
	}
	if response.Message != "原密码错误" {
		t.Fatalf("expected original password error, got %q", response.Message)
	}

	reloaded, err := loadUserByIDForTest(user.Id, true)
	if err != nil {
		t.Fatalf("failed to reload user: %v", err)
	}
	if !platformsecurity.ValidatePasswordAndHash("password123", reloaded.Password) {
		t.Fatalf("expected password to remain unchanged, got %q", reloaded.Password)
	}
}

func TestDeleteSelfRejectsRootUser(t *testing.T) {
	setupIdentityHTTPTestDB(t)

	user := &identityschema.User{
		Id:          1,
		Username:    "root-user",
		Password:    "password123",
		DisplayName: "Root User",
		Role:        constant.RoleRootUser,
		Status:      constant.UserStatusEnabled,
		Group:       "default",
	}
	if err := identitystore.CreateUser(user); err != nil {
		t.Fatalf("failed to seed root user: %v", err)
	}

	ctx, recorder := newAuthenticatedContext(t, "DELETE", "/api/user/self", nil, user.Id)
	DeleteSelf(ctx)

	response := decodeAPIResponse(t, recorder)
	if response.Success {
		t.Fatalf("expected root delete rejection, got %#v", response)
	}
}

func TestDeleteSelfSoftDeletesCommonUser(t *testing.T) {
	setupIdentityHTTPTestDB(t)

	user := &identityschema.User{
		Id:          1,
		Username:    "delete-self-user",
		Password:    "password123",
		DisplayName: "Delete Self User",
		Role:        constant.RoleCommonUser,
		Status:      constant.UserStatusEnabled,
		Group:       "default",
	}
	if err := identitystore.CreateUser(user); err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}

	ctx, recorder := newAuthenticatedContext(t, "DELETE", "/api/user/self", nil, user.Id)
	DeleteSelf(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected delete success, got %#v", response)
	}

	if _, err := loadUserByIDForTest(user.Id, false); err == nil {
		t.Fatalf("expected user to be deleted")
	}
}

func TestGenerateAccessTokenPersistsRotatedToken(t *testing.T) {
	setupIdentityHTTPTestDB(t)

	user := &identityschema.User{
		Id:          1,
		Username:    "self-token-user",
		Password:    "password123",
		DisplayName: "Self Token User",
		Role:        constant.RoleCommonUser,
		Status:      constant.UserStatusEnabled,
		Group:       "default",
	}
	if err := identitystore.CreateUser(user); err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}

	ctx, recorder := newAuthenticatedContext(t, "GET", "/api/user/token", nil, user.Id)
	GenerateAccessToken(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected token rotation success, got %#v", response)
	}

	var token string
	if err := platformencoding.Unmarshal(response.Data, &token); err != nil {
		t.Fatalf("failed to decode rotated token: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty rotated token")
	}

	reloaded, err := loadUserByIDForTest(user.Id, true)
	if err != nil {
		t.Fatalf("failed to reload user: %v", err)
	}
	if reloaded.GetAccessToken() != token {
		t.Fatalf("expected persisted access token %q, got %q", token, reloaded.GetAccessToken())
	}
}
