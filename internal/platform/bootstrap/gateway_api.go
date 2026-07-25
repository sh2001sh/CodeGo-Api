package bootstrap

import (
	platformobservability "github.com/sh2001sh/CodeGo-Api/internal/platform/observability"
	"time"

	platformhttp "github.com/sh2001sh/CodeGo-Api/internal/platform/transport/http"
)

func RunGatewayAPI() {
	startTime := time.Now()

	if err := prepareRuntime("gateway-api"); err != nil {
		return
	}
	defer closeDatabase()

	if err := startGatewayBackgroundTasks(); err != nil {
		platformobservability.FatalLog("failed to wire gateway runtime: " + err.Error())
		return
	}
	startDiagnostics()

	server := buildHTTPServer(platformhttp.RegisterGatewayRuntimeRoutes)
	port := resolvePort("GATEWAY_PORT")
	platformobservability.LogStartupSuccess(startTime, port)

	if err := server.Run(":" + port); err != nil {
		platformobservability.FatalLog("failed to start gateway API server: " + err.Error())
	}
}

func startGatewayBackgroundTasks() error {
	startOptionSyncLoop()
	return applyRuntimeWiring("gateway-api")
}
