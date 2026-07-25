package http

import (
	"bytes"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/sh2001sh/CodeGo-Api/constant"
	identitydomain "github.com/sh2001sh/CodeGo-Api/internal/identity/domain"
	identityschema "github.com/sh2001sh/CodeGo-Api/internal/identity/schema"
	identitystore "github.com/sh2001sh/CodeGo-Api/internal/identity/store"
	platformencoding "github.com/sh2001sh/CodeGo-Api/internal/platform/encodingx"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLoginCreatesAuthenticatedSession(t *testing.T) {
	setupIdentityHTTPTestDB(t)
	user := &identityschema.User{
		Id:          1,
		Username:    "auth-login-user",
		Password:    "password123",
		DisplayName: "Auth Login User",
		Role:        constant.RoleCommonUser,
		Status:      constant.UserStatusEnabled,
		Group:       "default",
	}
	if err := identitystore.CreateUser(user); err != nil {
		t.Fatalf("failed to seed login user: %v", err)
	}

	engine := gin.New()
	engine.Use(sessions.Sessions("session", cookie.NewStore([]byte("auth-test-secret"))))
	engine.POST("/login", Login)
	engine.GET("/session", func(c *gin.Context) {
		session := sessions.Default(c)
		c.JSON(http.StatusOK, gin.H{
			"id":       session.Get("id"),
			"username": session.Get("username"),
			"role":     session.Get("role"),
			"group":    session.Get("group"),
		})
	})

	loginReq := buildJSONRequest(t, http.MethodPost, "/login", map[string]any{
		"username": user.Username,
		"password": "password123",
	})
	loginRecorder := httptest.NewRecorder()
	engine.ServeHTTP(loginRecorder, loginReq)

	loginResponse := decodeAPIResponse(t, loginRecorder)
	if !loginResponse.Success {
		t.Fatalf("expected login success, got %#v", loginResponse)
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
		Group    string  `json:"group"`
	}
	if err := platformencoding.Unmarshal(verifyRecorder.Body.Bytes(), &sessionPayload); err != nil {
		t.Fatalf("failed to decode session payload: %v", err)
	}
	if int(sessionPayload.Id) != user.Id || sessionPayload.Username != user.Username || sessionPayload.Group != user.Group {
		t.Fatalf("unexpected authenticated session payload: %#v", sessionPayload)
	}
}

func TestLoginWithTwoFAStoresPendingSession(t *testing.T) {
	db := setupIdentityHTTPTestDB(t)
	user := &identityschema.User{
		Id:          1,
		Username:    "auth-login-2fa-user",
		Password:    "password123",
		DisplayName: "Auth Login 2FA User",
		Role:        constant.RoleCommonUser,
		Status:      constant.UserStatusEnabled,
		Group:       "default",
	}
	if err := identitystore.CreateUser(user); err != nil {
		t.Fatalf("failed to seed 2fa login user: %v", err)
	}
	if err := db.Create(&identitydomain.TwoFA{UserId: user.Id, Secret: "secret", IsEnabled: true}).Error; err != nil {
		t.Fatalf("failed to seed twofa record: %v", err)
	}

	engine := gin.New()
	engine.Use(sessions.Sessions("session", cookie.NewStore([]byte("auth-test-secret"))))
	engine.POST("/login", Login)
	engine.GET("/session", func(c *gin.Context) {
		session := sessions.Default(c)
		c.JSON(http.StatusOK, gin.H{
			"pending_username": session.Get("pending_username"),
			"pending_user_id":  session.Get("pending_user_id"),
		})
	})

	loginReq := buildJSONRequest(t, http.MethodPost, "/login", map[string]any{
		"username": user.Username,
		"password": "password123",
	})
	loginRecorder := httptest.NewRecorder()
	engine.ServeHTTP(loginRecorder, loginReq)

	loginResponse := decodeAPIResponse(t, loginRecorder)
	if !loginResponse.Success {
		t.Fatalf("expected login success with 2fa challenge, got %#v", loginResponse)
	}

	var payload struct {
		RequireTwoFA bool `json:"require_2fa"`
	}
	if err := platformencoding.Unmarshal(loginResponse.Data, &payload); err != nil {
		t.Fatalf("failed to decode 2fa login payload: %v", err)
	}
	if !payload.RequireTwoFA {
		t.Fatalf("expected require_2fa=true, got %#v", payload)
	}

	verifyReq := httptest.NewRequest(http.MethodGet, "/session", nil)
	verifyReq.Header.Set("Cookie", loginRecorder.Header().Get("Set-Cookie"))
	verifyRecorder := httptest.NewRecorder()
	engine.ServeHTTP(verifyRecorder, verifyReq)
	var sessionPayload struct {
		PendingUsername string  `json:"pending_username"`
		PendingUserID   float64 `json:"pending_user_id"`
	}
	if err := platformencoding.Unmarshal(verifyRecorder.Body.Bytes(), &sessionPayload); err != nil {
		t.Fatalf("failed to decode pending session payload: %v", err)
	}
	if sessionPayload.PendingUsername != user.Username || int(sessionPayload.PendingUserID) != user.Id {
		t.Fatalf("unexpected pending session payload: %#v", sessionPayload)
	}
}

func TestRegisterCreatesPasswordUser(t *testing.T) {
	setupIdentityHTTPTestDB(t)
	originalGenerateDefaultToken := constant.GenerateDefaultToken
	constant.GenerateDefaultToken = false
	t.Cleanup(func() {
		constant.GenerateDefaultToken = originalGenerateDefaultToken
	})

	body, err := platformencoding.Marshal(map[string]any{
		"username": "auth-register-user",
		"password": "password123",
	})
	if err != nil {
		t.Fatalf("failed to encode register request: %v", err)
	}
	engine := gin.New()
	engine.Use(sessions.Sessions("session", cookie.NewStore([]byte("auth-test-secret"))))
	engine.POST("/api/user/register", Register)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/user/register", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(recorder, request)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected register success, got %#v", response)
	}
	var sessionUser struct {
		ID int `json:"id"`
	}
	if err := platformencoding.Unmarshal(response.Data, &sessionUser); err != nil {
		t.Fatalf("failed to decode registered session payload: %v", err)
	}
	if sessionUser.ID != 1 {
		t.Fatalf("expected registered session user id 1, got %#v", sessionUser)
	}
	if recorder.Header().Get("Set-Cookie") == "" {
		t.Fatal("expected registration to establish an authenticated session")
	}

	user, err := loadUserByIDForTest(1, true)
	if err != nil {
		t.Fatalf("failed to reload registered user: %v", err)
	}
	if user.Username != "auth-register-user" || user.DisplayName != "auth-register-user" {
		t.Fatalf("unexpected registered user payload: %#v", user)
	}
}

