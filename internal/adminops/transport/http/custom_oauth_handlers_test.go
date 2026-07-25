package http

import (
	"bytes"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/sh2001sh/CodeGo-Api/constant"
	commerceschema "github.com/sh2001sh/CodeGo-Api/internal/commerce/schema"
	gatewayschema "github.com/sh2001sh/CodeGo-Api/internal/gateway/schema"
	identitydomain "github.com/sh2001sh/CodeGo-Api/internal/identity/domain"
	"github.com/sh2001sh/CodeGo-Api/internal/identity/oauth"
	identityschema "github.com/sh2001sh/CodeGo-Api/internal/identity/schema"
	platformcache "github.com/sh2001sh/CodeGo-Api/internal/platform/cache"
	platformconfig "github.com/sh2001sh/CodeGo-Api/internal/platform/config"
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
	platformencoding "github.com/sh2001sh/CodeGo-Api/internal/platform/encodingx"
	platformschema "github.com/sh2001sh/CodeGo-Api/internal/platform/schema"

	"gorm.io/gorm"
	stdhttp "net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

type adminOpsAPIResponse struct {
	Success bool            `json:"success"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func setupAdminOpsHTTPTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	gin.SetMode(gin.TestMode)
	platformdb.UsingSQLite = true
	platformdb.UsingMySQL = false
	platformdb.UsingPostgreSQL = false
	platformcache.RedisEnabled = false
	platformconfig.OptionMap = map[string]string{}

	dsn := "file:" + strings.ReplaceAll(t.Name(), "/", "_") + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	platformdb.DB = db
	platformdb.LogDB = db

	if err := db.AutoMigrate(
		&identityschema.User{},
		&identitydomain.CustomOAuthProvider{},
		&identitydomain.UserOAuthBinding{},
		&gatewayschema.Channel{},
		&platformschema.Option{},
		&gatewayschema.PrefillGroup{},
		&commerceschema.Redemption{},
		&commerceschema.SubscriptionPlan{},
		&gatewayschema.Vendor{},
	); err != nil {
		t.Fatalf("failed to migrate adminops http tables: %v", err)
	}

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func newAdminOpsContext(t *testing.T, method string, target string, body any) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()

	var requestBody *bytes.Reader
	if body != nil {
		payload, err := platformencoding.Marshal(body)
		if err != nil {
			t.Fatalf("failed to marshal request body: %v", err)
		}
		requestBody = bytes.NewReader(payload)
	} else {
		requestBody = bytes.NewReader(nil)
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(method, target, requestBody)
	if body != nil {
		ctx.Request.Header.Set("Content-Type", "application/json")
	}
	return ctx, recorder
}

func decodeAdminOpsResponse(t *testing.T, recorder *httptest.ResponseRecorder) adminOpsAPIResponse {
	t.Helper()

	var response adminOpsAPIResponse
	if err := platformencoding.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode api response: %v", err)
	}
	return response
}

func TestFetchCustomOAuthDiscoveryReturnsDocument(t *testing.T) {
	server := httptest.NewServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"issuer":"https://issuer.example.com","authorization_endpoint":"https://issuer.example.com/auth"}`))
	}))
	defer server.Close()

	ctx, recorder := newAdminOpsContext(t, stdhttp.MethodPost, "/api/custom-oauth-provider/discovery", map[string]any{
		"well_known_url": server.URL,
	})
	FetchCustomOAuthDiscovery(ctx)

	response := decodeAdminOpsResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected discovery request to succeed, got %#v", response)
	}

	var payload struct {
		WellKnownURL string         `json:"well_known_url"`
		Discovery    map[string]any `json:"discovery"`
	}
	if err := platformencoding.Unmarshal(response.Data, &payload); err != nil {
		t.Fatalf("failed to decode discovery payload: %v", err)
	}
	if payload.WellKnownURL != server.URL {
		t.Fatalf("expected discovery url %q, got %q", server.URL, payload.WellKnownURL)
	}
	if payload.Discovery["issuer"] != "https://issuer.example.com" {
		t.Fatalf("unexpected discovery payload: %#v", payload.Discovery)
	}
}

func TestGetCustomOAuthProvidersOmitsClientSecret(t *testing.T) {
	db := setupAdminOpsHTTPTestDB(t)
	provider := &identitydomain.CustomOAuthProvider{
		Name:                  "Provider One",
		Slug:                  "provider-one",
		ClientId:              "client-1",
		ClientSecret:          "super-secret",
		AuthorizationEndpoint: "https://provider.example.com/auth",
		TokenEndpoint:         "https://provider.example.com/token",
		UserInfoEndpoint:      "https://provider.example.com/userinfo",
	}
	if err := db.Create(provider).Error; err != nil {
		t.Fatalf("failed to seed provider: %v", err)
	}

	ctx, recorder := newAdminOpsContext(t, stdhttp.MethodGet, "/api/custom-oauth-provider/", nil)
	GetCustomOAuthProviders(ctx)

	response := decodeAdminOpsResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected provider list to succeed, got %#v", response)
	}
	if strings.Contains(recorder.Body.String(), "super-secret") {
		t.Fatalf("provider list leaked client secret: %s", recorder.Body.String())
	}
}

