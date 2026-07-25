package http

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestRuntimeRouteIsolation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	control := gin.New()
	RegisterControlRuntimeRoutes(control, ThemeAssets{})

	gateway := gin.New()
	RegisterGatewayRuntimeRoutes(gateway)

	controlRoutes := registeredRouteSet(control)
	gatewayRoutes := registeredRouteSet(gateway)

	assert.NotContains(t, controlRoutes, "POST /v1/chat/completions")
	assert.Contains(t, controlRoutes, "GET /api/status")
	assert.Contains(t, gatewayRoutes, "POST /v1/chat/completions")
	assert.NotContains(t, gatewayRoutes, "GET /api/status")
	assert.NotContains(t, gatewayRoutes, "GET /")
}

func registeredRouteSet(engine *gin.Engine) map[string]struct{} {
	routes := make(map[string]struct{})
	for _, route := range engine.Routes() {
		routes[route.Method+" "+route.Path] = struct{}{}
	}
	return routes
}
