package store

import (
	"testing"

	"github.com/glebarez/sqlite"
	identityschema "github.com/sh2001sh/CodeGo-Api/internal/identity/schema"
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestCreateUserWithTxAssignsImmutableExternalUserID(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&identityschema.User{}))

	originalDB := platformdb.DB
	t.Cleanup(func() { platformdb.DB = originalDB })
	platformdb.DB = db

	user := &identityschema.User{Username: "external-id-user", Password: "password123", DisplayName: "External ID User"}
	tx := db.Begin()
	require.NoError(t, CreateUserWithTx(tx, user))
	require.NoError(t, tx.Commit().Error)
	require.Len(t, user.ExternalId, identityschema.ExternalUserIDLength)

	originalExternalID := user.ExternalId
	user.ExternalId = "AAAAAA"
	user.DisplayName = "Updated User"
	require.NoError(t, UpdateUser(user, false))

	var reloaded identityschema.User
	require.NoError(t, db.First(&reloaded, user.Id).Error)
	require.Equal(t, originalExternalID, reloaded.ExternalId)
}
