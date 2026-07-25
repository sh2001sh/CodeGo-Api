package middleware

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	platformcache "github.com/sh2001sh/CodeGo-Api/internal/platform/cache"
	platformconfig "github.com/sh2001sh/CodeGo-Api/internal/platform/config"
	platformratelimit "github.com/sh2001sh/CodeGo-Api/internal/platform/ratelimit"
)

var timeFormat = "2006-01-02T15:04:05.000Z"

var inMemoryRateLimiter InMemoryRateLimiter

var defNext = func(c *gin.Context) {
	c.Next()
}

func redisRateLimiter(c *gin.Context, maxRequestNum int, duration int64, mark string) {
	ctx := context.Background()
	rdb := platformcache.RDB
	key := "rateLimit:" + mark + c.ClientIP()
	listLength, err := rdb.LLen(ctx, key).Result()
	if err != nil {
		fmt.Println(err.Error())
		c.Status(http.StatusInternalServerError)
		c.Abort()
		return
	}
	if listLength < int64(maxRequestNum) {
		rdb.LPush(ctx, key, time.Now().Format(timeFormat))
		rdb.Expire(ctx, key, platformconfig.RateLimitKeyExpirationDuration)
	} else {
		oldTimeStr, _ := rdb.LIndex(ctx, key, -1).Result()
		oldTime, err := time.Parse(timeFormat, oldTimeStr)
		if err != nil {
			fmt.Println(err)
			c.Status(http.StatusInternalServerError)
			c.Abort()
			return
		}
		nowTimeStr := time.Now().Format(timeFormat)
		nowTime, err := time.Parse(timeFormat, nowTimeStr)
		if err != nil {
			fmt.Println(err)
			c.Status(http.StatusInternalServerError)
			c.Abort()
			return
		}
		// time.Since will return negative number!
		// See: https://stackoverflow.com/questions/50970900/why-is-time-since-returning-negative-durations-on-windows
		if int64(nowTime.Sub(oldTime).Seconds()) < duration {
			rdb.Expire(ctx, key, platformconfig.RateLimitKeyExpirationDuration)
			c.Status(http.StatusTooManyRequests)
			c.Abort()
			return
		} else {
			rdb.LPush(ctx, key, time.Now().Format(timeFormat))
			rdb.LTrim(ctx, key, 0, int64(maxRequestNum-1))
			rdb.Expire(ctx, key, platformconfig.RateLimitKeyExpirationDuration)
		}
	}
}

func memoryRateLimiter(c *gin.Context, maxRequestNum int, duration int64, mark string) {
	key := mark + c.ClientIP()
	if !inMemoryRateLimiter.Request(key, maxRequestNum, duration) {
		c.Status(http.StatusTooManyRequests)
		c.Abort()
		return
	}
}

func rateLimitFactory(maxRequestNum int, duration int64, mark string) func(c *gin.Context) {
	if platformcache.RedisEnabled {
		return func(c *gin.Context) {
			redisRateLimiter(c, maxRequestNum, duration, mark)
		}
	} else {
		// It's safe to call multi times.
		inMemoryRateLimiter.Init(platformconfig.RateLimitKeyExpirationDuration)
		return func(c *gin.Context) {
			memoryRateLimiter(c, maxRequestNum, duration, mark)
		}
	}
}

func GlobalWebRateLimit() func(c *gin.Context) {
	if platformconfig.GlobalWebRateLimitEnable {
		return rateLimitFactory(platformconfig.GlobalWebRateLimitNum, platformconfig.GlobalWebRateLimitDuration, "GW")
	}
	return defNext
}

func GlobalAPIRateLimit() func(c *gin.Context) {
	if platformconfig.GlobalApiRateLimitEnable {
		return rateLimitFactory(platformconfig.GlobalApiRateLimitNum, platformconfig.GlobalApiRateLimitDuration, "GA")
	}
	return defNext
}

// GlobalAuthenticatedAPIRateLimit isolates authenticated traffic by user so
// one noisy client IP cannot consume another user's quota.
func GlobalAuthenticatedAPIRateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !enforceGlobalAuthenticatedAPIRateLimit(c) {
			return
		}
		c.Next()
	}
}

func enforceGlobalAuthenticatedAPIRateLimit(c *gin.Context) bool {
	if !platformconfig.GlobalApiRateLimitEnable {
		return true
	}

	key := globalAuthenticatedRateLimitKey(c)
	if key == "" {
		return true
	}
	if platformcache.RedisEnabled {
		allowed, err := platformratelimit.NewRedisLimiter(c.Request.Context(), platformcache.RDB).Allow(
			c.Request.Context(),
			"rateLimit:GA:"+key,
			platformratelimit.WithCapacity(int64(platformconfig.GlobalApiRateLimitNum)),
			platformratelimit.WithRate(globalAPIRatePerSecond()),
			platformratelimit.WithRequested(1),
		)
		if err != nil {
			c.Status(http.StatusInternalServerError)
			c.Abort()
			return false
		}
		if !allowed {
			c.Status(http.StatusTooManyRequests)
			c.Abort()
			return false
		}
		return true
	}

	if !inMemoryRateLimiter.Request("GA:"+key, platformconfig.GlobalApiRateLimitNum, platformconfig.GlobalApiRateLimitDuration) {
		c.Status(http.StatusTooManyRequests)
		c.Abort()
		return false
	}
	return true
}

