package app

import (
	"github.com/sh2001sh/CodeGo-Api/constant"
	identityschema "github.com/sh2001sh/CodeGo-Api/internal/identity/schema"
	platformconfig "github.com/sh2001sh/CodeGo-Api/internal/platform/config"
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
	platformops "github.com/sh2001sh/CodeGo-Api/internal/platform/opssettings"
	platformschema "github.com/sh2001sh/CodeGo-Api/internal/platform/schema"
	platformsecurity "github.com/sh2001sh/CodeGo-Api/internal/platform/security"
	platformstore "github.com/sh2001sh/CodeGo-Api/internal/platform/store"

	"time"
	// SetupStatus describes whether the instance has been initialized.
)

type SetupStatus struct {
	Status       bool   `json:"status"`
	RootInit     bool   `json:"root_init"`
	DatabaseType string `json:"database_type"`
}

// SetupRequest captures the initial setup submission payload.
type SetupRequest struct {
	Username           string `json:"username"`
	Password           string `json:"password"`
	ConfirmPassword    string `json:"confirmPassword"`
	SelfUseModeEnabled bool   `json:"SelfUseModeEnabled"`
	DemoSiteEnabled    bool   `json:"DemoSiteEnabled"`
}

// GetSetupStatus returns the current installation state.
func GetSetupStatus() SetupStatus {
	setup := SetupStatus{Status: constant.Setup}
	if constant.Setup {
		return setup
	}

	setup.RootInit = rootUserExists()
	switch {
	case platformdb.UsingMySQL:
		setup.DatabaseType = "mysql"
	case platformdb.UsingPostgreSQL:
		setup.DatabaseType = "postgres"
	case platformdb.UsingSQLite:
		setup.DatabaseType = "sqlite"
	}
	return setup
}

// CheckRuntimeHealth verifies core runtime dependencies needed for admin health checks.
func CheckRuntimeHealth() error {
	return pingDB()
}

// CompleteSetup persists the first-run setup state and bootstraps the root account if needed.
func CompleteSetup(req SetupRequest) error {
	if constant.Setup {
		return ErrSetupAlreadyCompleted
	}

	rootExists := rootUserExists()
	if !rootExists {
		if len(req.Username) > 12 {
			return ErrSetupUsernameTooLong
		}
		if req.Password != req.ConfirmPassword {
			return ErrSetupPasswordMismatch
		}
		if len(req.Password) < 8 {
			return ErrSetupPasswordTooShort
		}

		hashedPassword, err := platformsecurity.Password2Hash(req.Password)
		if err != nil {
			return wrapSetupError("系统错误: ", err)
		}

		rootUser := identityschema.User{
			Username:    req.Username,
			Password:    hashedPassword,
			Role:        constant.RoleRootUser,
			Status:      constant.UserStatusEnabled,
			DisplayName: "Root User",
			AccessToken: nil,
			Quota:       100000000,
		}
		if err := platformdb.DB.Create(&rootUser).Error; err != nil {
			return wrapSetupError("创建管理员账号失败: ", err)
		}
	}

	platformops.SetSelfUseModeEnabled(req.SelfUseModeEnabled)
	platformops.SetDemoSiteEnabled(req.DemoSiteEnabled)

	if err := platformstore.UpdateOption("SelfUseModeEnabled", formatSetupBool(req.SelfUseModeEnabled)); err != nil {
		return wrapSetupError("保存自用模式设置失败: ", err)
	}
	if err := platformstore.UpdateOption("DemoSiteEnabled", formatSetupBool(req.DemoSiteEnabled)); err != nil {
		return wrapSetupError("保存演示站点模式设置失败: ", err)
	}

	constant.Setup = true
	setup := platformschema.Setup{
		Version:       platformconfig.Version,
		InitializedAt: time.Now().Unix(),
	}
	if err := platformdb.DB.Create(&setup).Error; err != nil {
		return wrapSetupError("系统初始化失败: ", err)
	}

	return nil
}

func formatSetupBool(value bool) string {
	if value {
		return "true"
	}
	return "false"
}
