package store

import (
	"encoding/json"
	"errors"
	"fmt"
	identityschema "github.com/sh2001sh/CodeGo-Api/internal/identity/schema"
	platformerrx "github.com/sh2001sh/CodeGo-Api/internal/platform/errx"
	"strings"

	"github.com/sh2001sh/CodeGo-Api/constant"
	"github.com/sh2001sh/CodeGo-Api/dto"
	identitydomain "github.com/sh2001sh/CodeGo-Api/internal/identity/domain"
	platformcache "github.com/sh2001sh/CodeGo-Api/internal/platform/cache"
	platformconfig "github.com/sh2001sh/CodeGo-Api/internal/platform/config"
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
	platformobservability "github.com/sh2001sh/CodeGo-Api/internal/platform/observability"
	platformsecurity "github.com/sh2001sh/CodeGo-Api/internal/platform/security"
	"gorm.io/gorm"
)

func CreateUser(user *identityschema.User) error {
	if user == nil {
		return errors.New("user is nil")
	}
	if err := prepareNewUser(user); err != nil {
		return err
	}
	if err := platformdb.DB.Transaction(func(tx *gorm.DB) error {
		return createPreparedUser(tx, user)
	}); err != nil {
		return err
	}
	return FinalizeCreatedUser(user.Id)
}

func CreateUserWithTx(tx *gorm.DB, user *identityschema.User) error {
	if tx == nil {
		return errors.New("transaction is required")
	}
	if user == nil {
		return errors.New("user is nil")
	}
	if err := prepareNewUser(user); err != nil {
		return err
	}
	return createPreparedUser(tx, user)
}

func FinalizeCreatedUser(userID int) error {
	if userID == 0 {
		return errors.New("user id is empty")
	}
	createdUser, err := LoadUserByID(userID, true)
	if err != nil {
		return err
	}
	defaultSidebarConfig := generateDefaultSidebarConfigForRole(createdUser.Role)
	if defaultSidebarConfig == "" {
		return nil
	}

	currentSetting := identitydomain.GetSetting(createdUser)
	currentSetting.SidebarModules = defaultSidebarConfig
	identitydomain.SetSetting(createdUser, currentSetting)
	if err := persistUserSettingUpdate(createdUser); err != nil {
		return err
	}
	platformobservability.SysLog(fmt.Sprintf("为新用户 %s (角色: %d) 初始化边栏配置", createdUser.Username, createdUser.Role))
	return nil
}

func AuthenticateUserCredentials(username string, password string) (*identityschema.User, error) {
	username = strings.TrimSpace(username)
	if username == "" || password == "" {
		return nil, identitydomain.ErrUserEmptyCredentials
	}

	user := &identityschema.User{}
	err := platformdb.DB.Where("username = ? OR email = ?", username, username).First(user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, identitydomain.ErrInvalidCredentials
		}
		return nil, fmt.Errorf("%w: %v", platformerrx.ErrDatabase, err)
	}
	if !platformsecurity.ValidatePasswordAndHash(password, user.Password) || user.Status != constant.UserStatusEnabled {
		return nil, identitydomain.ErrInvalidCredentials
	}
	return user, nil
}

func prepareNewUser(user *identityschema.User) error {
	if user.Password != "" {
		hashedPassword, err := platformsecurity.Password2Hash(user.Password)
		if err != nil {
			return err
		}
		user.Password = hashedPassword
	}
	user.Quota = platformconfig.QuotaForNewUser
	if user.ExternalId == "" {
		externalID, err := identityschema.GenerateExternalUserID()
		if err != nil {
			return fmt.Errorf("generate external user id: %w", err)
		}
		user.ExternalId = externalID
	}
	if user.Setting == "" {
		defaultSetting := dto.UserSetting{}
		identitydomain.SetSetting(user, defaultSetting)
	}
	return nil
}

func createPreparedUser(db *gorm.DB, user *identityschema.User) error {
	const maxExternalIDAttempts = 5
	for attempt := 0; attempt < maxExternalIDAttempts; attempt++ {
		savePointName := fmt.Sprintf("external_user_id_%d", attempt)
		if err := db.SavePoint(savePointName).Error; err != nil {
			return err
		}
		err := db.Create(user).Error
		if err == nil {
			return nil
		}
		if !isExternalIDConflict(err) {
			return err
		}
		if err := db.RollbackTo(savePointName).Error; err != nil {
			return err
		}
		externalID, generateErr := identityschema.GenerateExternalUserID()
		if generateErr != nil {
			return fmt.Errorf("generate external user id: %w", generateErr)
		}
		user.Id = 0
		user.ExternalId = externalID
	}
	return errors.New("could not allocate a unique external user id")
}

func isExternalIDConflict(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "external_id")
}

func persistUserSettingUpdate(user *identityschema.User) error {
	if user == nil || user.Id == 0 {
		return errors.New("user is empty")
	}
	if err := platformdb.DB.Model(user).Update("setting", user.Setting).Error; err != nil {
		return err
	}
	return platformcache.WriteUserCache(user.Id, user.ToBaseUser())
}

func generateDefaultSidebarConfigForRole(userRole int) string {
	defaultConfig := map[string]any{}
	defaultConfig["chat"] = map[string]any{
		"enabled":    true,
		"playground": true,
		"chat":       true,
	}
	defaultConfig["console"] = map[string]any{
		"enabled": true,
		"detail":  true,
		"token":   true,
		"log":     true,
		"task":    true,
	}
	defaultConfig["personal"] = map[string]any{
		"enabled":  true,
		"topup":    true,
		"personal": true,
	}

	if userRole == constant.RoleAdminUser {
		defaultConfig["admin"] = map[string]any{
			"enabled":    true,
			"channel":    true,
			"models":     true,
			"redemption": true,
			"user":       true,
			"setting":    false,
		}
	} else if userRole == constant.RoleRootUser {
		defaultConfig["admin"] = map[string]any{
			"enabled":    true,
			"channel":    true,
			"models":     true,
			"redemption": true,
			"user":       true,
			"setting":    true,
		}
	}

	configBytes, err := json.Marshal(defaultConfig)
	if err != nil {
		platformobservability.SysLog("生成默认边栏配置失败: " + err.Error())
		return ""
	}
	return string(configBytes)
}
