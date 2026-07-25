package store

import (
	"errors"
	identityschema "github.com/sh2001sh/CodeGo-Api/internal/identity/schema"

	platformcache "github.com/sh2001sh/CodeGo-Api/internal/platform/cache"
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
	platformsecurity "github.com/sh2001sh/CodeGo-Api/internal/platform/security"
)

func UpdateUser(user *identityschema.User, updatePassword bool) error {
	if user == nil {
		return errors.New("user is nil")
	}

	if updatePassword {
		hashedPassword, err := platformsecurity.Password2Hash(user.Password)
		if err != nil {
			return err
		}
		user.Password = hashedPassword
	}

	newUser := *user
	storedUser := identityschema.User{}
	if err := platformdb.DB.First(&storedUser, user.Id).Error; err != nil {
		return err
	}
	newUser.ExternalId = storedUser.ExternalId
	if err := platformdb.DB.Model(&storedUser).Updates(newUser).Error; err != nil {
		return err
	}

	updatedUser := identityschema.User{}
	if err := platformdb.DB.First(&updatedUser, newUser.Id).Error; err != nil {
		return err
	}
	*user = updatedUser
	return platformcache.WriteUserCache(updatedUser.Id, updatedUser.ToBaseUser())
}

func EditUser(user *identityschema.User, updatePassword bool) error {
	if user == nil {
		return errors.New("user is nil")
	}

	if updatePassword {
		hashedPassword, err := platformsecurity.Password2Hash(user.Password)
		if err != nil {
			return err
		}
		user.Password = hashedPassword
	}

	newUser := *user
	updates := map[string]any{
		"username":     newUser.Username,
		"display_name": newUser.DisplayName,
		"group":        newUser.Group,
		"remark":       newUser.Remark,
	}
	if updatePassword {
		updates["password"] = newUser.Password
	}

	storedUser := identityschema.User{}
	if err := platformdb.DB.First(&storedUser, user.Id).Error; err != nil {
		return err
	}
	if err := platformdb.DB.Model(&storedUser).Updates(updates).Error; err != nil {
		return err
	}

	updatedUser := identityschema.User{}
	if err := platformdb.DB.First(&updatedUser, newUser.Id).Error; err != nil {
		return err
	}
	*user = updatedUser
	return platformcache.WriteUserCache(updatedUser.Id, updatedUser.ToBaseUser())
}

func ClearUserBinding(userID int, bindingType string) (*identityschema.User, error) {
	if userID == 0 {
		return nil, errors.New("user id is empty")
	}

	bindingColumnMap := map[string]string{
		"email":    "email",
		"github":   "github_id",
		"discord":  "discord_id",
		"oidc":     "oidc_id",
		"wechat":   "wechat_id",
		"telegram": "telegram_id",
		"linuxdo":  "linux_do_id",
	}

	column, ok := bindingColumnMap[bindingType]
	if !ok {
		return nil, errors.New("invalid binding type")
	}

	if err := platformdb.DB.Model(&identityschema.User{}).Where("id = ?", userID).Update(column, "").Error; err != nil {
		return nil, err
	}

	user, err := LoadUserByID(userID, false)
	if err != nil {
		return nil, err
	}
	if err := platformcache.WriteUserCache(user.Id, user.ToBaseUser()); err != nil {
		return nil, err
	}
	return user, nil
}

func DeleteUserByID(userID int) error {
	if userID == 0 {
		return errors.New("id 为空！")
	}
	if err := platformdb.DB.Delete(&identityschema.User{}, "id = ?", userID).Error; err != nil {
		return err
	}
	return platformcache.DeleteUserCache(userID)
}

func UpdateUserGitHubID(userID int, newGitHubID string) error {
	if userID == 0 {
		return errors.New("user id is empty")
	}
	if err := platformdb.DB.Model(&identityschema.User{}).Where("id = ?", userID).Update("github_id", newGitHubID).Error; err != nil {
		return err
	}
	user, err := LoadUserByID(userID, false)
	if err != nil {
		return err
	}
	return platformcache.WriteUserCache(user.Id, user.ToBaseUser())
}
