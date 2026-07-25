package http

import (
	"bytes"
	"fmt"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/pquerna/otp/totp"
	"github.com/sh2001sh/CodeGo-Api/constant"
	identitydomain "github.com/sh2001sh/CodeGo-Api/internal/identity/domain"
	identityschema "github.com/sh2001sh/CodeGo-Api/internal/identity/schema"
	platformencoding "github.com/sh2001sh/CodeGo-Api/internal/platform/encodingx"
	platformsecurity "github.com/sh2001sh/CodeGo-Api/internal/platform/security"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSetupAndEnableTwoFAFlow(t *testing.T) {
	db := setupIdentityHTTPTestDB(t)

	user := &identityschema.User{Id: 1, Username: "twofa-user", Password: "password123", DisplayName: "TwoFA User", Role: constant.RoleCommonUser, Status: constant.UserStatusEnabled, Group: "default"}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}

	setupCtx, setupRecorder := newAuthenticatedContext(t, http.MethodPost, "/api/user/2fa/setup", nil, user.Id)
	SetupTwoFA(setupCtx)

	setupResponse := decodeAPIResponse(t, setupRecorder)
	if !setupResponse.Success {
		t.Fatalf("expected successful setup, got %#v", setupResponse)
	}

	var setupPayload struct {
		Secret      string   `json:"secret"`
		BackupCodes []string `json:"backup_codes"`
	}
	if err := platformencoding.Unmarshal(setupResponse.Data, &setupPayload); err != nil {
		t.Fatalf("failed to decode setup payload: %v", err)
	}
	if setupPayload.Secret == "" || len(setupPayload.BackupCodes) != platformsecurity.BackupCodeCount {
		t.Fatalf("unexpected setup payload: %#v", setupPayload)
	}

	code, err := totp.GenerateCode(setupPayload.Secret, time.Now())
	if err != nil {
		t.Fatalf("failed to generate totp code: %v", err)
	}

	enableCtx, enableRecorder := newAuthenticatedContext(t, http.MethodPost, "/api/user/2fa/enable", map[string]any{"code": code}, user.Id)
	EnableTwoFA(enableCtx)
	enableResponse := decodeAPIResponse(t, enableRecorder)
	if !enableResponse.Success {
		t.Fatalf("expected successful enable, got %#v", enableResponse)
	}

	statusCtx, statusRecorder := newAuthenticatedContext(t, http.MethodGet, "/api/user/2fa/status", nil, user.Id)
	GetTwoFAStatus(statusCtx)
	statusResponse := decodeAPIResponse(t, statusRecorder)
	if !statusResponse.Success {
		t.Fatalf("expected successful status response, got %#v", statusResponse)
	}

	var statusPayload struct {
		Enabled              bool `json:"enabled"`
		Locked               bool `json:"locked"`
		BackupCodesRemaining int  `json:"backup_codes_remaining"`
	}
	if err := platformencoding.Unmarshal(statusResponse.Data, &statusPayload); err != nil {
		t.Fatalf("failed to decode status payload: %v", err)
	}
	if !statusPayload.Enabled || statusPayload.Locked || statusPayload.BackupCodesRemaining != platformsecurity.BackupCodeCount {
		t.Fatalf("unexpected twofa status payload: %#v", statusPayload)
	}
}

func TestVerify2FALoginCompletesPendingSession(t *testing.T) {
	db := setupIdentityHTTPTestDB(t)

	secretKey, err := platformsecurity.GenerateTOTPSecret("login-twofa-user", "codego-api")
	if err != nil {
		t.Fatalf("failed to generate secret: %v", err)
	}

	user := &identityschema.User{Id: 1, Username: "login-twofa-user", Password: "password123", DisplayName: "Login TwoFA User", Role: constant.RoleCommonUser, Status: constant.UserStatusEnabled, Group: "default"}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}
	if err := db.Create(&identitydomain.TwoFA{UserId: user.Id, Secret: secretKey.Secret(), IsEnabled: true}).Error; err != nil {
		t.Fatalf("failed to seed twofa record: %v", err)
	}

	engine := gin.New()
	engine.Use(sessions.Sessions("session", cookie.NewStore([]byte("twofa-test-secret"))))
	engine.POST("/prepare", func(c *gin.Context) {
		session := sessions.Default(c)
		session.Set("pending_username", user.Username)
		session.Set("pending_user_id", user.Id)
		if err := session.Save(); err != nil {
			t.Fatalf("failed to seed pending session: %v", err)
		}
		c.Status(http.StatusOK)
	})
	engine.POST("/login/2fa", Verify2FALogin)
	engine.GET("/session", func(c *gin.Context) {
		session := sessions.Default(c)
		c.JSON(http.StatusOK, gin.H{
			"id":       session.Get("id"),
			"username": session.Get("username"),
			"role":     session.Get("role"),
		})
	})

	prepareRecorder := httptest.NewRecorder()
	prepareReq := httptest.NewRequest(http.MethodPost, "/prepare", nil)
	engine.ServeHTTP(prepareRecorder, prepareReq)
	sessionCookie := prepareRecorder.Header().Get("Set-Cookie")
	if sessionCookie == "" {
		t.Fatal("expected session cookie from prepare request")
	}

	code, err := totp.GenerateCode(secretKey.Secret(), time.Now())
	if err != nil {
		t.Fatalf("failed to generate login code: %v", err)
	}
	loginReq := buildJSONRequest(t, http.MethodPost, "/login/2fa", map[string]any{"code": code})
	loginReq.Header.Set("Cookie", sessionCookie)
	loginRecorder := httptest.NewRecorder()
	engine.ServeHTTP(loginRecorder, loginReq)

	loginResponse := decodeAPIResponse(t, loginRecorder)
	if !loginResponse.Success {
		t.Fatalf("expected successful twofa login, got %#v", loginResponse)
	}

	verifyReq := httptest.NewRequest(http.MethodGet, "/session", nil)
	verifyReq.Header.Set("Cookie", loginRecorder.Header().Get("Set-Cookie"))
	verifyRecorder := httptest.NewRecorder()
	engine.ServeHTTP(verifyRecorder, verifyReq)
	if verifyRecorder.Code != http.StatusOK {
		t.Fatalf("expected 200 session verify, got %d", verifyRecorder.Code)
	}
	var sessionPayload struct {
		Id       float64 `json:"id"`
		Username string  `json:"username"`
		Role     float64 `json:"role"`
	}
	if err := platformencoding.Unmarshal(verifyRecorder.Body.Bytes(), &sessionPayload); err != nil {
		t.Fatalf("failed to decode session payload: %v", err)
	}
	if int(sessionPayload.Id) != user.Id || sessionPayload.Username != user.Username || int(sessionPayload.Role) != user.Role {
		t.Fatalf("unexpected authenticated session payload: %#v", sessionPayload)
	}
}