func globalAPIRatePerSecond() float64 {
	if platformconfig.GlobalApiRateLimitDuration <= 0 {
		return 1
	}
	return float64(platformconfig.GlobalApiRateLimitNum) / float64(platformconfig.GlobalApiRateLimitDuration)
}

func globalAuthenticatedRateLimitKey(c *gin.Context) string {
	if userID := c.GetInt("id"); userID > 0 {
		return fmt.Sprintf("user:%d", userID)
	}
	return ""
}

func CriticalRateLimit() func(c *gin.Context) {
	if platformconfig.CriticalRateLimitEnable {
		return rateLimitFactory(platformconfig.CriticalRateLimitNum, platformconfig.CriticalRateLimitDuration, "CT")
	}
	return defNext
}

func DownloadRateLimit() func(c *gin.Context) {
	return rateLimitFactory(platformconfig.DownloadRateLimitNum, platformconfig.DownloadRateLimitDuration, "DW")
}

func UploadRateLimit() func(c *gin.Context) {
	return rateLimitFactory(platformconfig.UploadRateLimitNum, platformconfig.UploadRateLimitDuration, "UP")
}

// userRateLimitFactory creates a rate limiter keyed by authenticated user ID
// instead of client IP, making it resistant to proxy rotation attacks.
// Must be used AFTER authentication middleware (UserAuth).
func userRateLimitFactory(maxRequestNum int, duration int64, mark string) func(c *gin.Context) {
	if platformcache.RedisEnabled {
		return func(c *gin.Context) {
			userId := c.GetInt("id")
			if userId == 0 {
				c.Status(http.StatusUnauthorized)
				c.Abort()
				return
			}
			key := fmt.Sprintf("rateLimit:%s:user:%d", mark, userId)
			userRedisRateLimiter(c, maxRequestNum, duration, key)
		}
	}
	// It's safe to call multi times.
	inMemoryRateLimiter.Init(platformconfig.RateLimitKeyExpirationDuration)
	return func(c *gin.Context) {
		userId := c.GetInt("id")
		if userId == 0 {
			c.Status(http.StatusUnauthorized)
			c.Abort()
			return
		}
		key := fmt.Sprintf("%s:user:%d", mark, userId)
		if !inMemoryRateLimiter.Request(key, maxRequestNum, duration) {
			c.Status(http.StatusTooManyRequests)
			c.Abort()
			return
		}
	}
}

// userRedisRateLimiter is like redisRateLimiter but accepts a pre-built key
// (to support user-ID-based keys).
func userRedisRateLimiter(c *gin.Context, maxRequestNum int, duration int64, key string) {
	ctx := context.Background()
	rdb := platformcache.RDB
	listLength, err := rdb.LLen(ctx, key).Result()
	if err != nil {
		fmt.Println(err.Error())
		c.Status(http.StatusInternalServerError)
		c.Abort()
		return
	}
	if listLength < int64(maxRequestNum) {
		rdb.LPush(ctx, key, time.Now().Format(timeFormat))
		rdb.Expire(ctx, key, platformconfig.RateLimitKeyExpirationDuration)
	} else {
		oldTimeStr, _ := rdb.LIndex(ctx, key, -1).Result()
		oldTime, err := time.Parse(timeFormat, oldTimeStr)
		if err != nil {
			fmt.Println(err)
			c.Status(http.StatusInternalServerError)
			c.Abort()
			return
		}
		nowTimeStr := time.Now().Format(timeFormat)
		nowTime, err := time.Parse(timeFormat, nowTimeStr)
		if err != nil {
			fmt.Println(err)
			c.Status(http.StatusInternalServerError)
			c.Abort()
			return
		}
		if int64(nowTime.Sub(oldTime).Seconds()) < duration {
			rdb.Expire(ctx, key, platformconfig.RateLimitKeyExpirationDuration)
			c.Status(http.StatusTooManyRequests)
			c.Abort()
			return
		} else {
			rdb.LPush(ctx, key, time.Now().Format(timeFormat))
			rdb.LTrim(ctx, key, 0, int64(maxRequestNum-1))
			rdb.Expire(ctx, key, platformconfig.RateLimitKeyExpirationDuration)
		}
	}
}

// SearchRateLimit returns a per-user rate limiter for search endpoints.
// Configurable via SEARCH_RATE_LIMIT_ENABLE / SEARCH_RATE_LIMIT / SEARCH_RATE_LIMIT_DURATION.
func SearchRateLimit() func(c *gin.Context) {
	if !platformconfig.SearchRateLimitEnable {
		return defNext
	}
	return userRateLimitFactory(platformconfig.SearchRateLimitNum, platformconfig.SearchRateLimitDuration, "SR")
}
