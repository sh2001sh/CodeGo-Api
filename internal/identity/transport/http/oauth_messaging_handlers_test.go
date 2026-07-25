package http

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/sh2001sh/CodeGo-Api/constant"
	identityapp "github.com/sh2001sh/CodeGo-Api/internal/identity/app"
	identityschema "github.com/sh2001sh/CodeGo-Api/internal/identity/schema"
	identitystore "github.com/sh2001sh/CodeGo-Api/internal/identity/store"
	platformconfig "github.com/sh2001sh/CodeGo-Api/internal/platform/config"
	platformencoding "github.com/sh2001sh/CodeGo-Api/internal/platform/encodingx"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"
)

func TestHandleWeChatOAuthCreatesUserAndSession(t *testing.T) {
	setupIdentityHTTPTestDB(t)

	previousEnabled := platformconfig.WeChatAuthEnabled
	previousRegisterEnabled := platformconfig.RegisterEnabled
	previousAddress := platformconfig.WeChatServerAddress
	previousToken := platformconfig.WeChatServerToken
	platformconfig.WeChatAuthEnabled = true
	platformconfig.RegisterEnabled = true
	platformconfig.WeChatServerAddress = "https://wechat.example"
	platformconfig.WeChatServerToken = "wechat-secret"
	identityapp.SetWeChatHTTPClientForTest(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		recorder := httptest.NewRecorder()
		recorder.WriteHeader(http.StatusOK)
		_, _ = recorder.WriteString(`{"success":true,"message":"","data":"wechat-open-id-1"}`)
		return recorder.Result(), nil
	})})
	t.Cleanup(func() {
		platformconfig.WeChatAuthEnabled = previousEnabled
		platformconfig.RegisterEnabled = previousRegisterEnabled
		platformconfig.WeChatServerAddress = previousAddress
		platformconfig.WeChatServerToken = previousToken
		identityapp.SetWeChatHTTPClientForTest(nil)
	})

	engine := gin.New()
	engine.Use(sessions.Sessions("session", cookie.NewStore([]byte("wechat-test-secret"))))
	engine.GET("/oauth/wechat", HandleWeChatOAuth)
	engine.GET("/session", func(c *gin.Context) {
		session := sessions.Default(c)
		c.JSON(http.StatusOK, gin.H{
			"id":       session.Get("id"),
			"username": session.Get("username"),
		})
	})

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/oauth/wechat?code=wechat-code", nil)
	engine.ServeHTTP(recorder, req)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got %#v", response)
	}

	user, err := loadUserByIDForTest(1, false)
	if err != nil {
		t.Fatalf("failed to reload wechat user: %v", err)
	}
	if user.Username != "wechat_1" || user.WeChatId != "wechat-open-id-1" {
		t.Fatalf("unexpected wechat user: %#v", user)
	}

	sessionRecorder := httptest.NewRecorder()
	sessionReq := httptest.NewRequest(http.MethodGet, "/session", nil)
	sessionReq.Header.Set("Cookie", recorder.Header().Get("Set-Cookie"))
	engine.ServeHTTP(sessionRecorder, sessionReq)

	var sessionPayload struct {
		ID       int    `json:"id"`
		Username string `json:"username"`
	}
	if err := platformencoding.Unmarshal(sessionRecorder.Body.Bytes(), &sessionPayload); err != nil {
		t.Fatalf("failed to decode session payload: %v", err)
	}
	if sessionPayload.ID != 1 || sessionPayload.Username != "wechat_1" {
		t.Fatalf("unexpected session payload: %#v", sessionPayload)
	}
}

