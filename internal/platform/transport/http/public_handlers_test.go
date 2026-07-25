package http

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/sh2001sh/CodeGo-Api/constant"
	identityschema "github.com/sh2001sh/CodeGo-Api/internal/identity/schema"
	platformcache "github.com/sh2001sh/CodeGo-Api/internal/platform/cache"
	platformconfig "github.com/sh2001sh/CodeGo-Api/internal/platform/config"
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
	platformencoding "github.com/sh2001sh/CodeGo-Api/internal/platform/encodingx"
	platformops "github.com/sh2001sh/CodeGo-Api/internal/platform/opssettings"
	platformschema "github.com/sh2001sh/CodeGo-Api/internal/platform/schema"
	platformstore "github.com/sh2001sh/CodeGo-Api/internal/platform/store"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func setupPlatformHTTPTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	gin.SetMode(gin.TestMode)
	platformdb.UsingSQLite = true
	platformdb.UsingMySQL = false
	platformdb.UsingPostgreSQL = false
	platformcache.RedisEnabled = false
	originalOptionMap := platformconfig.OptionMap

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	platformdb.DB = db
	platformdb.LogDB = db
	require.NoError(t, db.AutoMigrate(&identityschema.User{}, &platformschema.Option{}, &platformschema.Setup{}))
	platformstore.InitOptionMap()

	t.Cleanup(func() {
		platformconfig.OptionMap = originalOptionMap
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func TestGetSetupReturnsDatabaseTypeWhenNotInitialized(t *testing.T) {
	setupPlatformHTTPTestDB(t)

	originalSetup := constant.Setup
	constant.Setup = false
	t.Cleanup(func() {
		constant.Setup = originalSetup
	})

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/setup", nil)

	GetSetup(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Success bool `json:"success"`
		Data    struct {
			Status       bool   `json:"status"`
			RootInit     bool   `json:"root_init"`
			DatabaseType string `json:"database_type"`
		} `json:"data"`
	}
	require.NoError(t, platformencoding.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	require.False(t, response.Data.Status)
	require.False(t, response.Data.RootInit)
	require.Equal(t, "sqlite", response.Data.DatabaseType)
}

func TestPostSetupCreatesRootUserAndPersistsFlags(t *testing.T) {
	db := setupPlatformHTTPTestDB(t)

	originalSetup := constant.Setup
	originalSelfUse := platformops.IsSelfUseModeEnabled()
	originalDemoSite := platformops.IsDemoSiteEnabled()
	constant.Setup = false
	platformops.SetSelfUseModeEnabled(false)
	platformops.SetDemoSiteEnabled(false)
	t.Cleanup(func() {
		constant.Setup = originalSetup
		platformops.SetSelfUseModeEnabled(originalSelfUse)
		platformops.SetDemoSiteEnabled(originalDemoSite)
	})

	ctx, recorder := newPlatformJSONContext(t, http.MethodPost, "/api/setup", map[string]any{
		"username":           "rootadmin",
		"password":           "password123",
		"confirmPassword":    "password123",
		"SelfUseModeEnabled": true,
		"DemoSiteEnabled":    true,
	})

	PostSetup(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	require.NoError(t, platformencoding.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	require.Equal(t, "系统初始化成功", response.Message)

	var root identityschema.User
	require.NoError(t, db.First(&root).Error)
	require.Equal(t, "rootadmin", root.Username)
	require.Equal(t, constant.RoleRootUser, root.Role)

	require.True(t, constant.Setup)
	require.True(t, platformops.IsSelfUseModeEnabled())
	require.True(t, platformops.IsDemoSiteEnabled())
	require.Equal(t, "true", platformconfig.OptionMap["SelfUseModeEnabled"])
	require.Equal(t, "true", platformconfig.OptionMap["DemoSiteEnabled"])

	var setupRecord platformschema.Setup
	require.NoError(t, db.First(&setupRecord).Error)
	require.Equal(t, platformconfig.Version, setupRecord.Version)
	require.NotZero(t, setupRecord.InitializedAt)
}

func TestGetStatusReturnsPublicRuntimeSnapshot(t *testing.T) {
	setupPlatformHTTPTestDB(t)

	originalSystemName := platformconfig.SystemName
	originalNotice := platformconfig.OptionMap["Notice"]
	originalHeaderModules := platformconfig.OptionMap["HeaderNavModules"]
	originalSidebarModulesAdmin := platformconfig.OptionMap["SidebarModulesAdmin"]
	originalApiInfo := platformstore.GetConsoleSetting().ApiInfo
	originalApiInfoEnabled := platformstore.GetConsoleSetting().ApiInfoEnabled
	originalAnnouncements := platformstore.GetConsoleSetting().Announcements
	originalAnnouncementsEnabled := platformstore.GetConsoleSetting().AnnouncementsEnabled
	originalFAQ := platformstore.GetConsoleSetting().FAQ
	originalFAQEnabled := platformstore.GetConsoleSetting().FAQEnabled
	originalAgreement := platformstore.GetLegalSettings().UserAgreement
	originalPrivacy := platformstore.GetLegalSettings().PrivacyPolicy

	platformconfig.SystemName = "CodeGo Test"
	platformconfig.OptionMap["Notice"] = "notice"
	platformconfig.OptionMap["HeaderNavModules"] = `[{"key":"pricing"}]`
	platformconfig.OptionMap["SidebarModulesAdmin"] = `["users"]`
	platformstore.GetConsoleSetting().ApiInfoEnabled = true
	platformstore.GetConsoleSetting().ApiInfo = `[{"title":"API","description":"Docs","route":"/docs","color":"blue"}]`
	platformstore.GetConsoleSetting().AnnouncementsEnabled = true
	platformstore.GetConsoleSetting().Announcements = `[{"content":"hello","publishDate":"2026-07-08T00:00:00Z","type":"default"}]`
	platformstore.GetConsoleSetting().FAQEnabled = true
	platformstore.GetConsoleSetting().FAQ = `[{"question":"Q","answer":"A"}]`
	legalSetting := platformstore.GetLegalSettings()
	legalSetting.UserAgreement = "agreement"
	legalSetting.PrivacyPolicy = "privacy"

	t.Cleanup(func() {
		platformconfig.SystemName = originalSystemName
		platformconfig.OptionMap["Notice"] = originalNotice
		platformconfig.OptionMap["HeaderNavModules"] = originalHeaderModules
		platformconfig.OptionMap["SidebarModulesAdmin"] = originalSidebarModulesAdmin
		platformstore.GetConsoleSetting().ApiInfo = originalApiInfo
		platformstore.GetConsoleSetting().ApiInfoEnabled = originalApiInfoEnabled
		platformstore.GetConsoleSetting().Announcements = originalAnnouncements
		platformstore.GetConsoleSetting().AnnouncementsEnabled = originalAnnouncementsEnabled
		platformstore.GetConsoleSetting().FAQ = originalFAQ
		platformstore.GetConsoleSetting().FAQEnabled = originalFAQEnabled
		legalSetting.UserAgreement = originalAgreement
		legalSetting.PrivacyPolicy = originalPrivacy
	})

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/status", nil)

	GetStatus(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Success bool           `json:"success"`
		Data    map[string]any `json:"data"`
	}
	require.NoError(t, platformencoding.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	require.Equal(t, "CodeGo Test", response.Data["system_name"])
	require.Equal(t, true, response.Data["api_info_enabled"])
	require.Equal(t, true, response.Data["announcements_enabled"])
	require.Equal(t, true, response.Data["faq_enabled"])
	require.Equal(t, true, response.Data["user_agreement_enabled"])
	require.Equal(t, true, response.Data["privacy_policy_enabled"])
}

func TestGetNoticeReturnsConfiguredContent(t *testing.T) {
	setupPlatformHTTPTestDB(t)

	original := platformconfig.OptionMap["Notice"]
	platformconfig.OptionMap["Notice"] = "hello notice"
	t.Cleanup(func() {
		platformconfig.OptionMap["Notice"] = original
	})

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/notice", nil)

	GetNotice(ctx)

	require.JSONEq(t, `{"success":true,"message":"","data":"hello notice"}`, recorder.Body.String())
}

func TestGetUptimeKumaStatusReturnsGroupedMonitorData(t *testing.T) {
	setupPlatformHTTPTestDB(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/status-page/test-group":
			_, _ = w.Write([]byte(`{"publicGroupList":[{"id":1,"name":"Core","monitorList":[{"id":10,"name":"Gateway"}]}]}`))
		case "/api/status-page/heartbeat/test-group":
			_, _ = w.Write([]byte(`{"heartbeatList":{"10":[{"status":1}]},"uptimeList":{"10_24":99.5}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	originalGroups := platformstore.GetConsoleSetting().UptimeKumaGroups
	platformstore.GetConsoleSetting().UptimeKumaGroups = fmt.Sprintf(`[{"categoryName":"Infra","url":"%s","slug":"test-group"}]`, server.URL)
	t.Cleanup(func() {
		platformstore.GetConsoleSetting().UptimeKumaGroups = originalGroups
	})

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/uptime/status", nil)

	GetUptimeKumaStatus(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Success bool `json:"success"`
		Data    []struct {
			CategoryName string `json:"categoryName"`
			Monitors     []struct {
				Name   string  `json:"name"`
				Uptime float64 `json:"uptime"`
				Status int     `json:"status"`
				Group  string  `json:"group"`
			} `json:"monitors"`
		} `json:"data"`
	}
	require.NoError(t, platformencoding.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	require.Len(t, response.Data, 1)
	require.Equal(t, "Infra", response.Data[0].CategoryName)
	require.Len(t, response.Data[0].Monitors, 1)
	require.Equal(t, "Gateway", response.Data[0].Monitors[0].Name)
	require.Equal(t, 99.5, response.Data[0].Monitors[0].Uptime)
	require.Equal(t, 1, response.Data[0].Monitors[0].Status)
	require.Equal(t, "Core", response.Data[0].Monitors[0].Group)
}

func TestTestStatusReturnsHTTPStats(t *testing.T) {
	setupPlatformHTTPTestDB(t)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/status/test", nil)

	TestStatus(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response map[string]any
	require.NoError(t, platformencoding.Unmarshal(recorder.Body.Bytes(), &response))
	require.Equal(t, true, response["success"])
	require.Equal(t, "Server is running", response["message"])
	require.NotNil(t, response["http_stats"])
}

func newPlatformJSONContext(t *testing.T, method string, target string, body any) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = buildPlatformJSONRequest(t, method, target, body)
	return ctx, recorder
}

func buildPlatformJSONRequest(t *testing.T, method string, target string, body any) *http.Request {
	t.Helper()
	payload, err := platformencoding.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest(method, target, strings.NewReader(string(payload)))
	req.Header.Set("Content-Type", "application/json")
	return req
}
