package app

import (
	"errors"
	"fmt"
	commercedomain "github.com/sh2001sh/CodeGo-Api/internal/commerce/domain"
	commerceschema "github.com/sh2001sh/CodeGo-Api/internal/commerce/schema"
	identityschema "github.com/sh2001sh/CodeGo-Api/internal/identity/schema"
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"

	"gorm.io/gorm"
	"strings"
	"time"
)

func subscriptionUserGroupColumn() string {
	if platformdb.UsingPostgreSQL {
		return `"group"`
	}
	return "`group`"
}

func getUserGroupByIDTx(tx *gorm.DB, userID int) (string, error) {
	if tx == nil {
		tx = platformdb.DB
	}
	if userID <= 0 {
		return "", errors.New("invalid userId")
	}

	var group string
	if err := tx.Model(&identityschema.User{}).
		Where("id = ?", userID).
		Select(subscriptionUserGroupColumn()).
		Scan(&group).Error; err != nil {
		return "", err
	}
	return group, nil
}

func calcNextSubscriptionResetTime(base time.Time, plan *commerceschema.SubscriptionPlan, endUnix int64) int64 {
	if plan == nil {
		return 0
	}
	period := commercedomain.NormalizeResetPeriod(plan.QuotaResetPeriod)
	if period == commerceschema.SubscriptionResetNever {
		return 0
	}

	var next time.Time
	switch period {
	case commerceschema.SubscriptionResetDaily:
		next = time.Date(base.Year(), base.Month(), base.Day(), 0, 0, 0, 0, base.Location()).AddDate(0, 0, 1)
	case commerceschema.SubscriptionResetWeekly:
		next = base.AddDate(0, 0, 7)
	case commerceschema.SubscriptionResetMonthly:
		next = time.Date(base.Year(), base.Month(), 1, 0, 0, 0, 0, base.Location()).AddDate(0, 1, 0)
	case commerceschema.SubscriptionResetCustom:
		if plan.QuotaResetCustomSeconds <= 0 {
			return 0
		}
		next = base.Add(time.Duration(plan.QuotaResetCustomSeconds) * time.Second)
	default:
		return 0
	}
	if endUnix > 0 && next.Unix() > endUnix {
		return 0
	}
	return next.Unix()
}

func usesLegacySubscriptionPeriodicQuota(plan *commerceschema.SubscriptionPlan, sub *commerceschema.UserSubscription) bool {
	if plan == nil || sub == nil {
		return false
	}
	if commercedomain.NormalizeResetPeriod(plan.QuotaResetPeriod) == commerceschema.SubscriptionResetNever {
		return false
	}
	return sub.PeriodAmount <= 0 && sub.AmountTotal > 0
}

func getSubscriptionPeriodAmount(plan *commerceschema.SubscriptionPlan, sub *commerceschema.UserSubscription) int64 {
	if sub != nil && sub.PeriodAmount > 0 {
		return sub.PeriodAmount
	}
	if usesLegacySubscriptionPeriodicQuota(plan, sub) && sub != nil {
		return sub.AmountTotal
	}
	if plan != nil && plan.PeriodAmount > 0 {
		return plan.PeriodAmount
	}
	return 0
}

func applySubscriptionUsageDelta(plan *commerceschema.SubscriptionPlan, sub *commerceschema.UserSubscription, modelName string, delta int64) error {
	if sub == nil {
		return errors.New("subscription is nil")
	}
	if delta == 0 {
		return nil
	}

	legacyPeriodicQuota := usesLegacySubscriptionPeriodicQuota(plan, sub)
	newAmountUsed := sub.AmountUsed + delta
	if newAmountUsed < 0 {
		newAmountUsed = 0
	}
	if sub.AmountTotal > 0 && newAmountUsed > sub.AmountTotal {
		return fmt.Errorf("subscription used exceeds total, used=%d total=%d", newAmountUsed, sub.AmountTotal)
	}
	sub.AmountUsed = newAmountUsed

	if !legacyPeriodicQuota {
		periodAmount := getSubscriptionPeriodAmount(plan, sub)
		if periodAmount > 0 {
			newPeriodUsed := sub.PeriodUsed + delta
			if newPeriodUsed < 0 {
				newPeriodUsed = 0
			}
			if newPeriodUsed > periodAmount {
				return fmt.Errorf("subscription period quota exceeded, used=%d period=%d", newPeriodUsed, periodAmount)
			}
			sub.PeriodUsed = newPeriodUsed
		}
	}

	trimmedModelName := strings.TrimSpace(modelName)
	if trimmedModelName == "" {
		return nil
	}
	limits := sub.GetModelLimitsMap()
	limit, ok := limits[trimmedModelName]
	if !ok || limit <= 0 {
		return nil
	}
	usage := sub.GetModelUsageMap()
	newUsage := usage[trimmedModelName] + delta
	if newUsage < 0 {
		newUsage = 0
	}
	if newUsage > limit {
		return fmt.Errorf("subscription model quota exceeded, model=%s used=%d limit=%d", trimmedModelName, newUsage, limit)
	}
	if newUsage == 0 {
		delete(usage, trimmedModelName)
	} else {
		usage[trimmedModelName] = newUsage
	}
	return sub.SetModelUsageMap(usage)
}

