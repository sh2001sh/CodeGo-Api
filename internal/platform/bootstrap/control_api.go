package bootstrap

import (
	platformobservability "github.com/sh2001sh/CodeGo-Api/internal/platform/observability"
	"time"

	"github.com/gin-gonic/gin"
	platformhttp "github.com/sh2001sh/CodeGo-Api/internal/platform/transport/http"
	defaultweb "github.com/sh2001sh/CodeGo-Api/web/default"
)

// RunControlAPI boots the control-plane HTTP server.
func RunControlAPI() {
	startTime := time.Now()
	indexPage := buildIndexPage()

	if err := prepareRuntime("control-api"); err != nil {
		return
	}
	defer closeDatabase()

	startControlBackgroundTasks()
	startDiagnostics()

	server := buildHTTPServer(func(server *gin.Engine) {
		platformhttp.RegisterControlRuntimeRoutes(server, defaultweb.ThemeAssets(indexPage))
	})
	port := resolvePort("CONTROL_PORT")
	platformobservability.LogStartupSuccess(startTime, port)

	if err := server.Run(":" + port); err != nil {
		platformobservability.FatalLog("failed to start control API server: " + err.Error())
	}
}
