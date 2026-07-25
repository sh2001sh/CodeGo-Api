package notifyx

import (
	"fmt"
	"github.com/sh2001sh/CodeGo-Api/constant"
	"github.com/sh2001sh/CodeGo-Api/dto"
	identitydomain "github.com/sh2001sh/CodeGo-Api/internal/identity/domain"
	identityschema "github.com/sh2001sh/CodeGo-Api/internal/identity/schema"
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
	platformobservability "github.com/sh2001sh/CodeGo-Api/internal/platform/observability"
	// NotifyUpstreamModelUpdateWatchers sends upstream model update notifications to opted-in admins.
)

func NotifyUpstreamModelUpdateWatchers(subject string, content string) {
	var users []identityschema.User
	if err := platformdb.DB.
		Select("id", "email", "role", "status", "setting").
		Where("status = ? AND role >= ?", constant.UserStatusEnabled, constant.RoleAdminUser).
		Find(&users).Error; err != nil {
		platformobservability.SysLog(fmt.Sprintf("failed to query upstream update notification users: %s", err.Error()))
		return
	}

	notification := dto.NewNotify(dto.NotifyTypeChannelUpdate, subject, content, nil)
	sentCount := 0
	for _, user := range users {
		userSetting := identitydomain.GetSetting(&user)
		if !userSetting.UpstreamModelUpdateNotifyEnabled {
			continue
		}
		if err := NotifyUser(user.Id, user.Email, userSetting, notification); err != nil {
			platformobservability.SysLog(fmt.Sprintf("failed to notify user %d for upstream model update: %s", user.Id, err.Error()))
			continue
		}
		sentCount++
	}
	platformobservability.SysLog(fmt.Sprintf("upstream model update notifications sent: %d", sentCount))
}
