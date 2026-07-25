package bootstrap

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/bytedance/gopkg/util/gopool"
	"github.com/gin-contrib/sessions"
	sessionredis "github.com/gin-contrib/sessions/redis"
	"github.com/gin-gonic/gin"
	redigo "github.com/gomodule/redigo/redis"
	auditprojection "github.com/sh2001sh/CodeGo-Api/internal/audit/projection"
	gatewaystore "github.com/sh2001sh/CodeGo-Api/internal/gateway/store"
	platformcache "github.com/sh2001sh/CodeGo-Api/internal/platform/cache"
	platformconfig "github.com/sh2001sh/CodeGo-Api/internal/platform/config"
	platformobservability "github.com/sh2001sh/CodeGo-Api/internal/platform/observability"
	platformstore "github.com/sh2001sh/CodeGo-Api/internal/platform/store"
	"github.com/sh2001sh/CodeGo-Api/internal/platform/transport/http/middleware"

	_ "net/http/pprof"
)

type httpRouteRegistrar func(*gin.Engine)

func prepareRuntime(component string) error {
	if err := initResources(); err != nil {
		platformobservability.FatalLog("failed to initialize resources: " + err.Error())
		return err
	}
	logStartupMode(component)
	initCaches()
	return nil
}

func logStartupMode(component string) {
	platformobservability.SysLog(component + " " + platformconfig.Version + " started")
	if os.Getenv("GIN_MODE") != "debug" {
		gin.SetMode(gin.ReleaseMode)
	}
	if platformconfig.DebugEnabled {
		platformobservability.SysLog("running in debug mode")
	}
}

func closeDatabase() {
	if err := platformstore.CloseDatabases(); err != nil {
		platformobservability.FatalLog("failed to close database: " + err.Error())
	}
}

func initCaches() {
	if platformcache.RedisEnabled {
		platformconfig.MemoryCacheEnabled = true
	}
	if !platformconfig.MemoryCacheEnabled {
		return
	}

	platformobservability.SysLog("memory cache enabled")
	platformobservability.SysLog(fmt.Sprintf("sync frequency: %d seconds", platformconfig.SyncFrequency))
	func() {
		defer func() {
			if r := recover(); r != nil {
				platformobservability.SysLog(fmt.Sprintf("InitChannelCache panic: %v, retrying once", r))
				if _, _, fixErr := gatewaystore.RebuildChannelAbilities(); fixErr != nil {
					platformobservability.FatalLog(fmt.Sprintf("InitChannelCache failed: %s", fixErr.Error()))
				}
			}
		}()
		gatewaystore.InitChannelCache()
	}()
	go gatewaystore.SyncChannelCache(platformconfig.SyncFrequency)
}

func startOptionSyncLoop() {
	go platformstore.SyncOptions(platformconfig.SyncFrequency)
}

func startDiagnostics() {
	if os.Getenv("ENABLE_PPROF") == "true" {
		gopool.Go(func() {
			log.Println(http.ListenAndServe("0.0.0.0:8005", nil))
		})
		platformobservability.StartPprofCPUMonitor()
		platformobservability.SysLog("pprof enabled")
	}

	if err := platformobservability.StartPyroscope(); err != nil {
		platformobservability.SysError(fmt.Sprintf("start pyroscope error : %v", err))
	}

	auditprojection.Init()
}

func buildHTTPServer(registerRoutes httpRouteRegistrar) *gin.Engine {
	server := gin.New()
	middleware.ConfigureTrustedProxies(server)
	server.Use(gin.CustomRecovery(func(c *gin.Context, err any) {
		platformobservability.SysLog(fmt.Sprintf("panic detected: %v", err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"message": fmt.Sprintf("Panic detected, error: %v. Please submit an issue here: https://github.com/sh2001sh/CodeGo-Api", err),
				"type":    "codego_api_panic",
			},
		})
	}))
	server.Use(middleware.RequestId())
	server.Use(middleware.PoweredBy())
	server.Use(middleware.I18n())
	middleware.SetUpLogger(server)
	server.Use(sessions.Sessions("session", buildSessionStore()))
	registerRoutes(server)
	return server
}