func TestBindWeChatOAuthPersistsBinding(t *testing.T) {
	setupIdentityHTTPTestDB(t)

	user := &identityschema.User{
		Id:          1,
		Username:    "wechat-bind-user",
		Password:    "password123",
		DisplayName: "WeChat Bind User",
		Role:        constant.RoleCommonUser,
		Status:      constant.UserStatusEnabled,
		Group:       "default",
	}
	if err := identitystore.CreateUser(user); err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}

	previousEnabled := platformconfig.WeChatAuthEnabled
	previousAddress := platformconfig.WeChatServerAddress
	previousToken := platformconfig.WeChatServerToken
	platformconfig.WeChatAuthEnabled = true
	platformconfig.WeChatServerAddress = "https://wechat.example"
	platformconfig.WeChatServerToken = "wechat-secret"
	identityapp.SetWeChatHTTPClientForTest(&http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		recorder := httptest.NewRecorder()
		recorder.WriteHeader(http.StatusOK)
		_, _ = recorder.WriteString(`{"success":true,"message":"","data":"wechat-bind-open-id"}`)
		return recorder.Result(), nil
	})})
	t.Cleanup(func() {
		platformconfig.WeChatAuthEnabled = previousEnabled
		platformconfig.WeChatServerAddress = previousAddress
		platformconfig.WeChatServerToken = previousToken
		identityapp.SetWeChatHTTPClientForTest(nil)
	})

	engine := gin.New()
	engine.Use(sessions.Sessions("session", cookie.NewStore([]byte("wechat-bind-secret"))))
	engine.GET("/prepare", func(c *gin.Context) {
		session := sessions.Default(c)
		session.Set("id", user.Id)
		if err := session.Save(); err != nil {
			t.Fatalf("failed to seed bind session: %v", err)
		}
		c.Status(http.StatusOK)
	})
	engine.POST("/oauth/wechat/bind", BindWeChatOAuth)

	prepareRecorder := httptest.NewRecorder()
	engine.ServeHTTP(prepareRecorder, httptest.NewRequest(http.MethodGet, "/prepare", nil))
	sessionCookie := prepareRecorder.Header().Get("Set-Cookie")

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/oauth/wechat/bind", bytes.NewBufferString(`{"code":"bind-code"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Cookie", sessionCookie)
	engine.ServeHTTP(recorder, req)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got %#v", response)
	}

	reloaded, err := loadUserByIDForTest(user.Id, false)
	if err != nil {
		t.Fatalf("failed to reload bound user: %v", err)
	}
	if reloaded.WeChatId != "wechat-bind-open-id" {
		t.Fatalf("expected wechat binding to persist, got %#v", reloaded)
	}
}

func TestHandleTelegramLoginCreatesSessionForBoundUser(t *testing.T) {
	setupIdentityHTTPTestDB(t)

	user := &identityschema.User{
		Id:          1,
		Username:    "telegram-user",
		Password:    "password123",
		DisplayName: "Telegram User",
		Role:        constant.RoleCommonUser,
		Status:      constant.UserStatusEnabled,
		Group:       "default",
		TelegramId:  "tg-user-1",
	}
	if err := identitystore.CreateUser(user); err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}

	previousEnabled := platformconfig.TelegramOAuthEnabled
	previousBotToken := platformconfig.TelegramBotToken
	platformconfig.TelegramOAuthEnabled = true
	platformconfig.TelegramBotToken = "telegram-bot-token"
	t.Cleanup(func() {
		platformconfig.TelegramOAuthEnabled = previousEnabled
		platformconfig.TelegramBotToken = previousBotToken
	})

	engine := gin.New()
	engine.Use(sessions.Sessions("session", cookie.NewStore([]byte("telegram-test-secret"))))
	engine.GET("/oauth/telegram/login", HandleTelegramLogin)
	engine.GET("/session", func(c *gin.Context) {
		session := sessions.Default(c)
		c.JSON(http.StatusOK, gin.H{
			"id":       session.Get("id"),
			"username": session.Get("username"),
		})
	})

	target := "/oauth/telegram/login?" + buildTelegramQuery("telegram-bot-token", map[string]string{
		"id":         "tg-user-1",
		"first_name": "Telegram",
		"auth_date":  "1720000000",
	})
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, target, nil))

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected telegram login success, got %#v", response)
	}

	sessionRecorder := httptest.NewRecorder()
	sessionReq := httptest.NewRequest(http.MethodGet, "/session", nil)
	sessionReq.Header.Set("Cookie", recorder.Header().Get("Set-Cookie"))
	engine.ServeHTTP(sessionRecorder, sessionReq)

	var sessionPayload struct {
		ID       int    `json:"id"`
		Username string `json:"username"`
	}
	if err := platformencoding.Unmarshal(sessionRecorder.Body.Bytes(), &sessionPayload); err != nil {
		t.Fatalf("failed to decode session payload: %v", err)
	}
	if sessionPayload.ID != user.Id || sessionPayload.Username != user.Username {
		t.Fatalf("unexpected session payload: %#v", sessionPayload)
	}
}

