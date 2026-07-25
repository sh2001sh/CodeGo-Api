package store

import (
	"github.com/bytedance/gopkg/util/gopool"
	identityschema "github.com/sh2001sh/CodeGo-Api/internal/identity/schema"
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
	platformobservability "github.com/sh2001sh/CodeGo-Api/internal/platform/observability"
)

// CreateToken persists a newly created token.
func CreateToken(token *identityschema.Token) error {
	if token == nil {
		return nil
	}
	return platformdb.DB.Create(token).Error
}

// UpdateToken persists editable token fields and refreshes the token cache.
func UpdateToken(token *identityschema.Token) (err error) {
	if token == nil {
		return nil
	}
	defer refreshTokenCacheAsync(token, &err)

	return platformdb.DB.Model(token).
		Select(
			"name",
			"status",
			"expired_time",
			"remain_quota",
			"unlimited_quota",
			"model_limits_enabled",
			"model_limits",
			"allow_ips",
			"group",
			"cross_group_retry",
		).
		Updates(token).Error
}

func updateTokenStatus(token *identityschema.Token) (err error) {
	if token == nil {
		return nil
	}
	defer refreshTokenCacheAsync(token, &err)

	return platformdb.DB.Model(token).Select("accessed_time", "status").Updates(token).Error
}

func deleteTokenRecord(token *identityschema.Token) (err error) {
	if token == nil {
		return nil
	}
	defer deleteTokenCacheAsync(token, &err)

	return platformdb.DB.Delete(token).Error
}

func refreshTokenCacheAsync(token *identityschema.Token, errRef *error) {
	if token == nil || errRef == nil || *errRef != nil || !shouldUpdateRedis(true, *errRef) {
		return
	}
	tokenCopy := *token
	gopool.Go(func() {
		if err := cacheSetToken(tokenCopy); err != nil {
			platformobservability.SysLog("failed to update token cache: " + err.Error())
		}
	})
}

func deleteTokenCacheAsync(token *identityschema.Token, errRef *error) {
	if token == nil || errRef == nil || *errRef != nil || !shouldUpdateRedis(true, *errRef) {
		return
	}
	tokenKey := token.Key
	gopool.Go(func() {
		if err := cacheDeleteToken(tokenKey); err != nil {
			platformobservability.SysLog("failed to delete token cache: " + err.Error())
		}
	})
}
