package bootstrap

import (
	"strings"

	"github.com/joho/godotenv"
	"github.com/sh2001sh/CodeGo-Api/i18n"
	auditprojection "github.com/sh2001sh/CodeGo-Api/internal/audit/projection"
	gatewaystore "github.com/sh2001sh/CodeGo-Api/internal/gateway/store"
	identityapp "github.com/sh2001sh/CodeGo-Api/internal/identity/app"
	"github.com/sh2001sh/CodeGo-Api/internal/identity/oauth"
	platformcache "github.com/sh2001sh/CodeGo-Api/internal/platform/cache"
	platformconfig "github.com/sh2001sh/CodeGo-Api/internal/platform/config"
	platformhttpx "github.com/sh2001sh/CodeGo-Api/internal/platform/httpx"
	"github.com/sh2001sh/CodeGo-Api/internal/platform/logger"
	platformobservability "github.com/sh2001sh/CodeGo-Api/internal/platform/observability"
	platformstore "github.com/sh2001sh/CodeGo-Api/internal/platform/store"
	platformtokenx "github.com/sh2001sh/CodeGo-Api/internal/platform/tokenx"
)

func initResources() error {
	if err := godotenv.Load(".env"); err != nil && platformconfig.DebugEnabled {
		platformobservability.SysLog("No .env file found, using default environment variables. If needed, please create a .env file and set the relevant variables.")
	}

	initEnvironment()
	platformcache.ConfigureRedisRuntime(platformcache.RedisRuntimeConfig{
		DebugEnabled:  platformconfig.DebugEnabled,
		SyncFrequency: platformconfig.SyncFrequency,
		Logf:          platformobservability.SysLog,
		FatalLog: func(message string) {
			platformobservability.FatalLog(message)
		},
	})
	logger.SetupLogger()
	gatewaystore.InitRatioSettings()
	platformhttpx.InitHTTPClient()
	platformtokenx.InitTokenEncoders()

	if err := platformstore.InitPrimaryDB(); err != nil {
		platformobservability.FatalLog("failed to initialize database: " + err.Error())
		return err
	}
	if err := auditprojection.EnsureSchema(); err != nil {
		platformobservability.FatalLog("failed to initialize audit projection schema: " + err.Error())
		return err
	}

	platformstore.CheckSetup()

	platformstore.InitOptionMap()
	platformhttpx.CleanupOldCacheFiles()
	loadBootstrapPricing()

	if err := platformstore.InitLogDB(); err != nil {
		return err
	}
	if err := platformcache.InitRedisClient(); err != nil {
		return err
	}

	platformobservability.StartSystemMonitor()
	if err := i18n.Init(); err != nil {
		platformobservability.SysError("failed to initialize i18n: " + err.Error())
	} else {
		platformobservability.SysLog("i18n initialized with languages: " + strings.Join(i18n.SupportedLanguages(), ", "))
	}
	i18n.SetUserLangLoader(identityapp.LoadUserLanguage)

	if err := oauth.LoadCustomProviders(); err != nil {
		platformobservability.SysError("failed to load custom OAuth providers: " + err.Error())
	}

	return nil
}