func TestLogoutClearsSession(t *testing.T) {
	engine := gin.New()
	engine.Use(sessions.Sessions("session", cookie.NewStore([]byte("auth-test-secret"))))
	engine.GET("/prepare", func(c *gin.Context) {
		session := sessions.Default(c)
		session.Set("id", 1)
		session.Set("username", "logout-user")
		if err := session.Save(); err != nil {
			t.Fatalf("failed to seed logout session: %v", err)
		}
		c.Status(http.StatusOK)
	})
	engine.GET("/logout", Logout)
	engine.GET("/session", func(c *gin.Context) {
		session := sessions.Default(c)
		c.JSON(http.StatusOK, gin.H{
			"id":       session.Get("id"),
			"username": session.Get("username"),
		})
	})

	prepareRecorder := httptest.NewRecorder()
	prepareReq := httptest.NewRequest(http.MethodGet, "/prepare", nil)
	engine.ServeHTTP(prepareRecorder, prepareReq)
	sessionCookie := prepareRecorder.Header().Get("Set-Cookie")

	logoutReq := httptest.NewRequest(http.MethodGet, "/logout", nil)
	logoutReq.Header.Set("Cookie", sessionCookie)
	logoutRecorder := httptest.NewRecorder()
	engine.ServeHTTP(logoutRecorder, logoutReq)

	logoutResponse := decodeAPIResponse(t, logoutRecorder)
	if !logoutResponse.Success {
		t.Fatalf("expected logout success, got %#v", logoutResponse)
	}

	verifyReq := httptest.NewRequest(http.MethodGet, "/session", nil)
	verifyReq.Header.Set("Cookie", logoutRecorder.Header().Get("Set-Cookie"))
	verifyRecorder := httptest.NewRecorder()
	engine.ServeHTTP(verifyRecorder, verifyReq)
	var sessionPayload struct {
		Id       any `json:"id"`
		Username any `json:"username"`
	}
	if err := platformencoding.Unmarshal(verifyRecorder.Body.Bytes(), &sessionPayload); err != nil {
		t.Fatalf("failed to decode cleared session payload: %v", err)
	}
	if sessionPayload.Id != nil || sessionPayload.Username != nil {
		t.Fatalf("expected cleared session, got %#v", sessionPayload)
	}
}
