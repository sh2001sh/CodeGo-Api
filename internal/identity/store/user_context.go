package store

import (
	"database/sql"
	"errors"
	"github.com/bytedance/gopkg/util/gopool"
	"github.com/gin-gonic/gin"
	"github.com/sh2001sh/CodeGo-Api/constant"
	"github.com/sh2001sh/CodeGo-Api/dto"
	identitydomain "github.com/sh2001sh/CodeGo-Api/internal/identity/domain"
	identityschema "github.com/sh2001sh/CodeGo-Api/internal/identity/schema"
	platformcache "github.com/sh2001sh/CodeGo-Api/internal/platform/cache"
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
	platformobservability "github.com/sh2001sh/CodeGo-Api/internal/platform/observability"
	"gorm.io/gorm"
)

func LoadUserByID(userID int, selectAll bool) (*identityschema.User, error) {
	if userID == 0 {
		return nil, errors.New("id 为空！")
	}
	user := identityschema.User{Id: userID}
	var err error
	if selectAll {
		err = platformdb.DB.First(&user, "id = ?", userID).Error
	} else {
		err = platformdb.DB.Omit("password").First(&user, "id = ?", userID).Error
	}
	return &user, err
}

func LoadUserCacheSnapshot(userID int) (*identityschema.UserBase, error) {
	var user *identityschema.User
	var userCache identityschema.UserBase

	if platformcache.RedisReady() {
		if err := platformcache.ReadUserCache(userID, &userCache); err == nil {
			return &userCache, nil
		}
	}

	user, err := LoadUserByID(userID, false)
	if err != nil {
		return nil, err
	}
	if user != nil && platformcache.RedisReady() {
		userCopy := *user
		gopool.Go(func() {
			if err := platformcache.WriteUserCache(userCopy.Id, userCopy.ToBaseUser()); err != nil {
				platformobservability.SysLog("failed to update user status cache: " + err.Error())
			}
		})
	}
	return user.ToBaseUser(), nil
}

func LoadUserGroup(userID int, selectAll bool) (string, error) {
	if !selectAll {
		userCache, err := LoadUserCacheSnapshot(userID)
		if err == nil && userCache != nil {
			return userCache.Group, nil
		}
	}
	var group string
	err := platformdb.DB.Model(&identityschema.User{}).Where("id = ?", userID).Select("group").Find(&group).Error
	if err != nil {
		return "", err
	}
	return group, nil
}

func LoadUserFromAccessToken(accessToken string) (*identityschema.User, error) {
	if accessToken == "" {
		return nil, nil
	}
	token := accessToken
	if len(token) > 7 && token[:7] == "Bearer " {
		token = token[7:]
	}
	user := &identityschema.User{}
	err := platformdb.DB.Where("access_token = ?", token).First(user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return user, nil
}

func IsUserAdmin(userID int) bool {
	user, err := LoadUserByID(userID, false)
	if err != nil || user == nil {
		return false
	}
	return user.Role >= constant.RoleAdminUser
}

func WriteUserCacheToContext(c *gin.Context, userID int) error {
	userCache, err := LoadUserCacheSnapshot(userID)
	if err != nil {
		return err
	}
	WriteUserContext(c, userCache)
	return nil
}

func LoadUserLanguage(userID int) string {
	userCache, err := LoadUserCacheSnapshot(userID)
	if err != nil {
		return ""
	}
	return identitydomain.GetBaseSetting(userCache).Language
}

func LoadUserSetting(userID int, fromDB bool) (dto.UserSetting, error) {
	if !fromDB {
		userCache, err := LoadUserCacheSnapshot(userID)
		if err == nil && userCache != nil {
			return identitydomain.GetBaseSetting(userCache), nil
		}
	}

	var safeSetting sql.NullString
	if err := platformdb.DB.Model(&identityschema.User{}).Where("id = ?", userID).Select("setting").Find(&safeSetting).Error; err != nil {
		return dto.UserSetting{}, err
	}

	userBase := &identityschema.UserBase{}
	if safeSetting.Valid {
		userBase.Setting = safeSetting.String
	}
	return identitydomain.GetBaseSetting(userBase), nil
}

func LoadUserQuota(userID int, fromDB bool) (int, error) {
	if !fromDB {
		userCache, err := LoadUserCacheSnapshot(userID)
		if err == nil && userCache != nil {
			return userCache.Quota, nil
		}
	}

	var quota int
	if err := platformdb.DB.Model(&identityschema.User{}).Where("id = ?", userID).Select("quota").Find(&quota).Error; err != nil {
		return 0, err
	}
	return quota, nil
}

func LoadUsernameByID(userID int, fromDB bool) (string, error) {
	if !fromDB {
		userCache, err := LoadUserCacheSnapshot(userID)
		if err == nil && userCache != nil {
			return userCache.Username, nil
		}
	}

	var username string
	if err := platformdb.DB.Model(&identityschema.User{}).Where("id = ?", userID).Select("username").Find(&username).Error; err != nil {
		return "", err
	}
	return username, nil
}

func InvalidateUserCache(userID int) error {
	return platformcache.DeleteUserCache(userID)
}

func UpdateUserGroupCache(userID int, group string) error {
	return platformcache.SetUserCacheField(userID, "Group", group)
}
