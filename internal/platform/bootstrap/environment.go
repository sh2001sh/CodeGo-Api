package bootstrap

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/sh2001sh/CodeGo-Api/constant"
	platformconfig "github.com/sh2001sh/CodeGo-Api/internal/platform/config"
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
	platformtext "github.com/sh2001sh/CodeGo-Api/internal/platform/textx"
)

func initEnvironment() {
	flag.Parse()

	if envVersion := os.Getenv("VERSION"); envVersion != "" {
		platformconfig.Version = envVersion
	}

	if *platformconfig.PrintVersion {
		fmt.Println(platformconfig.Version)
		os.Exit(0)
	}

	if *platformconfig.PrintHelp {
		printHelp()
		os.Exit(0)
	}

	if os.Getenv("SESSION_SECRET") != "" {
		ss := os.Getenv("SESSION_SECRET")
		if ss == "random_string" {
			log.Println("WARNING: SESSION_SECRET is set to the default value 'random_string', please change it to a random string.")
			log.Println("警告：SESSION_SECRET被设置为默认值'random_string'，请修改为随机字符串。")
			log.Fatal("Please set SESSION_SECRET to a random string.")
		}
		platformconfig.SessionSecret = ss
	}
	if os.Getenv("CRYPTO_SECRET") != "" {
		platformconfig.CryptoSecret = os.Getenv("CRYPTO_SECRET")
	} else {
		platformconfig.CryptoSecret = platformconfig.SessionSecret
	}
	platformconfig.InitSessionCookieConfig()
	if os.Getenv("SQLITE_PATH") != "" {
		platformdb.SQLitePath = os.Getenv("SQLITE_PATH")
	}
	if *platformconfig.LogDir != "" {
		var err error
		*platformconfig.LogDir, err = filepath.Abs(*platformconfig.LogDir)
		if err != nil {
			log.Fatal(err)
		}
		if _, err := os.Stat(*platformconfig.LogDir); os.IsNotExist(err) {
			if err = os.Mkdir(*platformconfig.LogDir, 0777); err != nil {
				log.Fatal(err)
			}
		}
	}

	platformconfig.DebugEnabled = os.Getenv("DEBUG") == "true"
	platformtext.SetDebugEnabled(platformconfig.DebugEnabled)
	platformconfig.MemoryCacheEnabled = os.Getenv("MEMORY_CACHE_ENABLED") == "true"
	platformconfig.IsMasterNode = os.Getenv("NODE_TYPE") != "slave"
	platformconfig.NodeName = os.Getenv("NODE_NAME")
	platformconfig.TLSInsecureSkipVerify = platformconfig.GetEnvOrDefaultBool("TLS_INSECURE_SKIP_VERIFY", false)
	if platformconfig.TLSInsecureSkipVerify {
		if tr, ok := http.DefaultTransport.(*http.Transport); ok && tr != nil {
			if tr.TLSClientConfig != nil {
				tr.TLSClientConfig.InsecureSkipVerify = true
			} else {
				tr.TLSClientConfig = platformconfig.InsecureTLSConfig
			}
		}
	}

	requestInterval, _ := strconv.Atoi(os.Getenv("POLLING_INTERVAL"))
	platformconfig.RequestInterval = time.Duration(requestInterval) * time.Second

	platformconfig.SyncFrequency = platformconfig.GetEnvOrDefaultInt("SYNC_FREQUENCY", 60)
	platformconfig.BatchUpdateInterval = platformconfig.GetEnvOrDefaultInt("BATCH_UPDATE_INTERVAL", 5)
	platformconfig.RelayTimeout = platformconfig.GetEnvOrDefaultInt("RELAY_TIMEOUT", 0)
	platformconfig.RelayResponseHeaderTimeout = platformconfig.GetEnvOrDefaultInt("RELAY_RESPONSE_HEADER_TIMEOUT", 45)
	platformconfig.RelayMaxIdleConns = platformconfig.GetEnvOrDefaultInt("RELAY_MAX_IDLE_CONNS", 500)
	platformconfig.RelayMaxIdleConnsPerHost = platformconfig.GetEnvOrDefaultInt("RELAY_MAX_IDLE_CONNS_PER_HOST", 100)
	platformconfig.GeminiSafetySetting = platformconfig.GetEnvOrDefaultString("GEMINI_SAFETY_SETTING", "BLOCK_NONE")
	platformconfig.CohereSafetySetting = platformconfig.GetEnvOrDefaultString("COHERE_SAFETY_SETTING", "NONE")

	platformconfig.GlobalApiRateLimitEnable = platformconfig.GetEnvOrDefaultBool("GLOBAL_API_RATE_LIMIT_ENABLE", true)
	platformconfig.GlobalApiRateLimitNum = platformconfig.GetEnvOrDefaultInt("GLOBAL_API_RATE_LIMIT", 180)
	platformconfig.GlobalApiRateLimitDuration = int64(platformconfig.GetEnvOrDefaultInt("GLOBAL_API_RATE_LIMIT_DURATION", 180))

	platformconfig.GlobalWebRateLimitEnable = platformconfig.GetEnvOrDefaultBool("GLOBAL_WEB_RATE_LIMIT_ENABLE", true)
	platformconfig.GlobalWebRateLimitNum = platformconfig.GetEnvOrDefaultInt("GLOBAL_WEB_RATE_LIMIT", 60)
	platformconfig.GlobalWebRateLimitDuration = int64(platformconfig.GetEnvOrDefaultInt("GLOBAL_WEB_RATE_LIMIT_DURATION", 180))

	platformconfig.CriticalRateLimitEnable = platformconfig.GetEnvOrDefaultBool("CRITICAL_RATE_LIMIT_ENABLE", true)
	platformconfig.CriticalRateLimitNum = platformconfig.GetEnvOrDefaultInt("CRITICAL_RATE_LIMIT", 20)
	platformconfig.CriticalRateLimitDuration = int64(platformconfig.GetEnvOrDefaultInt("CRITICAL_RATE_LIMIT_DURATION", 20*60))

	platformconfig.SearchRateLimitEnable = platformconfig.GetEnvOrDefaultBool("SEARCH_RATE_LIMIT_ENABLE", true)
	platformconfig.SearchRateLimitNum = platformconfig.GetEnvOrDefaultInt("SEARCH_RATE_LIMIT", 10)
	platformconfig.SearchRateLimitDuration = int64(platformconfig.GetEnvOrDefaultInt("SEARCH_RATE_LIMIT_DURATION", 60))

	trustedProxiesStr := platformconfig.GetEnvOrDefaultString("TRUSTED_PROXIES", "")
	if trustedProxiesStr == "" {
		platformconfig.TrustedProxies = []string{
			"127.0.0.1",
			"::1",
			"10.0.0.0/8",
			"172.16.0.0/12",
			"192.168.0.0/16",
			"fc00::/7",
		}
	} else {
		var trustedProxies []string
		for _, proxy := range strings.Split(trustedProxiesStr, ",") {
			trimmedProxy := strings.TrimSpace(proxy)
			if trimmedProxy != "" {
				trustedProxies = append(trustedProxies, trimmedProxy)
			}
		}
		platformconfig.TrustedProxies = trustedProxies
	}

	initConstantEnv()
}

