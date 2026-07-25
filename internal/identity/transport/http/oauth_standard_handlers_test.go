package http

import (
	"context"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/sh2001sh/CodeGo-Api/constant"
	"github.com/sh2001sh/CodeGo-Api/internal/identity/oauth"
	identityschema "github.com/sh2001sh/CodeGo-Api/internal/identity/schema"
	identitystore "github.com/sh2001sh/CodeGo-Api/internal/identity/store"
	"net/http"
	"net/http/httptest"
	"testing"
)

type stubOAuthProvider struct {
	name           string
	enabled        bool
	providerUserID string
	username       string
	displayName    string
	email          string
}

func (p *stubOAuthProvider) GetName() string {
	return p.name
}

func (p *stubOAuthProvider) IsEnabled() bool {
	return p.enabled
}

func (p *stubOAuthProvider) ExchangeToken(ctx context.Context, code string, c *gin.Context) (*oauth.OAuthToken, error) {
	return &oauth.OAuthToken{AccessToken: "stub-token", TokenType: "Bearer"}, nil
}

func (p *stubOAuthProvider) GetUserInfo(ctx context.Context, token *oauth.OAuthToken) (*oauth.OAuthUser, error) {
	return &oauth.OAuthUser{
		ProviderUserID: p.providerUserID,
		Username:       p.username,
		DisplayName:    p.displayName,
		Email:          p.email,
	}, nil
}

func (p *stubOAuthProvider) IsUserIDTaken(providerUserID string) bool {
	return identitystore.IsGitHubIDTaken(providerUserID)
}

func (p *stubOAuthProvider) FillUserByProviderID(user *identityschema.User, providerUserID string) error {
	loadedUser, err := identitystore.LoadUserByGitHubID(providerUserID)
	if err != nil {
		return err
	}
	*user = *loadedUser
	return nil
}

func (p *stubOAuthProvider) SetProviderUserID(user *identityschema.User, providerUserID string) {
	user.GitHubId = providerUserID
}

func (p *stubOAuthProvider) GetProviderPrefix() string {
	return "stub_"
}

func TestHandleOAuthLogsInNewUser(t *testing.T) {
	setupIdentityHTTPTestDB(t)

	provider := &stubOAuthProvider{
		name:           "StubOAuth",
		enabled:        true,
		providerUserID: "stub-provider-id",
		username:       "oauth-user",
		displayName:    "OAuth User",
		email:          "oauth-user@example.com",
	}
	oauth.Register("stub", provider)
	t.Cleanup(func() {
		oauth.Unregister("stub")
	})

	engine := gin.New()
	engine.Use(sessions.Sessions("session", cookie.NewStore([]byte("oauth-test-secret"))))
	engine.GET("/prepare", func(c *gin.Context) {
		session := sessions.Default(c)
		session.Set("oauth_state", "state123")
		if err := session.Save(); err != nil {
			t.Fatalf("failed to seed oauth state: %v", err)
		}
		c.Status(http.StatusOK)
	})
	engine.GET("/oauth/:provider", HandleOAuth)

	prepareRecorder := httptest.NewRecorder()
	engine.ServeHTTP(prepareRecorder, httptest.NewRequest(http.MethodGet, "/prepare", nil))
	sessionCookie := prepareRecorder.Header().Get("Set-Cookie")

	req := httptest.NewRequest(http.MethodGet, "/oauth/stub?state=state123&code=oauth-code", nil)
	req.Header.Set("Cookie", sessionCookie)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected oauth login success, got %#v", response)
	}

	user, err := loadUserByIDForTest(1, false)
	if err != nil {
		t.Fatalf("failed to reload oauth user: %v", err)
	}
	if user.Username != "oauth-user" || user.GitHubId != "stub-provider-id" {
		t.Fatalf("unexpected oauth user: %#v", user)
	}
}

func TestHandleOAuthBindsLoggedInUser(t *testing.T) {
	setupIdentityHTTPTestDB(t)

	user := &identityschema.User{
		Id:          1,
		Username:    "oauth-bind-user",
		Password:    "password123",
		DisplayName: "OAuth Bind User",
		Role:        constant.RoleCommonUser,
		Status:      constant.UserStatusEnabled,
		Group:       "default",
	}
	if err := identitystore.CreateUser(user); err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}

	provider := &stubOAuthProvider{
		name:           "StubOAuth",
		enabled:        true,
		providerUserID: "bind-provider-id",
		username:       "oauth-bind-provider",
		displayName:    "OAuth Bind Provider",
		email:          "bind@example.com",
	}
	oauth.Register("stubbind", provider)
	t.Cleanup(func() {
		oauth.Unregister("stubbind")
	})

	engine := gin.New()
	engine.Use(sessions.Sessions("session", cookie.NewStore([]byte("oauth-test-secret"))))
	engine.GET("/prepare", func(c *gin.Context) {
		session := sessions.Default(c)
		session.Set("oauth_state", "state456")
		session.Set("id", user.Id)
		session.Set("username", user.Username)
		if err := session.Save(); err != nil {
			t.Fatalf("failed to seed oauth bind session: %v", err)
		}
		c.Status(http.StatusOK)
	})
	engine.GET("/oauth/:provider", HandleOAuth)

	prepareRecorder := httptest.NewRecorder()
	engine.ServeHTTP(prepareRecorder, httptest.NewRequest(http.MethodGet, "/prepare", nil))
	sessionCookie := prepareRecorder.Header().Get("Set-Cookie")

	req := httptest.NewRequest(http.MethodGet, "/oauth/stubbind?state=state456&code=oauth-code", nil)
	req.Header.Set("Cookie", sessionCookie)
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, req)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected oauth bind success, got %#v", response)
	}

	reloaded, err := loadUserByIDForTest(user.Id, false)
	if err != nil {
		t.Fatalf("failed to reload bound user: %v", err)
	}
	if reloaded.GitHubId != "bind-provider-id" {
		t.Fatalf("expected provider binding to persist, got %#v", reloaded)
	}
}
