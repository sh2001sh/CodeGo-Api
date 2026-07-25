package app

import (
	commerceschema "github.com/sh2001sh/CodeGo-Api/internal/commerce/schema"
	commercestore "github.com/sh2001sh/CodeGo-Api/internal/commerce/store"
	identityschema "github.com/sh2001sh/CodeGo-Api/internal/identity/schema"
	identitystore "github.com/sh2001sh/CodeGo-Api/internal/identity/store"
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
	platformruntime "github.com/sh2001sh/CodeGo-Api/internal/platform/runtime"
	"gorm.io/gorm"
	"strings"
)

// ExpireDueSubscriptions marks expired subscriptions and handles group downgrade.
func ExpireDueSubscriptions(limit int) (int, error) {
	if limit <= 0 {
		limit = 200
	}

	now := commercestore.GetDBTimestamp()
	var subs []commerceschema.UserSubscription
	if err := platformdb.DB.Where("status = ? AND end_time > 0 AND end_time <= ?", "active", now).
		Order("end_time asc, id asc").
		Limit(limit).
		Find(&subs).Error; err != nil {
		return 0, err
	}
	if len(subs) == 0 {
		return 0, nil
	}

	expiredCount := 0
	userIDs := make(map[int]struct{}, len(subs))
	for _, sub := range subs {
		if sub.UserId > 0 {
			userIDs[sub.UserId] = struct{}{}
		}
	}
	for userID := range userIDs {
		cacheGroup := ""
		err := platformdb.DB.Transaction(func(tx *gorm.DB) error {
			res := tx.Model(&commerceschema.UserSubscription{}).
				Where("user_id = ? AND status = ? AND end_time > 0 AND end_time <= ?", userID, "active", now).
				Updates(map[string]any{
					"status":     "expired",
					"updated_at": platformruntime.GetTimestamp(),
				})
			if res.Error != nil {
				return res.Error
			}
			expiredCount += int(res.RowsAffected)

			var activeSub commerceschema.UserSubscription
			activeQuery := tx.Where("user_id = ? AND status = ? AND end_time > ? AND upgrade_group <> ''",
				userID, "active", now).
				Order("end_time desc, id desc").
				Limit(1).
				Find(&activeSub)
			if activeQuery.Error == nil && activeQuery.RowsAffected > 0 {
				return nil
			}

			var lastExpired commerceschema.UserSubscription
			expiredQuery := tx.Where("user_id = ? AND status = ? AND upgrade_group <> ''",
				userID, "expired").
				Order("end_time desc, id desc").
				Limit(1).
				Find(&lastExpired)
			if expiredQuery.Error != nil || expiredQuery.RowsAffected == 0 {
				return nil
			}

			upgradeGroup := strings.TrimSpace(lastExpired.UpgradeGroup)
			prevGroup := strings.TrimSpace(lastExpired.PrevUserGroup)
			if upgradeGroup == "" || prevGroup == "" {
				return nil
			}
			currentGroup, err := getUserGroupByIDTx(tx, userID)
			if err != nil {
				return err
			}
			if currentGroup != upgradeGroup || currentGroup == prevGroup {
				return nil
			}
			if err := tx.Model(&identityschema.User{}).Where("id = ?", userID).Update("group", prevGroup).Error; err != nil {
				return err
			}
			cacheGroup = prevGroup
			return nil
		})
		if err != nil {
			return expiredCount, err
		}
		if cacheGroup != "" {
			_ = identitystore.UpdateUserGroupCache(userID, cacheGroup)
		}
	}
	return expiredCount, nil
}

// ResetDueSubscriptions resets subscriptions whose next reset time has passed.
func ResetDueSubscriptions(limit int) (int, error) {
	if limit <= 0 {
		limit = 200
	}

	now := commercestore.GetDBTimestamp()
	var subs []commerceschema.UserSubscription
	if err := platformdb.DB.Where("next_reset_time > 0 AND next_reset_time <= ? AND status = ?", now, "active").
		Order("next_reset_time asc").
		Limit(limit).
		Find(&subs).Error; err != nil {
		return 0, err
	}
	if len(subs) == 0 {
		return 0, nil
	}

	resetCount := 0
	for _, sub := range subs {
		subCopy := sub
		err := platformdb.DB.Transaction(func(tx *gorm.DB) error {
			locked := &commerceschema.UserSubscription{}
			if err := tx.Set("gorm:query_option", "FOR UPDATE").
				Where("id = ? AND next_reset_time > 0 AND next_reset_time <= ?", subCopy.Id, now).
				First(locked).Error; err != nil {
				return nil
			}
			plan, err := getSubscriptionPlanRecordTx(tx, locked.PlanId)
			if err != nil || plan == nil {
				return nil
			}
			if err := maybeResetUserSubscriptionWithPlanTx(tx, locked, plan, now); err != nil {
				return err
			}
			resetCount++
			return nil
		})
		if err != nil {
			return resetCount, err
		}
	}
	return resetCount, nil
}

// CleanupSubscriptionPreConsumeRecords removes old idempotency records.
func CleanupSubscriptionPreConsumeRecords(olderThanSeconds int64) (int64, error) {
	if olderThanSeconds <= 0 {
		olderThanSeconds = 7 * 24 * 3600
	}
	cutoff := commercestore.GetDBTimestamp() - olderThanSeconds
	res := platformdb.DB.Where("updated_at < ?", cutoff).Delete(&commerceschema.SubscriptionPreConsumeRecord{})
	return res.RowsAffected, res.Error
}
