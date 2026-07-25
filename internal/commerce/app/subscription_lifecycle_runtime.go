package app

import (
	"errors"
	"fmt"
	commercedomain "github.com/sh2001sh/CodeGo-Api/internal/commerce/domain"
	commerceschema "github.com/sh2001sh/CodeGo-Api/internal/commerce/schema"
	commercestore "github.com/sh2001sh/CodeGo-Api/internal/commerce/store"
	identityschema "github.com/sh2001sh/CodeGo-Api/internal/identity/schema"
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
	platformruntime "github.com/sh2001sh/CodeGo-Api/internal/platform/runtime"
	"gorm.io/gorm"
	"math"
	"strings"
	"time"
)

func calcSubscriptionPlanEndTime(start time.Time, plan *commerceschema.SubscriptionPlan) (int64, error) {
	if plan == nil {
		return 0, errors.New("plan is nil")
	}
	if plan.DurationValue <= 0 && plan.DurationUnit != commerceschema.SubscriptionDurationCustom {
		return 0, errors.New("duration_value must be > 0")
	}

	switch plan.DurationUnit {
	case commerceschema.SubscriptionDurationYear:
		return start.AddDate(plan.DurationValue, 0, 0).Unix(), nil
	case commerceschema.SubscriptionDurationMonth:
		return start.AddDate(0, plan.DurationValue, 0).Unix(), nil
	case commerceschema.SubscriptionDurationDay:
		return start.Add(time.Duration(plan.DurationValue) * 24 * time.Hour).Unix(), nil
	case commerceschema.SubscriptionDurationHour:
		return start.Add(time.Duration(plan.DurationValue) * time.Hour).Unix(), nil
	case commerceschema.SubscriptionDurationCustom:
		if plan.CustomSeconds <= 0 {
			return 0, errors.New("custom_seconds must be > 0")
		}
		return start.Add(time.Duration(plan.CustomSeconds) * time.Second).Unix(), nil
	default:
		return 0, fmt.Errorf("invalid duration_unit: %s", plan.DurationUnit)
	}
}

func resolveSubscriptionPurchasePreviewTx(tx *gorm.DB, userID int, targetPlan *commerceschema.SubscriptionPlan) (*commercedomain.SubscriptionPurchasePreview, error) {
	if targetPlan == nil || targetPlan.Id <= 0 {
		return nil, errors.New("invalid plan")
	}
	if userID <= 0 {
		return nil, errors.New("invalid userId")
	}

	preview := &commercedomain.SubscriptionPurchasePreview{
		Action:        commerceschema.SubscriptionPurchaseActionSubscribe,
		BaseAmountDue: targetPlan.PriceAmount,
		AmountDue:     targetPlan.PriceAmount,
	}
	if isManagedSubscriptionPlan(targetPlan) {
		if err := applyManagedSubscriptionPreview(tx, userID, targetPlan, preview); err != nil {
			return nil, err
		}
	}
	return preview, nil
}

func applyManagedSubscriptionPreview(tx *gorm.DB, userID int, targetPlan *commerceschema.SubscriptionPlan, preview *commercedomain.SubscriptionPurchasePreview) error {
	currentSub, currentPlan, err := pickPrimaryActivePackageTx(tx, userID, commercestore.GetDBTimestamp())
	if err != nil || currentSub == nil || currentPlan == nil {
		return err
	}

	preview.CurrentSubscription = currentSub
	preview.CurrentPlan = currentPlan
	remainingQuota := max(currentSub.AmountTotal-currentSub.AmountUsed, 0)
	hasRemainingQuota := currentSub.AmountTotal <= 0 || hasMeaningfulSubscriptionQuotaRemaining(currentSub)

	switch compareSubscriptionPlanTier(targetPlan, currentPlan) {
	case -1:
		if hasRemainingQuota {
			preview.Action = commerceschema.SubscriptionPurchaseActionDisabled
			preview.BaseAmountDue = 0
			preview.AmountDue = 0
			preview.DisabledReason = "cannot subscribe to a lower-tier plan while your current package still has remaining quota"
		}
	case 0:
		if !isSubscriptionRenewalEligible(currentSub) {
			preview.Action = commerceschema.SubscriptionPurchaseActionDisabled
			preview.BaseAmountDue = 0
			preview.AmountDue = 0
			preview.DisabledReason = subscriptionRenewalQuotaThresholdReason
			return nil
		}
		preview.Action = commerceschema.SubscriptionPurchaseActionRenew
		preview.BaseAmountDue = calculateRenewalPrice(targetPlan, currentSub)
		preview.AmountDue = preview.BaseAmountDue
	default:
		preview.Action = commerceschema.SubscriptionPurchaseActionUpgrade
		discount := 0.0
		if currentPlan.PriceAmount > 0 && currentSub.AmountTotal > 0 && remainingQuota > 0 {
			discount = currentPlan.PriceAmount * float64(remainingQuota) / float64(currentSub.AmountTotal)
		}
		preview.BaseAmountDue = math.Max(targetPlan.PriceAmount-discount, 0.01)
		preview.AmountDue = preview.BaseAmountDue
	}
	return nil
}

