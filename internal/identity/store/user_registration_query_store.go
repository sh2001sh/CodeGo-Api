package store

import (
	"errors"
	identityschema "github.com/sh2001sh/CodeGo-Api/internal/identity/schema"

	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
	"gorm.io/gorm"
)

func UserExistsOrDeleted(username string, email string) (bool, error) {
	var user identityschema.User

	var err error
	if email == "" {
		err = platformdb.DB.Unscoped().First(&user, "username = ?", username).Error
	} else {
		err = platformdb.DB.Unscoped().First(&user, "username = ? or email = ?", username, email).Error
	}
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func LoadMaxUserID() int {
	var user identityschema.User
	platformdb.DB.Unscoped().Last(&user)
	return user.Id
}