func printHelp() {
	fmt.Println("codego-api " + platformconfig.Version + " - API unified management platform")
	fmt.Println("Project Repository: https://github.com/sh2001sh/CodeGo-Api")
	fmt.Println("Usage: codego-api [--port <port>] [--log-dir <log directory>] [--version] [--help]")
}

func initConstantEnv() {
	constant.StreamingTimeout = platformconfig.GetEnvOrDefaultInt("STREAMING_TIMEOUT", 300)
	constant.DifyDebug = platformconfig.GetEnvOrDefaultBool("DIFY_DEBUG", true)
	constant.MaxFileDownloadMB = platformconfig.GetEnvOrDefaultInt("MAX_FILE_DOWNLOAD_MB", 64)
	constant.StreamScannerMaxBufferMB = platformconfig.GetEnvOrDefaultInt("STREAM_SCANNER_MAX_BUFFER_MB", 128)
	constant.MaxRequestBodyMB = platformconfig.GetEnvOrDefaultInt("MAX_REQUEST_BODY_MB", 128)
	constant.AnonymousRequestBodyLimitKB = platformconfig.GetEnvOrDefaultInt("ANONYMOUS_REQUEST_BODY_LIMIT_KB", 512)
	constant.ForceStreamOption = platformconfig.GetEnvOrDefaultBool("FORCE_STREAM_OPTION", true)
	constant.CountToken = platformconfig.GetEnvOrDefaultBool("CountToken", true)
	constant.GetMediaToken = platformconfig.GetEnvOrDefaultBool("GET_MEDIA_TOKEN", true)
	constant.GetMediaTokenNotStream = platformconfig.GetEnvOrDefaultBool("GET_MEDIA_TOKEN_NOT_STREAM", false)
	constant.UpdateTask = platformconfig.GetEnvOrDefaultBool("UPDATE_TASK", true)
	constant.AzureDefaultAPIVersion = platformconfig.GetEnvOrDefaultString("AZURE_DEFAULT_API_VERSION", "2025-04-01-preview")
	constant.NotifyLimitCount = platformconfig.GetEnvOrDefaultInt("NOTIFY_LIMIT_COUNT", 2)
	constant.NotificationLimitDurationMinute = platformconfig.GetEnvOrDefaultInt("NOTIFICATION_LIMIT_DURATION_MINUTE", 10)
	constant.GenerateDefaultToken = platformconfig.GetEnvOrDefaultBool("GENERATE_DEFAULT_TOKEN", false)
	constant.ErrorLogEnabled = platformconfig.GetEnvOrDefaultBool("ERROR_LOG_ENABLED", false)
	constant.TaskQueryLimit = platformconfig.GetEnvOrDefaultInt("TASK_QUERY_LIMIT", 1000)
	constant.TaskTimeoutMinutes = platformconfig.GetEnvOrDefaultInt("TASK_TIMEOUT_MINUTES", 1440)

	taskPricePatchStr := platformconfig.GetEnvOrDefaultString("TASK_PRICE_PATCH", "")
	if taskPricePatchStr != "" {
		var taskPricePatches []string
		pricePatches := strings.Split(taskPricePatchStr, ",")
		for _, patch := range pricePatches {
			trimmedPatch := strings.TrimSpace(patch)
			if trimmedPatch != "" {
				taskPricePatches = append(taskPricePatches, trimmedPatch)
			}
		}
		constant.TaskPricePatches = taskPricePatches
	}

	trustedDomainsStr := platformconfig.GetEnvOrDefaultString("TRUSTED_REDIRECT_DOMAINS", "")
	var trustedDomains []string
	for _, domain := range strings.Split(trustedDomainsStr, ",") {
		trimmedDomain := strings.TrimSpace(domain)
		if trimmedDomain != "" {
			trustedDomains = append(trustedDomains, strings.ToLower(trimmedDomain))
		}
	}
	constant.TrustedRedirectDomains = trustedDomains
}
