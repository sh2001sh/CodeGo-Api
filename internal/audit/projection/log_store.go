package projection

import (
	identityschema "github.com/sh2001sh/CodeGo-Api/internal/identity/schema"
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
)

func getAuditTokenNameByID(tokenID int) (string, error) {
	if tokenID <= 0 {
		return "", nil
	}
	var tokenName string
	err := platformdb.DB.Model(&identityschema.Token{}).Where("id = ?", tokenID).Select("name").Find(&tokenName).Error
	return tokenName, err
}
