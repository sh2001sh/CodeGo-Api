package cache

import (
	"fmt"
	"time"
)

func UserCacheKey(userID int) string {
	return fmt.Sprintf("user:%d", userID)
}

func DeleteUserCache(userID int) error {
	if !RedisReady() {
		return nil
	}
	return RedisDelKey(UserCacheKey(userID))
}

func WriteUserCache(userID int, snapshot any) error {
	if !RedisReady() {
		return nil
	}
	return RedisHSetObj(
		UserCacheKey(userID),
		snapshot,
		time.Duration(RedisKeyCacheSeconds())*time.Second,
	)
}

func ReadUserCache(userID int, dst any) error {
	if !RedisReady() {
		return fmt.Errorf("redis is not ready")
	}
	return RedisHGetObj(UserCacheKey(userID), dst)
}

func AdjustUserQuotaCache(userID int, delta int64) error {
	if !RedisReady() {
		return nil
	}
	return RedisHIncrBy(UserCacheKey(userID), "Quota", delta)
}

func SetUserCacheField(userID int, field string, value any) error {
	if !RedisReady() {
		return nil
	}
	return RedisHSetField(UserCacheKey(userID), field, value)
}
