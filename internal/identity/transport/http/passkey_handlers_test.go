package http

import (
	"encoding/base64"
	"github.com/gin-gonic/gin"
	"github.com/sh2001sh/CodeGo-Api/constant"
	identitydomain "github.com/sh2001sh/CodeGo-Api/internal/identity/domain"
	identityschema "github.com/sh2001sh/CodeGo-Api/internal/identity/schema"
	platformencoding "github.com/sh2001sh/CodeGo-Api/internal/platform/encodingx"
	"net/http"
	"testing"
	"time"
)

func TestPasskeyStatusReturnsDisabledWhenNoCredential(t *testing.T) {
	db := setupIdentityHTTPTestDB(t)

	user := &identityschema.User{Id: 1, Username: "passkey-status-user", Password: "password123", DisplayName: "Passkey Status User", Role: constant.RoleCommonUser, Status: constant.UserStatusEnabled, Group: "default"}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/user/passkey", nil, user.Id)
	PasskeyStatus(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got %#v", response)
	}

	var payload struct {
		Enabled bool `json:"enabled"`
	}
	if err := platformencoding.Unmarshal(response.Data, &payload); err != nil {
		t.Fatalf("failed to decode passkey status: %v", err)
	}
	if payload.Enabled {
		t.Fatalf("expected disabled passkey status, got %#v", payload)
	}
}

func TestPasskeyStatusReturnsLastUsedAtWhenBound(t *testing.T) {
	db := setupIdentityHTTPTestDB(t)

	user := &identityschema.User{Id: 1, Username: "passkey-bound-user", Password: "password123", DisplayName: "Passkey Bound User", Role: constant.RoleCommonUser, Status: constant.UserStatusEnabled, Group: "default"}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}
	lastUsedAt := time.Now().UTC().Truncate(time.Second)
	credential := &identitydomain.PasskeyCredential{
		UserID:       user.Id,
		CredentialID: base64.StdEncoding.EncodeToString([]byte("credential-id")),
		PublicKey:    base64.StdEncoding.EncodeToString([]byte("public-key")),
		LastUsedAt:   &lastUsedAt,
	}
	if err := db.Create(credential).Error; err != nil {
		t.Fatalf("failed to seed passkey credential: %v", err)
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/user/passkey", nil, user.Id)
	PasskeyStatus(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got %#v", response)
	}

	var payload struct {
		Enabled    bool       `json:"enabled"`
		LastUsedAt *time.Time `json:"last_used_at"`
	}
	if err := platformencoding.Unmarshal(response.Data, &payload); err != nil {
		t.Fatalf("failed to decode passkey status: %v", err)
	}
	if !payload.Enabled || payload.LastUsedAt == nil {
		t.Fatalf("expected enabled passkey status with last_used_at, got %#v", payload)
	}
}

func TestAdminResetPasskeyDeletesCredential(t *testing.T) {
	db := setupIdentityHTTPTestDB(t)

	admin := &identityschema.User{Id: 1, Username: "passkey-admin", Password: "password123", DisplayName: "Passkey Admin", Role: constant.RoleAdminUser, Status: constant.UserStatusEnabled, Group: "default"}
	target := &identityschema.User{Id: 2, Username: "passkey-target", Password: "password123", DisplayName: "Passkey Target", Role: constant.RoleCommonUser, Status: constant.UserStatusEnabled, Group: "default"}
	for _, user := range []*identityschema.User{admin, target} {
		if err := db.Create(user).Error; err != nil {
			t.Fatalf("failed to seed user: %v", err)
		}
	}
	credential := &identitydomain.PasskeyCredential{
		UserID:       target.Id,
		CredentialID: base64.StdEncoding.EncodeToString([]byte("target-credential")),
		PublicKey:    base64.StdEncoding.EncodeToString([]byte("public-key")),
	}
	if err := db.Create(credential).Error; err != nil {
		t.Fatalf("failed to seed passkey credential: %v", err)
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodDelete, "/api/user/2/reset_passkey", nil, admin.Id)
	ctx.Set("role", constant.RoleAdminUser)
	ctx.Params = gin.Params{{Key: "id", Value: "2"}}
	AdminResetPasskey(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got %#v", response)
	}

	var reloaded identitydomain.PasskeyCredential
	if err := db.Where("user_id = ?", target.Id).First(&reloaded).Error; err == nil {
		t.Fatalf("expected passkey credential to be deleted, got credential=%#v", reloaded)
	}
}
