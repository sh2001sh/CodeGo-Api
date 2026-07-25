package app

import (
	"errors"
	"fmt"
	billingdomain "github.com/sh2001sh/CodeGo-Api/internal/billing/domain"
	billingschema "github.com/sh2001sh/CodeGo-Api/internal/billing/schema"
	commercedomain "github.com/sh2001sh/CodeGo-Api/internal/commerce/domain"
	commerceschema "github.com/sh2001sh/CodeGo-Api/internal/commerce/schema"
	"strings"

	commercestore "github.com/sh2001sh/CodeGo-Api/internal/commerce/store"
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
	"gorm.io/gorm"
)

// PreConsumeUserSubscription pre-consumes quota from an active subscription.
func PreConsumeUserSubscription(requestID string, userID int, modelName string, amount int64) (*commercedomain.SubscriptionPreConsumeResult, error) {
	if userID <= 0 {
		return nil, errors.New("invalid userId")
	}
	if strings.TrimSpace(requestID) == "" {
		return nil, errors.New("requestId is empty")
	}
	if amount <= 0 {
		return nil, errors.New("amount must be > 0")
	}

	now := commercestore.GetDBTimestamp()
	result := &commercedomain.SubscriptionPreConsumeResult{}
	err := platformdb.DB.Transaction(func(tx *gorm.DB) error {
		record := &commerceschema.SubscriptionPreConsumeRecord{}
		query := tx.Where("request_id = ?", requestID).Limit(1).Find(record)
		if query.Error != nil {
			return query.Error
		}
		if query.RowsAffected > 0 {
			if record.Status == "refunded" {
				return errors.New("subscription pre-consume already refunded")
			}
			sub := &commerceschema.UserSubscription{}
			if err := tx.Where("id = ?", record.UserSubscriptionId).First(sub).Error; err != nil {
				return err
			}
			result.UserSubscriptionId = sub.Id
			result.PreConsumed = record.PreConsumed
			result.AmountTotal = sub.AmountTotal
			result.AmountUsedBefore = sub.AmountUsed
			result.AmountUsedAfter = sub.AmountUsed
			return nil
		}

		var subs []commerceschema.UserSubscription
		if err := tx.Set("gorm:query_option", "FOR UPDATE").
			Where("user_id = ? AND status = ? AND end_time > ?", userID, "active", now).
			Order("end_time asc, id asc").
			Find(&subs).Error; err != nil {
			return errors.New("no active subscription")
		}
		if len(subs) == 0 {
			return errors.New("no active subscription")
		}

		ordered, err := orderActiveUserSubscriptionsTx(tx, userID, subs)
		if err != nil {
			return err
		}
		for _, candidate := range ordered {
			sub := candidate
			plan, err := getSubscriptionPlanRecordTx(tx, sub.PlanId)
			if err != nil {
				return err
			}
			if err := maybeResetUserSubscriptionWithPlanTx(tx, &sub, plan, now); err != nil {
				return err
			}

			usedBefore := sub.AmountUsed
			if sub.AmountTotal > 0 {
				remain := sub.AmountTotal - usedBefore
				if remain < amount {
					continue
				}
			}

			periodAmount := getSubscriptionPeriodAmount(plan, &sub)
			if !usesLegacySubscriptionPeriodicQuota(plan, &sub) && periodAmount > 0 {
				periodRemain := periodAmount - sub.PeriodUsed
				if periodRemain < amount {
					continue
				}
			}

			trimmedModelName := strings.TrimSpace(modelName)
			if trimmedModelName != "" {
				modelLimits := sub.GetModelLimitsMap()
				if modelLimit, ok := modelLimits[trimmedModelName]; ok && modelLimit > 0 {
					modelUsage := sub.GetModelUsageMap()
					if modelUsage[trimmedModelName]+amount > modelLimit {
						continue
					}
				}
			}

			record = &commerceschema.SubscriptionPreConsumeRecord{
				RequestId:          requestID,
				UserId:             userID,
				UserSubscriptionId: sub.Id,
				ModelName:          trimmedModelName,
				PreConsumed:        amount,
				Status:             "consumed",
			}
			if err := tx.Create(record).Error; err != nil {
				dup := &commerceschema.SubscriptionPreConsumeRecord{}
				if err2 := tx.Where("request_id = ?", requestID).First(dup).Error; err2 == nil {
					if dup.Status == "refunded" {
						return errors.New("subscription pre-consume already refunded")
					}
					result.UserSubscriptionId = sub.Id
					result.PreConsumed = dup.PreConsumed
					result.AmountTotal = sub.AmountTotal
					result.AmountUsedBefore = sub.AmountUsed
					result.AmountUsedAfter = sub.AmountUsed
					return nil
				}
				return err
			}
			if err := reserveSubscriptionLedgerTx(tx, &sub, record); err != nil {
				return err
			}

			if err := applySubscriptionUsageDelta(plan, &sub, record.ModelName, amount); err != nil {
				return err
			}
			if err := tx.Save(&sub).Error; err != nil {
				return err
			}

			result.UserSubscriptionId = sub.Id
			result.PreConsumed = amount
			result.AmountTotal = sub.AmountTotal
			result.AmountUsedBefore = usedBefore
			result.AmountUsedAfter = sub.AmountUsed
			return nil
		}
		return fmt.Errorf("subscription quota insufficient, need=%d", amount)
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func reserveSubscriptionLedgerTx(tx *gorm.DB, sub *commerceschema.UserSubscription, record *commerceschema.SubscriptionPreConsumeRecord) error {
	if sub == nil || record == nil || sub.AmountTotal <= 0 {
		return nil
	}
	account, err := billingdomain.EnsureBillingAccountTx(tx, billingdomain.EnsureAccountParams{
		AccountType: "subscription",
		OwnerType:   "user_subscription",
		OwnerID:     int64(sub.Id),
		QuotaUnit:   "quota",
	})
	if err != nil {
		return err
	}

	var entryCount int64
	if err := tx.Model(&billingschema.BillingLedgerEntry{}).Where("account_id = ?", account.AccountID).Count(&entryCount).Error; err != nil {
		return err
	}
	if entryCount == 0 {
		available := sub.AmountTotal - sub.AmountUsed
		if available > 0 {
			if _, err := billingdomain.CreditAccountTx(tx, billingdomain.CreditAccountParams{
				AccountID:      account.AccountID,
				Amount:         available,
				IdempotencyKey: fmt.Sprintf("subscription-bootstrap:%d", sub.Id),
				ReasonCode:     "subscription_balance_bootstrap",
				ReferenceType:  "user_subscription",
				ReferenceID:    fmt.Sprintf("%d", sub.Id),
				OperatorType:   "subscription_projection",
				OperatorID:     record.RequestId,
			}); err != nil {
				return err
			}
		}
	}
	_, err = billingdomain.CreateReservationTx(tx, billingdomain.CreateReservationParams{
		AccountID:      account.AccountID,
		RequestID:      record.RequestId,
		ReservedAmount: record.PreConsumed,
		IdempotencyKey: "subscription:" + record.RequestId + ":reserve",
	})
	return err
}

// ReserveAdditionalSubscriptionQuota reserves a confirmed extra amount for a request.
// The subscription fields remain a query projection; ledger reservations enforce balance.
func ReserveAdditionalSubscriptionQuota(requestID string, subscriptionID int, modelName string, amount int64) error {
	if strings.TrimSpace(requestID) == "" || subscriptionID <= 0 || amount <= 0 {
		return errors.New("requestId, subscriptionId and amount are required")
	}
	return platformdb.DB.Transaction(func(tx *gorm.DB) error {
		sub := &commerceschema.UserSubscription{}
		if err := tx.Set("gorm:query_option", "FOR UPDATE").Where("id = ?", subscriptionID).First(sub).Error; err != nil {
			return err
		}
		now := commercestore.GetDBTimestamp()
		if sub.Status != "active" || sub.EndTime <= now {
			return errors.New("subscription is no longer active")
		}
		plan, err := getSubscriptionPlanRecordTx(tx, sub.PlanId)
		if err != nil {
			return err
		}
		if err := applySubscriptionUsageDelta(plan, sub, modelName, amount); err != nil {
			return err
		}
		if err := tx.Save(sub).Error; err != nil {
			return err
		}
		account, err := billingdomain.EnsureBillingAccountTx(tx, billingdomain.EnsureAccountParams{
			AccountType: "subscription", OwnerType: "user_subscription", OwnerID: int64(subscriptionID), QuotaUnit: "quota",
		})
		if err != nil {
			return err
		}
		_, err = billingdomain.CreateReservationTx(tx, billingdomain.CreateReservationParams{
			AccountID: account.AccountID, RequestID: requestID, ReservedAmount: amount,
			IdempotencyKey: fmt.Sprintf("subscription:%s:reserve-extra:%d", requestID, amount),
		})
		return err
	})
}

// RefundSubscriptionPreConsume refunds a previous subscription pre-consume idempotently.
func RefundSubscriptionPreConsume(requestID string) error {
	if strings.TrimSpace(requestID) == "" {
		return errors.New("requestId is empty")
	}

	return platformdb.DB.Transaction(func(tx *gorm.DB) error {
		record := &commerceschema.SubscriptionPreConsumeRecord{}
		if err := tx.Set("gorm:query_option", "FOR UPDATE").
			Where("request_id = ?", requestID).
			First(record).Error; err != nil {
			return err
		}
		if record.Status == "refunded" {
			return nil
		}
		if record.PreConsumed <= 0 {
			record.Status = "refunded"
			return tx.Save(record).Error
		}
		releasedAmount, err := releaseSubscriptionReservationTx(tx, record)
		if err != nil {
			return err
		}
		if err := postConsumeUserSubscriptionUsageDeltaTx(tx, record.UserSubscriptionId, record.ModelName, -releasedAmount); err != nil {
			return err
		}
		record.Status = "refunded"
		return tx.Save(record).Error
	})
}

// SettleSubscriptionReservation atomically settles the ledger reservation and updates
// the subscription usage projection to the confirmed upstream usage.
func SettleSubscriptionReservation(requestID string, subscriptionID int, modelName string, actualAmount int64) error {
	if strings.TrimSpace(requestID) == "" {
		return errors.New("requestId is empty")
	}
	if subscriptionID <= 0 {
		return errors.New("invalid userSubscriptionId")
	}
	if actualAmount < 0 {
		return errors.New("actual amount cannot be negative")
	}

	return platformdb.DB.Transaction(func(tx *gorm.DB) error {
		record := &commerceschema.SubscriptionPreConsumeRecord{}
		if err := tx.Set("gorm:query_option", "FOR UPDATE").Where("request_id = ?", requestID).First(record).Error; err != nil {
			return err
		}
		if record.UserSubscriptionId != subscriptionID {
			return errors.New("subscription reservation ownership mismatch")
		}
		if record.Status == "refunded" {
			return errors.New("subscription pre-consume already refunded")
		}
		return settleSubscriptionReservationTx(tx, record, actualAmount)
	})
}

func settleSubscriptionReservationTx(tx *gorm.DB, record *commerceschema.SubscriptionPreConsumeRecord, actualAmount int64) error {
	reservations, err := findOpenSubscriptionReservationsTx(tx, record.RequestId)
	if err != nil {
		return err
	}
	if len(reservations) == 0 {
		var settledCount int64
		if err := tx.Model(&billingschema.BillingReservation{}).
			Where("request_id = ? AND status = ?", record.RequestId, billingschema.BillingReservationStatusSettled).
			Count(&settledCount).Error; err != nil {
			return err
		}
		if settledCount > 0 {
			return nil
		}
		return errors.New("subscription ledger reservation is missing")
	}

	remaining := actualAmount
	reservedTotal := int64(0)
	for index, reservation := range reservations {
		reservedTotal += reservation.ReservedAmount
		settledAmount := reservation.ReservedAmount
		if remaining < settledAmount {
			settledAmount = remaining
		}
		if index == len(reservations)-1 && remaining > settledAmount {
			settledAmount = remaining
		}
		if _, err := billingdomain.SettleReservationTx(tx, billingdomain.SettleReservationParams{
			ReservationID:   reservation.ReservationID,
			UsageEvidenceID: record.RequestId,
			ActualAmount:    settledAmount,
			IdempotencyKey:  "subscription:" + record.RequestId + ":settle:" + reservation.ReservationID,
		}); err != nil {
			return err
		}
		remaining -= settledAmount
		if remaining < 0 {
			remaining = 0
		}
	}
	if remaining > 0 {
		return errors.New("subscription ledger reservation is insufficient")
	}
	if actualAmount != reservedTotal {
		return postConsumeUserSubscriptionUsageDeltaTx(tx, record.UserSubscriptionId, record.ModelName, actualAmount-reservedTotal)
	}
	return nil
}

func releaseSubscriptionReservationTx(tx *gorm.DB, record *commerceschema.SubscriptionPreConsumeRecord) (int64, error) {
	reservations, err := findOpenSubscriptionReservationsTx(tx, record.RequestId)
	if err != nil {
		return 0, err
	}
	if len(reservations) == 0 {
		return 0, errors.New("subscription ledger reservation is missing")
	}
	releasedAmount := int64(0)
	for _, reservation := range reservations {
		releasedAmount += reservation.ReservedAmount
		if _, err := billingdomain.ReleaseReservationTx(tx, billingdomain.ReleaseReservationParams{
			ReservationID:  reservation.ReservationID,
			IdempotencyKey: "subscription:" + record.RequestId + ":release:" + reservation.ReservationID,
			ReasonCode:     "relay_failed_before_settlement",
		}); err != nil {
			return 0, err
		}
	}
	return releasedAmount, nil
}

func findSubscriptionReservationTx(tx *gorm.DB, requestID string) (*billingschema.BillingReservation, error) {
	var reservation billingschema.BillingReservation
	err := tx.Where("idempotency_key = ?", "subscription:"+requestID+":reserve").First(&reservation).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &reservation, nil
}

func findOpenSubscriptionReservationsTx(tx *gorm.DB, requestID string) ([]billingschema.BillingReservation, error) {
	var reservations []billingschema.BillingReservation
	if err := tx.Where("request_id = ? AND status = ?", requestID, billingschema.BillingReservationStatusOpen).
		Order("created_at asc, reservation_id asc").Find(&reservations).Error; err != nil {
		return nil, err
	}
	return reservations, nil
}

// PostConsumeUserSubscriptionDelta updates total subscription usage without model-specific usage.
func PostConsumeUserSubscriptionDelta(userSubscriptionID int, delta int64) error {
	return PostConsumeUserSubscriptionUsageDelta(userSubscriptionID, "", delta)
}

// PostConsumeUserSubscriptionUsageDelta applies a usage delta to a subscription.
func PostConsumeUserSubscriptionUsageDelta(userSubscriptionID int, modelName string, delta int64) error {
	if userSubscriptionID <= 0 {
		return errors.New("invalid userSubscriptionId")
	}
	if delta == 0 {
		return nil
	}
	return platformdb.DB.Transaction(func(tx *gorm.DB) error {
		return postConsumeUserSubscriptionUsageDeltaTx(tx, userSubscriptionID, modelName, delta)
	})
}

func postConsumeUserSubscriptionUsageDeltaTx(tx *gorm.DB, userSubscriptionID int, modelName string, delta int64) error {
	if tx == nil {
		return errors.New("tx is nil")
	}
	sub := &commerceschema.UserSubscription{}
	if err := tx.Set("gorm:query_option", "FOR UPDATE").
		Where("id = ?", userSubscriptionID).
		First(sub).Error; err != nil {
		return err
	}
	plan, err := getSubscriptionPlanRecordTx(tx, sub.PlanId)
	if err != nil {
		return err
	}
	if err := applySubscriptionUsageDelta(plan, sub, modelName, delta); err != nil {
		return err
	}
	return tx.Save(sub).Error
}