func TestRegenerateBackupCodesReplacesAvailableCodes(t *testing.T) {
	db := setupIdentityHTTPTestDB(t)

	secretKey, err := platformsecurity.GenerateTOTPSecret("backup-regen-user", "codego-api")
	if err != nil {
		t.Fatalf("failed to generate secret: %v", err)
	}

	user := &identityschema.User{Id: 1, Username: "backup-regen-user", Password: "password123", DisplayName: "Backup Regen User", Role: constant.RoleCommonUser, Status: constant.UserStatusEnabled, Group: "default"}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}
	if err := db.Create(&identitydomain.TwoFA{UserId: user.Id, Secret: secretKey.Secret(), IsEnabled: true}).Error; err != nil {
		t.Fatalf("failed to seed twofa record: %v", err)
	}
	for _, codeValue := range []string{"ABCD-EFGH", "JKLM-NOPQ", "RSTU-VWXY", "1234-5678"} {
		hashedCode, err := platformsecurity.HashBackupCode(codeValue)
		if err != nil {
			t.Fatalf("failed to hash backup code: %v", err)
		}
		if err := db.Create(&identitydomain.TwoFABackupCode{UserId: user.Id, CodeHash: hashedCode, IsUsed: false}).Error; err != nil {
			t.Fatalf("failed to seed backup code: %v", err)
		}
	}

	code, err := totp.GenerateCode(secretKey.Secret(), time.Now())
	if err != nil {
		t.Fatalf("failed to generate totp code: %v", err)
	}
	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/user/2fa/backup_codes", map[string]any{"code": code}, user.Id)
	RegenerateBackupCodes(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected successful backup regeneration, got %#v", response)
	}

	var payload struct {
		BackupCodes []string `json:"backup_codes"`
	}
	if err := platformencoding.Unmarshal(response.Data, &payload); err != nil {
		t.Fatalf("failed to decode backup code payload: %v", err)
	}
	if len(payload.BackupCodes) != platformsecurity.BackupCodeCount {
		t.Fatalf("expected %d backup codes, got %#v", platformsecurity.BackupCodeCount, payload.BackupCodes)
	}

	var count int64
	if err := db.Model(&identitydomain.TwoFABackupCode{}).Where("user_id = ? AND is_used = false", user.Id).Count(&count).Error; err != nil {
		t.Fatalf("failed to count backup codes: %v", err)
	}
	if int(count) != platformsecurity.BackupCodeCount {
		t.Fatalf("expected %d persisted backup codes, got %d", platformsecurity.BackupCodeCount, count)
	}
}

func TestAdminDisable2FARemovesTargetConfiguration(t *testing.T) {
	db := setupIdentityHTTPTestDB(t)

	admin := &identityschema.User{Id: 1, Username: "admin-user", Password: "password123", DisplayName: "Admin User", Role: constant.RoleAdminUser, Status: constant.UserStatusEnabled, Group: "default"}
	target := &identityschema.User{Id: 2, Username: "target-user", Password: "password123", DisplayName: "Target User", Role: constant.RoleCommonUser, Status: constant.UserStatusEnabled, Group: "default"}
	if err := db.Create(admin).Error; err != nil {
		t.Fatalf("failed to seed admin: %v", err)
	}
	if err := db.Create(target).Error; err != nil {
		t.Fatalf("failed to seed target: %v", err)
	}
	if err := db.Create(&identitydomain.TwoFA{UserId: target.Id, Secret: "secret", IsEnabled: true}).Error; err != nil {
		t.Fatalf("failed to seed target twofa: %v", err)
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodDelete, fmt.Sprintf("/api/user/%d/2fa", target.Id), nil, admin.Id)
	ctx.Set("role", constant.RoleAdminUser)
	ctx.Set("username", admin.Username)
	ctx.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", target.Id)}}
	AdminDisable2FA(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected successful admin disable response, got %#v", response)
	}

	var reloaded identitydomain.TwoFA
	if err := db.Where("user_id = ?", target.Id).First(&reloaded).Error; err == nil {
		t.Fatalf("expected target 2fa to be removed, got %#v", reloaded)
	}
}

func buildJSONRequest(t *testing.T, method string, target string, body any) *http.Request {
	t.Helper()

	payload, err := platformencoding.Marshal(body)
	if err != nil {
		t.Fatalf("failed to marshal request body: %v", err)
	}

	req := httptest.NewRequest(method, target, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	return req
}
