package http

import (
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/pquerna/otp/totp"
	"github.com/sh2001sh/CodeGo-Api/constant"
	identitydomain "github.com/sh2001sh/CodeGo-Api/internal/identity/domain"
	identityschema "github.com/sh2001sh/CodeGo-Api/internal/identity/schema"
	"github.com/sh2001sh/CodeGo-Api/internal/identity/sessionstate"
	platformencoding "github.com/sh2001sh/CodeGo-Api/internal/platform/encodingx"
	platformsecurity "github.com/sh2001sh/CodeGo-Api/internal/platform/security"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestUniversalVerifyWithTwoFAStoresSecureVerificationSession(t *testing.T) {
	db := setupIdentityHTTPTestDB(t)

	secretKey, err := platformsecurity.GenerateTOTPSecret("verify-twofa-user", "codego-api")
	if err != nil {
		t.Fatalf("failed to generate secret: %v", err)
	}
	user := &identityschema.User{Id: 1, Username: "verify-twofa-user", Password: "password123", DisplayName: "Verify TwoFA User", Role: constant.RoleCommonUser, Status: constant.UserStatusEnabled, Group: "default"}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}
	if err := db.Create(&identitydomain.TwoFA{UserId: user.Id, Secret: secretKey.Secret(), IsEnabled: true}).Error; err != nil {
		t.Fatalf("failed to seed twofa: %v", err)
	}

	engine := gin.New()
	engine.Use(sessions.Sessions("session", cookie.NewStore([]byte("secure-verify-test"))))
	engine.POST("/verify", func(c *gin.Context) {
		c.Set("id", user.Id)
		session := sessions.Default(c)
		if err := session.Save(); err != nil {
			t.Fatalf("failed to seed session: %v", err)
		}
		UniversalVerify(c)
	})
	engine.GET("/session", func(c *gin.Context) {
		session := sessions.Default(c)
		c.JSON(http.StatusOK, gin.H{
			"verified_at": session.Get(sessionstate.SecureVerificationSessionKey),
			"method":      session.Get(sessionstate.SecureVerificationMethodSessionKey),
		})
	})

	code, err := totp.GenerateCode(secretKey.Secret(), time.Now())
	if err != nil {
		t.Fatalf("failed to generate totp code: %v", err)
	}
	verifyReq := buildJSONRequest(t, http.MethodPost, "/verify", map[string]any{"method": "2fa", "code": code})
	verifyRecorder := httptest.NewRecorder()
	engine.ServeHTTP(verifyRecorder, verifyReq)

	response := decodeAPIResponse(t, verifyRecorder)
	if !response.Success {
		t.Fatalf("expected successful verify response, got %#v", response)
	}

	sessionReq := httptest.NewRequest(http.MethodGet, "/session", nil)
	sessionReq.Header.Set("Cookie", verifyRecorder.Header().Get("Set-Cookie"))
	sessionRecorder := httptest.NewRecorder()
	engine.ServeHTTP(sessionRecorder, sessionReq)

	var payload struct {
		VerifiedAt float64 `json:"verified_at"`
		Method     string  `json:"method"`
	}
	if err := platformencoding.Unmarshal(sessionRecorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode session payload: %v", err)
	}
	if payload.VerifiedAt <= 0 || payload.Method != "2fa" {
		t.Fatalf("unexpected secure verification session payload: %#v", payload)
	}
}

func TestUniversalVerifyWithPasskeyConsumesReadyMarker(t *testing.T) {
	db := setupIdentityHTTPTestDB(t)

	user := &identityschema.User{Id: 1, Username: "verify-passkey-user", Password: "password123", DisplayName: "Verify Passkey User", Role: constant.RoleCommonUser, Status: constant.UserStatusEnabled, Group: "default"}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}
	credential := &identitydomain.PasskeyCredential{
		UserID:       user.Id,
		CredentialID: "dmVyaWZ5LXBhc3NrZXk=",
		PublicKey:    "cHVibGljLWtleQ==",
	}
	if err := db.Create(credential).Error; err != nil {
		t.Fatalf("failed to seed passkey credential: %v", err)
	}

	engine := gin.New()
	engine.Use(sessions.Sessions("session", cookie.NewStore([]byte("secure-passkey-test"))))
	engine.POST("/verify", func(c *gin.Context) {
		c.Set("id", user.Id)
		session := sessions.Default(c)
		session.Set(sessionstate.PasskeyReadySessionKey, time.Now().Unix())
		if err := session.Save(); err != nil {
			t.Fatalf("failed to seed passkey ready session: %v", err)
		}
		UniversalVerify(c)
	})

	verifyReq := buildJSONRequest(t, http.MethodPost, "/verify", map[string]any{"method": "passkey"})
	verifyRecorder := httptest.NewRecorder()
	engine.ServeHTTP(verifyRecorder, verifyReq)

	response := decodeAPIResponse(t, verifyRecorder)
	if !response.Success {
		t.Fatalf("expected successful verify response, got %#v", response)
	}
}