func buildSessionStore() sessions.Store {
	redisURL, err := url.Parse(os.Getenv("REDIS_CONN_STRING"))
	if err != nil || redisURL.Host == "" || !platformcache.RedisReady() {
		log.Fatal("Redis-backed sessions require a reachable REDIS_CONN_STRING")
	}
	store, err := newRedisSessionStore(redisURL)
	if err != nil {
		log.Fatal("failed to initialize Redis session store: " + err.Error())
	}
	if err := sessionredis.SetKeyPrefix(store, "codego:session:"); err != nil {
		log.Fatal("failed to configure Redis session store: " + err.Error())
	}
	store.Options(sessions.Options{
		Path:     "/",
		MaxAge:   2592000,
		HttpOnly: true,
		Secure:   platformconfig.SessionCookieSecure(),
		SameSite: http.SameSiteStrictMode,
	})
	return store
}

func newRedisSessionStore(redisURL *url.URL) (sessionredis.Store, error) {
	if redisURL.Scheme != "redis" && redisURL.Scheme != "rediss" {
		return nil, fmt.Errorf("unsupported Redis URL scheme %q", redisURL.Scheme)
	}
	database, err := strconv.Atoi(redisURL.Query().Get("db"))
	if err != nil && redisURL.Query().Get("db") != "" {
		return nil, fmt.Errorf("invalid Redis database: %w", err)
	}
	if redisURL.Path != "" && redisURL.Path != "/" {
		database, err = strconv.Atoi(redisURL.Path[1:])
		if err != nil {
			return nil, fmt.Errorf("invalid Redis database: %w", err)
		}
	}
	username := redisURL.User.Username()
	password, _ := redisURL.User.Password()
	pool := &redigo.Pool{
		MaxIdle:     10,
		MaxActive:   10,
		Wait:        true,
		IdleTimeout: time.Minute,
		Dial: func() (redigo.Conn, error) {
			return dialRedisSessionConnection(redisURL, username, password, database)
		},
		TestOnBorrow: func(connection redigo.Conn, _ time.Time) error {
			_, err := connection.Do("PING")
			return err
		},
	}
	connection := pool.Get()
	defer connection.Close()
	if err := connection.Err(); err != nil {
		pool.Close()
		return nil, fmt.Errorf("connect Redis session store: %w", err)
	}
	if _, err := connection.Do("PING"); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping Redis session store: %w", err)
	}
	store, err := sessionredis.NewStoreWithPool(pool, []byte(platformconfig.SessionSecret))
	if err != nil {
		pool.Close()
		return nil, err
	}
	return store, nil
}

func dialRedisSessionConnection(redisURL *url.URL, username, password string, database int) (redigo.Conn, error) {
	options := make([]redigo.DialOption, 0, 2)
	if redisURL.Scheme == "rediss" {
		options = append(options,
			redigo.DialUseTLS(true),
			redigo.DialTLSConfig(&tls.Config{MinVersion: tls.VersionTLS12, ServerName: redisURL.Hostname()}),
		)
	}
	connection, err := redigo.Dial("tcp", redisURL.Host, options...)
	if err != nil {
		return nil, err
	}
	if err := authorizeRedisSessionConnection(connection, username, password, database); err != nil {
		connection.Close()
		return nil, err
	}
	return connection, nil
}

func authorizeRedisSessionConnection(connection redigo.Conn, username, password string, database int) error {
	var err error
	if username != "" {
		_, err = connection.Do("AUTH", username, password)
	} else if password != "" {
		_, err = connection.Do("AUTH", password)
	}
	if err != nil {
		return fmt.Errorf("authenticate Redis session store: %w", err)
	}
	if database == 0 {
		return nil
	}
	if _, err := connection.Do("SELECT", database); err != nil {
		return fmt.Errorf("select Redis database: %w", err)
	}
	return nil
}

func resolvePort(primaryEnv string) string {
	port := os.Getenv(primaryEnv)
	if port == "" {
		port = os.Getenv("PORT")
	}
	if port == "" {
		port = strconv.Itoa(*platformconfig.Port)
	}
	return port
}
