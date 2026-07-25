package store

import (
	"errors"
	"fmt"
	"github.com/bytedance/gopkg/util/gopool"
	"github.com/sh2001sh/CodeGo-Api/constant"
	identityschema "github.com/sh2001sh/CodeGo-Api/internal/identity/schema"
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
	platformobservability "github.com/sh2001sh/CodeGo-Api/internal/platform/observability"
	platformruntime "github.com/sh2001sh/CodeGo-Api/internal/platform/runtime"
	platformsecurity "github.com/sh2001sh/CodeGo-Api/internal/platform/security"
	"strings"

	// AdjustTokenQuota applies a signed quota delta to one token and mirrors the remain quota cache.
	platformcache "github.com/sh2001sh/CodeGo-Api/internal/platform/cache"
	"gorm.io/gorm"
)

func AdjustTokenQuota(tokenID int, tokenKey string, delta int) error {
	if delta == 0 {
		return nil
	}
	tokenKey = strings.TrimSpace(tokenKey)
	if delta > 0 {
		return decreaseTokenQuota(tokenID, tokenKey, delta)
	}
	return increaseTokenQuota(tokenID, tokenKey, -delta)
}

func increaseTokenQuota(tokenID int, tokenKey string, quota int) error {
	if quota < 0 {
		return errors.New("quota 不能为负数！")
	}
	if platformcache.RedisEnabled {
		gopool.Go(func() {
			if err := cacheAdjustTokenQuota(tokenKey, int64(quota)); err != nil {
				platformobservability.SysLog("failed to increase token quota: " + err.Error())
			}
		})
	}
	return platformdb.DB.Model(&identityschema.Token{}).Where("id = ?", tokenID).Updates(map[string]any{
		"remain_quota":  gorm.Expr("remain_quota + ?", quota),
		"used_quota":    gorm.Expr("used_quota - ?", quota),
		"accessed_time": platformruntime.GetTimestamp(),
	}).Error
}

func decreaseTokenQuota(tokenID int, tokenKey string, quota int) error {
	if quota < 0 {
		return errors.New("quota 不能为负数！")
	}
	if platformcache.RedisEnabled {
		gopool.Go(func() {
			if err := cacheAdjustTokenQuota(tokenKey, -int64(quota)); err != nil {
				platformobservability.SysLog("failed to decrease token quota: " + err.Error())
			}
		})
	}
	return platformdb.DB.Model(&identityschema.Token{}).Where("id = ?", tokenID).Updates(map[string]any{
		"remain_quota":  gorm.Expr("remain_quota - ?", quota),
		"used_quota":    gorm.Expr("used_quota + ?", quota),
		"accessed_time": platformruntime.GetTimestamp(),
	}).Error
}

func cacheAdjustTokenQuota(key string, delta int64) error {
	if !platformcache.RedisReady() {
		return nil
	}
	return platformcache.RedisHIncrBy(fmt.Sprintf("token:%s", platformsecurity.GenerateHMAC(key)), constant.TokenFiledRemainQuota, delta)
}
