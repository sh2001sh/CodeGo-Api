package store

import (
	"testing"

	"github.com/glebarez/sqlite"
	gatewayschema "github.com/sh2001sh/CodeGo-Api/internal/gateway/schema"
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestChannelHasExclusiveEnabledAbility(t *testing.T) {
	originalDB := platformdb.DB
	originalSQLite := platformdb.UsingSQLite
	originalPostgreSQL := platformdb.UsingPostgreSQL
	t.Cleanup(func() {
		platformdb.DB = originalDB
		platformdb.UsingSQLite = originalSQLite
		platformdb.UsingPostgreSQL = originalPostgreSQL
	})

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&gatewayschema.Ability{}))
	platformdb.DB = db
	platformdb.UsingSQLite = true
	platformdb.UsingPostgreSQL = false

	require.NoError(t, db.Create(&gatewayschema.Ability{Group: "free", Model: "shared", ChannelId: 1, Enabled: true}).Error)
	require.NoError(t, db.Create(&gatewayschema.Ability{Group: "free", Model: "shared", ChannelId: 2, Enabled: true}).Error)
	require.NoError(t, db.Create(&gatewayschema.Ability{Group: "free", Model: "exclusive", ChannelId: 1, Enabled: true}).Error)
	require.NoError(t, db.Create(&gatewayschema.Ability{Group: "vip", Model: "shared", ChannelId: 1, Enabled: true}).Error)
	require.NoError(t, db.Create(&gatewayschema.Ability{Group: "vip", Model: "shared", ChannelId: 3, Enabled: false}).Error)

	exclusive, err := ChannelHasExclusiveEnabledAbility(1)
	require.NoError(t, err)
	require.True(t, exclusive)

	exclusive, err = ChannelHasExclusiveEnabledAbility(2)
	require.NoError(t, err)
	require.False(t, exclusive)

	alternative, err := HasAlternativeEnabledAbility(1, "free", "shared")
	require.NoError(t, err)
	require.True(t, alternative)

	alternative, err = HasAlternativeEnabledAbility(1, "free", "exclusive")
	require.NoError(t, err)
	require.False(t, alternative)
}