// CreateUserSubscriptionFromPlanTx creates an active subscription snapshot from a plan.
func CreateUserSubscriptionFromPlanTx(tx *gorm.DB, userID int, plan *commerceschema.SubscriptionPlan, source string) (*commerceschema.UserSubscription, error) {
	if tx == nil {
		return nil, errors.New("tx is nil")
	}
	if plan == nil || plan.Id == 0 {
		return nil, errors.New("invalid plan")
	}
	if userID <= 0 {
		return nil, errors.New("invalid user id")
	}

	if plan.MaxPurchasePerUser > 0 {
		var count int64
		if err := tx.Model(&commerceschema.UserSubscription{}).
			Where("user_id = ? AND plan_id = ?", userID, plan.Id).
			Count(&count).Error; err != nil {
			return nil, err
		}
		if count >= int64(plan.MaxPurchasePerUser) {
			return nil, errors.New("已达到该套餐购买上限")
		}
	}

	nowUnix := commercestore.GetDBTimestamp()
	now := time.Unix(nowUnix, 0)
	endUnix, err := calcSubscriptionPlanEndTime(now, plan)
	if err != nil {
		return nil, err
	}
	nextReset := calcNextSubscriptionResetTime(now, plan, endUnix)
	lastReset := int64(0)
	if nextReset > 0 {
		lastReset = now.Unix()
	}

	upgradeGroup := strings.TrimSpace(plan.UpgradeGroup)
	prevGroup := ""
	if upgradeGroup != "" {
		currentGroup, err := getUserGroupByIDTx(tx, userID)
		if err != nil {
			return nil, err
		}
		if currentGroup != upgradeGroup {
			prevGroup = currentGroup
			if err := tx.Model(&identityschema.User{}).Where("id = ?", userID).Update("group", upgradeGroup).Error; err != nil {
				return nil, err
			}
		}
	}

	sub := &commerceschema.UserSubscription{
		UserId:        userID,
		PlanId:        plan.Id,
		AmountTotal:   plan.TotalAmount,
		AmountUsed:    0,
		PeriodAmount:  plan.PeriodAmount,
		PeriodUsed:    0,
		ModelLimits:   plan.ModelLimits,
		ModelUsage:    "",
		StartTime:     now.Unix(),
		EndTime:       endUnix,
		Status:        "active",
		Source:        source,
		LastResetTime: lastReset,
		NextResetTime: nextReset,
		UpgradeGroup:  upgradeGroup,
		PrevUserGroup: prevGroup,
		CreatedAt:     platformruntime.GetTimestamp(),
		UpdatedAt:     platformruntime.GetTimestamp(),
	}
	if err := tx.Create(sub).Error; err != nil {
		return nil, err
	}
	return sub, nil
}

func applySubscriptionUpgradeGroupTx(tx *gorm.DB, sub *commerceschema.UserSubscription, plan *commerceschema.SubscriptionPlan) error {
	if tx == nil || sub == nil || plan == nil {
		return errors.New("invalid upgrade group args")
	}
	upgradeGroup := strings.TrimSpace(plan.UpgradeGroup)
	sub.UpgradeGroup = upgradeGroup
	if upgradeGroup == "" {
		return nil
	}

	currentGroup, err := getUserGroupByIDTx(tx, sub.UserId)
	if err != nil {
		return err
	}
	if currentGroup == upgradeGroup {
		if strings.TrimSpace(sub.PrevUserGroup) == "" {
			sub.PrevUserGroup = currentGroup
		}
		return nil
	}
	if strings.TrimSpace(sub.PrevUserGroup) == "" {
		sub.PrevUserGroup = currentGroup
	}
	return tx.Model(&identityschema.User{}).Where("id = ?", sub.UserId).Update("group", upgradeGroup).Error
}

func renewUserSubscriptionWithPlanTx(tx *gorm.DB, sub *commerceschema.UserSubscription, plan *commerceschema.SubscriptionPlan, source string) (*commerceschema.UserSubscription, error) {
	if tx == nil || sub == nil || plan == nil {
		return nil, errors.New("invalid renewal args")
	}
	nowUnix := commercestore.GetDBTimestamp()
	now := time.Unix(nowUnix, 0)
	newEndTime, err := calcSubscriptionPlanEndTime(now, plan)
	if err != nil {
		return nil, err
	}

	sub.StartTime = nowUnix
	sub.EndTime = newEndTime
	sub.Status = "active"
	sub.Source = source
	sub.AmountTotal = plan.TotalAmount
	sub.AmountUsed = 0
	sub.PeriodAmount = plan.PeriodAmount
	sub.PeriodUsed = 0
	sub.ModelLimits = plan.ModelLimits
	sub.ModelUsage = ""
	if err := applySubscriptionUpgradeGroupTx(tx, sub, plan); err != nil {
		return nil, err
	}

	if commercedomain.NormalizeResetPeriod(plan.QuotaResetPeriod) == commerceschema.SubscriptionResetNever {
		sub.LastResetTime = 0
		sub.NextResetTime = 0
	} else {
		sub.LastResetTime = nowUnix
		sub.NextResetTime = calcNextSubscriptionResetTime(now, plan, sub.EndTime)
		if err := maybeResetUserSubscriptionWithPlanTx(tx, sub, plan, nowUnix); err != nil {
			return nil, err
		}
	}
	if err := tx.Save(sub).Error; err != nil {
		return nil, err
	}
	if err := replenishSubscriptionLedgerForCycleTx(tx, sub, "renewal"); err != nil {
		return nil, err
	}
	return sub, nil
}

