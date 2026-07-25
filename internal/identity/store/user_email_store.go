package store

import (
	identityschema "github.com/sh2001sh/CodeGo-Api/internal/identity/schema"
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
	platformsecurity "github.com/sh2001sh/CodeGo-Api/internal/platform/security"
)

func IsEmailTaken(email string) bool {
	return platformdb.DB.Unscoped().Where("email = ?", email).Find(&identityschema.User{}).RowsAffected == 1
}

func ResetUserPasswordByEmail(email string, password string) error {
	hashedPassword, err := platformsecurity.Password2Hash(password)
	if err != nil {
		return err
	}
	return platformdb.DB.Model(&identityschema.User{}).Where("email = ?", email).Update("password", hashedPassword).Error
}
