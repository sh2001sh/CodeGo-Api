package store

import (
	"github.com/sh2001sh/CodeGo-Api/constant"
	identityschema "github.com/sh2001sh/CodeGo-Api/internal/identity/schema"
	platformconfig "github.com/sh2001sh/CodeGo-Api/internal/platform/config"
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
	platformobservability "github.com/sh2001sh/CodeGo-Api/internal/platform/observability"
	platformschema "github.com/sh2001sh/CodeGo-Api/internal/platform/schema"
	"time"
)

// CheckSetup synchronizes the runtime setup flag from persisted bootstrap state.
func CheckSetup() {
	setup := loadSetup()
	if setup == nil {
		var user identityschema.User
		if err := platformdb.DB.Where("role = ?", constant.RoleRootUser).First(&user).Error; err == nil {
			platformobservability.SysLog("system is not initialized, but root user exists")
			newSetup := platformschema.Setup{
				Version:       platformconfig.Version,
				InitializedAt: time.Now().Unix(),
			}
			if err := platformdb.DB.Create(&newSetup).Error; err != nil {
				platformobservability.SysLog("failed to create setup record: " + err.Error())
			}
			constant.Setup = true
		} else {
			platformobservability.SysLog("system is not initialized and no root user exists")
			constant.Setup = false
		}
		return
	}

	platformobservability.SysLog("system is already initialized at: " + time.Unix(setup.InitializedAt, 0).String())
	constant.Setup = true
}

func loadSetup() *platformschema.Setup {
	var setup platformschema.Setup
	if err := platformdb.DB.First(&setup).Error; err != nil {
		return nil
	}
	return &setup
}