func TestBindTelegramOAuthRedirectsAndPersistsBinding(t *testing.T) {
	setupIdentityHTTPTestDB(t)

	user := &identityschema.User{
		Id:          1,
		Username:    "telegram-bind-user",
		Password:    "password123",
		DisplayName: "Telegram Bind User",
		Role:        constant.RoleCommonUser,
		Status:      constant.UserStatusEnabled,
		Group:       "default",
	}
	if err := identitystore.CreateUser(user); err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}

	previousEnabled := platformconfig.TelegramOAuthEnabled
	previousBotToken := platformconfig.TelegramBotToken
	platformconfig.TelegramOAuthEnabled = true
	platformconfig.TelegramBotToken = "telegram-bot-token"
	t.Cleanup(func() {
		platformconfig.TelegramOAuthEnabled = previousEnabled
		platformconfig.TelegramBotToken = previousBotToken
	})

	engine := gin.New()
	engine.Use(sessions.Sessions("session", cookie.NewStore([]byte("telegram-bind-secret"))))
	engine.GET("/prepare", func(c *gin.Context) {
		session := sessions.Default(c)
		session.Set("id", user.Id)
		if err := session.Save(); err != nil {
			t.Fatalf("failed to seed telegram bind session: %v", err)
		}
		c.Status(http.StatusOK)
	})
	engine.GET("/oauth/telegram/bind", func(c *gin.Context) {
		BindTelegramOAuth(c)
	})

	prepareRecorder := httptest.NewRecorder()
	engine.ServeHTTP(prepareRecorder, httptest.NewRequest(http.MethodGet, "/prepare", nil))
	sessionCookie := prepareRecorder.Header().Get("Set-Cookie")

	target := "/oauth/telegram/bind?" + buildTelegramQuery("telegram-bot-token", map[string]string{
		"id":         "tg-bind-1",
		"first_name": "Telegram",
		"auth_date":  "1720000000",
	})
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	req.Header.Set("Cookie", sessionCookie)
	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusFound {
		t.Fatalf("expected redirect status, got %d", recorder.Code)
	}
	if location := recorder.Header().Get("Location"); location != "/profile" {
		t.Fatalf("expected redirect to /profile, got %q", location)
	}

	reloaded, err := loadUserByIDForTest(user.Id, false)
	if err != nil {
		t.Fatalf("failed to reload telegram bound user: %v", err)
	}
	if reloaded.TelegramId != "tg-bind-1" {
		t.Fatalf("expected telegram binding to persist, got %#v", reloaded)
	}
}

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func buildTelegramQuery(token string, values map[string]string) string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	lines := make([]string, 0, len(values))
	for _, key := range keys {
		lines = append(lines, key+"="+values[key])
	}

	sha256Hash := sha256.New()
	io.WriteString(sha256Hash, token)
	hmacHash := hmac.New(sha256.New, sha256Hash.Sum(nil))
	io.WriteString(hmacHash, joinLines(lines))
	hash := hex.EncodeToString(hmacHash.Sum(nil))

	query := ""
	for _, key := range keys {
		if query != "" {
			query += "&"
		}
		query += key + "=" + values[key]
	}
	return query + "&hash=" + hash
}

func joinLines(lines []string) string {
	result := ""
	for _, line := range lines {
		if result != "" {
			result += "\n"
		}
		result += line
	}
	return result
}
