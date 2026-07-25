package http

import (
	"embed"
	"fmt"
	platformobservability "github.com/sh2001sh/CodeGo-Api/internal/platform/observability"
	nethttp "net/http"
	"os"
	"strings"

	"github.com/gin-contrib/gzip"
	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
	gatewayhttp "github.com/sh2001sh/CodeGo-Api/internal/gateway/transport/http"
	platformconfig "github.com/sh2001sh/CodeGo-Api/internal/platform/config"
	"github.com/sh2001sh/CodeGo-Api/internal/platform/transport/http/middleware"
)

type ThemeAssets struct {
	DefaultBuildFS   embed.FS
	DefaultIndexPage []byte
}

func registerControlWebRoutes(router *gin.Engine, assets ThemeAssets) {
	frontendBaseURL := os.Getenv("FRONTEND_BASE_URL")
	if platformconfig.IsMasterNode && frontendBaseURL != "" {
		frontendBaseURL = ""
		platformobservability.SysLog("FRONTEND_BASE_URL is ignored on master node")
	}
	if frontendBaseURL == "" {
		registerEmbeddedWebRoutes(router, assets)
		return
	}

	frontendBaseURL = strings.TrimSuffix(frontendBaseURL, "/")
	router.NoRoute(func(c *gin.Context) {
		c.Set(middleware.RouteTagKey, "web")
		c.Redirect(nethttp.StatusMovedPermanently, fmt.Sprintf("%s%s", frontendBaseURL, c.Request.RequestURI))
	})
}

func registerEmbeddedWebRoutes(router *gin.Engine, assets ThemeAssets) {
	defaultFS := embedFolder(assets.DefaultBuildFS, "dist")
	staticHandler := static.Serve("/", defaultFS)

	router.Use(gzip.Gzip(gzip.DefaultCompression))
	router.Use(middleware.GlobalWebRateLimit())
	router.Use(middleware.Cache())

	homeHandler := func(c *gin.Context) {
		serveIndexPage(c, assets.DefaultIndexPage)
	}

	router.GET("/", homeHandler)
	router.HEAD("/", homeHandler)
	router.GET("/index.html", homeHandler)
	router.HEAD("/index.html", homeHandler)
	router.Use(func(c *gin.Context) {
		path := c.Request.URL.Path
		// Application routes are extensionless. Let the SPA handle them even if
		// an outdated build happens to contain a same-named directory.
		lastSegment := path[strings.LastIndex(path, "/")+1:]
		if !strings.Contains(lastSegment, ".") {
			c.Next()
			return
		}
		staticHandler(c)
	})
	router.NoRoute(func(c *gin.Context) {
		c.Set(middleware.RouteTagKey, "web")
		if strings.HasPrefix(c.Request.RequestURI, "/v1") || strings.HasPrefix(c.Request.RequestURI, "/api") || strings.HasPrefix(c.Request.RequestURI, "/assets") {
			gatewayhttp.RelayNotFound(c)
			return
		}
		serveIndexPage(c, assets.DefaultIndexPage)
	})
}

func serveIndexPage(c *gin.Context, indexPage []byte) {
	c.Header("Cache-Control", "no-cache")
	c.Data(nethttp.StatusOK, "text/html; charset=utf-8", indexPage)
}
