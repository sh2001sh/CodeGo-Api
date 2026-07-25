package app

import (
	"github.com/sh2001sh/CodeGo-Api/constant"
	identityschema "github.com/sh2001sh/CodeGo-Api/internal/identity/schema"
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
)

func rootUserExists() bool {
	var user identityschema.User
	err := platformdb.DB.Where("role = ?", constant.RoleRootUser).First(&user).Error
	return err == nil
}

func pingDB() error {
	sqlDB, err := platformdb.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Ping()
}