func upgradeUserSubscriptionWithPlanTx(tx *gorm.DB, sub *commerceschema.UserSubscription, plan *commerceschema.SubscriptionPlan, source string) (*commerceschema.UserSubscription, error) {
	if tx == nil || sub == nil || plan == nil {
		return nil, errors.New("invalid upgrade args")
	}
	sub.PlanId = plan.Id
	sub.Status = "active"
	sub.Source = source
	sub.ModelLimits = plan.ModelLimits

	nowUnix := commercestore.GetDBTimestamp()
	now := time.Unix(nowUnix, 0)
	endUnix, err := calcSubscriptionPlanEndTime(now, plan)
	if err != nil {
		return nil, err
	}
	sub.StartTime = nowUnix
	sub.EndTime = endUnix
	sub.AmountTotal = plan.TotalAmount
	sub.AmountUsed = 0
	sub.PeriodAmount = plan.PeriodAmount
	sub.PeriodUsed = 0
	if err := applySubscriptionUpgradeGroupTx(tx, sub, plan); err != nil {
		return nil, err
	}

	if commercedomain.NormalizeResetPeriod(plan.QuotaResetPeriod) == commerceschema.SubscriptionResetNever {
		sub.LastResetTime = 0
		sub.NextResetTime = 0
	} else {
		sub.LastResetTime = nowUnix
		sub.NextResetTime = calcNextSubscriptionResetTime(now, plan, sub.EndTime)
		if err := maybeResetUserSubscriptionWithPlanTx(tx, sub, plan, nowUnix); err != nil {
			return nil, err
		}
	}
	if err := tx.Save(sub).Error; err != nil {
		return nil, err
	}
	if err := replenishSubscriptionLedgerForCycleTx(tx, sub, "upgrade"); err != nil {
		return nil, err
	}
	return sub, nil
}

// ApplySubscriptionPurchaseTx applies subscribe/renew/upgrade semantics inside a transaction.
func ApplySubscriptionPurchaseTx(tx *gorm.DB, userID int, plan *commerceschema.SubscriptionPlan, source string) (*commerceschema.UserSubscription, *commercedomain.SubscriptionPurchasePreview, error) {
	if tx == nil {
		return nil, nil, errors.New("tx is nil")
	}
	if plan == nil || plan.Id == 0 {
		return nil, nil, errors.New("invalid plan")
	}

	preview, err := resolveSubscriptionPurchasePreviewTx(tx, userID, plan)
	if err != nil {
		return nil, nil, err
	}
	switch preview.Action {
	case commerceschema.SubscriptionPurchaseActionDisabled:
		if strings.TrimSpace(preview.DisabledReason) != "" {
			return nil, preview, errors.New(preview.DisabledReason)
		}
		return nil, preview, errors.New("plan is not available for the current subscription")
	case commerceschema.SubscriptionPurchaseActionRenew:
		if preview.CurrentSubscription != nil {
			sub, err := renewUserSubscriptionWithPlanTx(tx, preview.CurrentSubscription, plan, source)
			return sub, preview, err
		}
	case commerceschema.SubscriptionPurchaseActionUpgrade:
		if preview.CurrentSubscription != nil {
			sub, err := upgradeUserSubscriptionWithPlanTx(tx, preview.CurrentSubscription, plan, source)
			return sub, preview, err
		}
	}

	sub, err := CreateUserSubscriptionFromPlanTx(tx, userID, plan, source)
	return sub, preview, err
}

func pickPrimaryActivePackageTx(tx *gorm.DB, userID int, now int64) (*commerceschema.UserSubscription, *commerceschema.SubscriptionPlan, error) {
	query := platformdb.DB
	if tx != nil {
		query = tx
	}
	var subs []commerceschema.UserSubscription
	if err := query.Where("user_id = ? AND status = ? AND end_time > ?", userID, "active", now).
		Order("end_time desc, id desc").
		Find(&subs).Error; err != nil {
		return nil, nil, err
	}

	var pickedSub *commerceschema.UserSubscription
	var pickedPlan *commerceschema.SubscriptionPlan
	for _, candidate := range subs {
		plan, err := getSubscriptionPlanRecordTx(tx, candidate.PlanId)
		if err != nil || !isManagedSubscriptionPlan(plan) {
			continue
		}
		candidateCopy := candidate
		if pickedSub == nil || compareSubscriptionPlanTier(plan, pickedPlan) > 0 || (compareSubscriptionPlanTier(plan, pickedPlan) == 0 && candidate.EndTime > pickedSub.EndTime) {
			pickedSub = &candidateCopy
			pickedPlan = plan
		}
	}
	return pickedSub, pickedPlan, nil
}