func maybeResetUserSubscriptionWithPlanTx(tx *gorm.DB, sub *commerceschema.UserSubscription, plan *commerceschema.SubscriptionPlan, now int64) error {
	if tx == nil || sub == nil || plan == nil {
		return errors.New("invalid reset args")
	}
	if commercedomain.NormalizeResetPeriod(plan.QuotaResetPeriod) == commerceschema.SubscriptionResetNever {
		return nil
	}

	baseUnix := sub.LastResetTime
	if baseUnix <= 0 || (sub.StartTime > 0 && baseUnix < sub.StartTime) {
		baseUnix = sub.StartTime
	}
	base := time.Unix(baseUnix, 0)
	next := calcNextSubscriptionResetTime(base, plan, sub.EndTime)
	if next == 0 || next > now {
		if sub.LastResetTime != base.Unix() || sub.NextResetTime != next {
			sub.LastResetTime = base.Unix()
			sub.NextResetTime = next
			return tx.Save(sub).Error
		}
		return nil
	}

	advanced := false
	for next > 0 && next <= now {
		advanced = true
		base = time.Unix(next, 0)
		next = calcNextSubscriptionResetTime(base, plan, sub.EndTime)
	}
	if !advanced {
		if sub.NextResetTime == 0 && next > 0 {
			sub.NextResetTime = next
			sub.LastResetTime = base.Unix()
			return tx.Save(sub).Error
		}
		return nil
	}

	if usesLegacySubscriptionPeriodicQuota(plan, sub) {
		sub.AmountUsed = 0
		if err := restoreSubscriptionLedgerBalanceAfterResetTx(tx, sub, fmt.Sprintf("periodic:%d:%d", sub.Id, base.Unix())); err != nil {
			return err
		}
	} else {
		sub.PeriodUsed = 0
	}
	sub.ModelUsage = ""
	sub.LastResetTime = base.Unix()
	sub.NextResetTime = next
	return tx.Save(sub).Error
}

func downgradeUserGroupForSubscriptionTx(tx *gorm.DB, sub *commerceschema.UserSubscription, now int64) (string, error) {
	if tx == nil || sub == nil {
		return "", errors.New("invalid downgrade args")
	}
	upgradeGroup := strings.TrimSpace(sub.UpgradeGroup)
	if upgradeGroup == "" {
		return "", nil
	}

	currentGroup, err := getUserGroupByIDTx(tx, sub.UserId)
	if err != nil {
		return "", err
	}
	if currentGroup != upgradeGroup {
		return "", nil
	}

	var activeSub commerceschema.UserSubscription
	activeQuery := tx.Where("user_id = ? AND status = ? AND end_time > ? AND id <> ? AND upgrade_group <> ''",
		sub.UserId, "active", now, sub.Id).
		Order("end_time desc, id desc").
		Limit(1).
		Find(&activeSub)
	if activeQuery.Error == nil && activeQuery.RowsAffected > 0 {
		return "", nil
	}

	prevGroup := strings.TrimSpace(sub.PrevUserGroup)
	if prevGroup == "" || prevGroup == currentGroup {
		return "", nil
	}
	if err := tx.Model(&identityschema.User{}).Where("id = ?", sub.UserId).Update("group", prevGroup).Error; err != nil {
		return "", err
	}
	return prevGroup, nil
}