func TestCreateCustomOAuthProviderRegistersProvider(t *testing.T) {
	setupAdminOpsHTTPTestDB(t)
	slug := "test-create-provider"
	t.Cleanup(func() {
		oauth.UnregisterCustomProvider(slug)
	})

	ctx, recorder := newAdminOpsContext(t, stdhttp.MethodPost, "/api/custom-oauth-provider/", map[string]any{
		"name":                   "Create Provider",
		"slug":                   slug,
		"enabled":                true,
		"client_id":              "client-create",
		"client_secret":          "secret-create",
		"authorization_endpoint": "https://provider.example.com/auth",
		"token_endpoint":         "https://provider.example.com/token",
		"user_info_endpoint":     "https://provider.example.com/userinfo",
	})
	CreateCustomOAuthProvider(ctx)

	response := decodeAdminOpsResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected create provider to succeed, got %#v", response)
	}
	if !oauth.IsCustomProvider(slug) {
		t.Fatalf("expected provider slug %q to be registered as custom", slug)
	}
	if strings.Contains(recorder.Body.String(), "secret-create") {
		t.Fatalf("create response leaked client secret: %s", recorder.Body.String())
	}
}

func TestUpdateCustomOAuthProviderReplacesRegistrySlug(t *testing.T) {
	db := setupAdminOpsHTTPTestDB(t)
	provider := &identitydomain.CustomOAuthProvider{
		Name:                  "Updatable Provider",
		Slug:                  "old-provider-slug",
		ClientId:              "client-old",
		ClientSecret:          "secret-old",
		AuthorizationEndpoint: "https://provider.example.com/auth",
		TokenEndpoint:         "https://provider.example.com/token",
		UserInfoEndpoint:      "https://provider.example.com/userinfo",
	}
	if err := db.Create(provider).Error; err != nil {
		t.Fatalf("failed to seed provider: %v", err)
	}
	oauth.RegisterOrUpdateCustomProvider(provider)
	t.Cleanup(func() {
		oauth.UnregisterCustomProvider("old-provider-slug")
		oauth.UnregisterCustomProvider("new-provider-slug")
	})

	ctx, recorder := newAdminOpsContext(t, stdhttp.MethodPut, "/api/custom-oauth-provider/"+strconv.Itoa(provider.Id), map[string]any{
		"slug": "new-provider-slug",
	})
	ctx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(provider.Id)}}
	UpdateCustomOAuthProvider(ctx)

	response := decodeAdminOpsResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected update provider to succeed, got %#v", response)
	}
	if oauth.IsCustomProvider("old-provider-slug") {
		t.Fatalf("expected old slug to be removed from registry")
	}
	if !oauth.IsCustomProvider("new-provider-slug") {
		t.Fatalf("expected new slug to be registered")
	}
}

func TestDeleteCustomOAuthProviderRejectsExistingBindings(t *testing.T) {
	db := setupAdminOpsHTTPTestDB(t)
	provider := &identitydomain.CustomOAuthProvider{
		Name:                  "Bound Provider",
		Slug:                  "bound-provider",
		ClientId:              "client-bound",
		ClientSecret:          "secret-bound",
		AuthorizationEndpoint: "https://provider.example.com/auth",
		TokenEndpoint:         "https://provider.example.com/token",
		UserInfoEndpoint:      "https://provider.example.com/userinfo",
	}
	user := &identityschema.User{Id: 1, Username: "binding-user", Password: "password123", DisplayName: "Binding User", Role: constant.RoleCommonUser, Status: constant.UserStatusEnabled, Group: "default"}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}
	if err := db.Create(provider).Error; err != nil {
		t.Fatalf("failed to seed provider: %v", err)
	}
	if err := db.Create(&identitydomain.UserOAuthBinding{UserId: user.Id, ProviderId: provider.Id, ProviderUserId: "provider-user-1"}).Error; err != nil {
		t.Fatalf("failed to seed binding: %v", err)
	}

	ctx, recorder := newAdminOpsContext(t, stdhttp.MethodDelete, "/api/custom-oauth-provider/"+strconv.Itoa(provider.Id), nil)
	ctx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(provider.Id)}}
	DeleteCustomOAuthProvider(ctx)

	response := decodeAdminOpsResponse(t, recorder)
	if response.Success {
		t.Fatalf("expected delete provider to fail when bindings exist")
	}
}
