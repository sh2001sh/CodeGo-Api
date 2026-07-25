package app

import (
	"fmt"

	auditapp "github.com/sh2001sh/CodeGo-Api/internal/audit/app"
	auditschema "github.com/sh2001sh/CodeGo-Api/internal/audit/schema"
	identityschema "github.com/sh2001sh/CodeGo-Api/internal/identity/schema"
	identitystore "github.com/sh2001sh/CodeGo-Api/internal/identity/store"
	platformconfig "github.com/sh2001sh/CodeGo-Api/internal/platform/config"
	"github.com/sh2001sh/CodeGo-Api/internal/platform/logger"
	platformobservability "github.com/sh2001sh/CodeGo-Api/internal/platform/observability"
)

func createUserAndRecordRegistration(user *identityschema.User) error {
	if user == nil {
		return fmt.Errorf("user is nil")
	}
	if err := identitystore.CreateUser(user); err != nil {
		return err
	}
	recordRegistrationLog(user.Id)
	return nil
}

func finalizeOAuthUserRegistration(user *identityschema.User) {
	if user == nil {
		return
	}
	if err := identitystore.FinalizeCreatedUser(user.Id); err != nil {
		platformobservability.SysLog(fmt.Sprintf("failed to finalize created user %d: %v", user.Id, err))
	}
	recordRegistrationLog(user.Id)
}

func recordRegistrationLog(userID int) {
	if userID <= 0 || platformconfig.QuotaForNewUser <= 0 {
		return
	}
	auditapp.RecordLog(
		userID,
		auditschema.LogTypeSystem,
		fmt.Sprintf("新用户注册赠送 %s", logger.LogQuota(platformconfig.QuotaForNewUser)),
	)
}
