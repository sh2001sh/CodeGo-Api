package http

import (
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/sh2001sh/CodeGo-Api/constant"
	identityapp "github.com/sh2001sh/CodeGo-Api/internal/identity/app"
	identityschema "github.com/sh2001sh/CodeGo-Api/internal/identity/schema"
	identitystore "github.com/sh2001sh/CodeGo-Api/internal/identity/store"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBindEmailPersistsVerifiedEmail(t *testing.T) {
	setupIdentityHTTPTestDB(t)

	user := &identityschema.User{
		Id:          1,
		Username:    "bind-email-user",
		Password:    "password123",
		DisplayName: "Bind Email User",
		Role:        constant.RoleCommonUser,
		Status:      constant.UserStatusEnabled,
		Group:       "default",
	}
	if err := identitystore.CreateUser(user); err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}

	email := "bind@example.com"
	code := "123456"
	identityapp.RegisterVerificationCodeWithKey(email, code, identityapp.EmailVerificationPurpose)

	engine := gin.New()
	engine.Use(sessions.Sessions("session", cookie.NewStore([]byte("bind-email-test-secret"))))
	engine.GET("/prepare", func(c *gin.Context) {
		session := sessions.Default(c)
		session.Set("id", user.Id)
		if err := session.Save(); err != nil {
			t.Fatalf("failed to seed bind-email session: %v", err)
		}
		c.Status(http.StatusOK)
	})
	engine.POST("/bind", BindEmail)

	prepareRecorder := httptest.NewRecorder()
	prepareReq := httptest.NewRequest(http.MethodGet, "/prepare", nil)
	engine.ServeHTTP(prepareRecorder, prepareReq)
	sessionCookie := prepareRecorder.Header().Get("Set-Cookie")

	bindReq := buildJSONRequest(t, http.MethodPost, "/bind", map[string]any{
		"email": email,
		"code":  code,
	})
	bindReq.Header.Set("Cookie", sessionCookie)
	bindRecorder := httptest.NewRecorder()
	engine.ServeHTTP(bindRecorder, bindReq)

	response := decodeAPIResponse(t, bindRecorder)
	if !response.Success {
		t.Fatalf("expected bind email success, got %#v", response)
	}

	reloaded, err := loadUserByIDForTest(user.Id, false)
	if err != nil {
		t.Fatalf("failed to reload user: %v", err)
	}
	if reloaded.Email != email {
		t.Fatalf("expected email %q, got %q", email, reloaded.Email)
	}
}

func TestBindEmailRejectsInvalidVerificationCode(t *testing.T) {
	setupIdentityHTTPTestDB(t)

	user := &identityschema.User{
		Id:          1,
		Username:    "bind-email-invalid-user",
		Password:    "password123",
		DisplayName: "Bind Email Invalid User",
		Role:        constant.RoleCommonUser,
		Status:      constant.UserStatusEnabled,
		Group:       "default",
	}
	if err := identitystore.CreateUser(user); err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}

	engine := gin.New()
	engine.Use(sessions.Sessions("session", cookie.NewStore([]byte("bind-email-test-secret"))))
	engine.GET("/prepare", func(c *gin.Context) {
		session := sessions.Default(c)
		session.Set("id", user.Id)
		if err := session.Save(); err != nil {
			t.Fatalf("failed to seed bind-email session: %v", err)
		}
		c.Status(http.StatusOK)
	})
	engine.POST("/bind", BindEmail)

	prepareRecorder := httptest.NewRecorder()
	prepareReq := httptest.NewRequest(http.MethodGet, "/prepare", nil)
	engine.ServeHTTP(prepareRecorder, prepareReq)
	sessionCookie := prepareRecorder.Header().Get("Set-Cookie")

	bindReq := buildJSONRequest(t, http.MethodPost, "/bind", map[string]any{
		"email": "bind-invalid@example.com",
		"code":  "wrong",
	})
	bindReq.Header.Set("Cookie", sessionCookie)
	bindRecorder := httptest.NewRecorder()
	engine.ServeHTTP(bindRecorder, bindReq)

	response := decodeAPIResponse(t, bindRecorder)
	if response.Success {
		t.Fatalf("expected bind email failure, got %#v", response)
	}
}
