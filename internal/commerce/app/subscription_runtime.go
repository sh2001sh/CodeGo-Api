package app

import (
	"context"
	"errors"
	"fmt"
	"github.com/sh2001sh/CodeGo-Api/constant"
	auditapp "github.com/sh2001sh/CodeGo-Api/internal/audit/app"
	auditschema "github.com/sh2001sh/CodeGo-Api/internal/audit/schema"
	commercedomain "github.com/sh2001sh/CodeGo-Api/internal/commerce/domain"
	commerceschema "github.com/sh2001sh/CodeGo-Api/internal/commerce/schema"
	identitystore "github.com/sh2001sh/CodeGo-Api/internal/identity/store"
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
	"math"
	"strings"

	// ResolveSubscriptionPurchasePreview computes the current purchase action and payable amount.
	platformobservability "github.com/sh2001sh/CodeGo-Api/internal/platform/observability"
	platformruntime "github.com/sh2001sh/CodeGo-Api/internal/platform/runtime"

	"gorm.io/gorm"
)

func ResolveSubscriptionPurchasePreview(userID int, targetPlan *commerceschema.SubscriptionPlan) (*commercedomain.SubscriptionPurchasePreview, error) {
	return resolveSubscriptionPurchasePreviewTx(nil, userID, targetPlan)
}

// CreatePendingSubscriptionOrder creates a pending subscription order.
func CreatePendingSubscriptionOrder(order *commerceschema.SubscriptionOrder, baseMoney float64) error {
	if order == nil {
		return errors.New("order is nil")
	}
	if order.UserId <= 0 || strings.TrimSpace(order.TradeNo) == "" {
		return errors.New("invalid subscription order")
	}
	if baseMoney <= 0 {
		baseMoney = order.Money
	}

	err := platformdb.DB.Transaction(func(tx *gorm.DB) error {
		order.FulfillmentStatus = commerceschema.SubscriptionOrderFulfillmentPending
		order.Money = math.Round(baseMoney*100) / 100
		return tx.Create(order).Error
	})
	return err
}

// CompleteSubscriptionOrder completes a pending subscription order idempotently.
func CompleteSubscriptionOrder(tradeNo string, providerPayload string, expectedPaymentProvider string, actualPaymentMethod string) error {
	if strings.TrimSpace(tradeNo) == "" {
		return errors.New("tradeNo is empty")
	}

	var completedOrder *commerceschema.SubscriptionOrder
	err := platformdb.DB.Transaction(func(tx *gorm.DB) error {
		order := &commerceschema.SubscriptionOrder{}
		if err := tx.Set("gorm:query_option", "FOR UPDATE").Where(subscriptionTradeNoColumn()+" = ?", tradeNo).First(order).Error; err != nil {
			return commerceschema.ErrSubscriptionOrderNotFound
		}
		if expectedPaymentProvider != "" && order.PaymentProvider != expectedPaymentProvider {
			return commerceschema.ErrPaymentMethodMismatch
		}
		if order.Status == constant.TopUpStatusSuccess {
			if order.FulfillmentStatus != commerceschema.SubscriptionOrderFulfillmentCompleted {
				completedCopy := *order
				completedOrder = &completedCopy
			}
			return nil
		}
		if order.Status != constant.TopUpStatusPending {
			return commerceschema.ErrSubscriptionOrderStatusInvalid
		}

		order.Status = constant.TopUpStatusSuccess
		order.FulfillmentStatus = commerceschema.SubscriptionOrderFulfillmentPending
		order.CompleteTime = platformruntime.GetTimestamp()
		if providerPayload != "" {
			order.ProviderPayload = providerPayload
		}
		if actualPaymentMethod != "" && order.PaymentMethod != actualPaymentMethod {
			order.PaymentMethod = actualPaymentMethod
		}
		if err := tx.Save(order).Error; err != nil {
			return err
		}

		completedCopy := *order
		completedOrder = &completedCopy
		return nil
	})
	if err != nil {
		return err
	}

	if completedOrder != nil {
		if err := StartOrderFulfillmentWorkflow(context.Background(), completedOrder); err != nil {
			platformobservability.SysLog("start order fulfillment workflow: " + err.Error())
		}
	}
	return nil
}

// FulfillPaidSubscriptionOrder grants all purchased benefits exactly once. It is
// invoked by OrderFulfillmentWorkflow after a payment callback commits success.
func FulfillPaidSubscriptionOrder(tradeNo string) error {
	if strings.TrimSpace(tradeNo) == "" {
		return errors.New("tradeNo is empty")
	}

	var logUserID int
	var logPlanTitle string
	var logMoney float64
	var logPaymentMethod string
	var upgradeGroup string
	err := platformdb.DB.Transaction(func(tx *gorm.DB) error {
		order := &commerceschema.SubscriptionOrder{}
		if err := tx.Set("gorm:query_option", "FOR UPDATE").Where(subscriptionTradeNoColumn()+" = ?", tradeNo).First(order).Error; err != nil {
			return commerceschema.ErrSubscriptionOrderNotFound
		}
		if order.Status != constant.TopUpStatusSuccess {
			return commerceschema.ErrSubscriptionOrderStatusInvalid
		}
		if order.FulfillmentStatus == commerceschema.SubscriptionOrderFulfillmentCompleted {
			return nil
		}
		if order.PurchaseType == "subscription_booster" {
			return errors.New("legacy subscription booster orders are no longer fulfillable")
		}
		if order.PurchaseType == commerceschema.SubscriptionPurchaseTypeFuel {
			if err := fulfillSubscriptionFuelTx(tx, order); err != nil {
				return err
			}
			order.FulfillmentStatus = commerceschema.SubscriptionOrderFulfillmentCompleted
			return tx.Save(order).Error
		}
		plan, err := getSubscriptionPlanRecordTx(tx, order.PlanId)
		if err != nil {
			return err
		}
		sub, preview, err := ApplySubscriptionPurchaseTx(tx, order.UserId, plan, "order")
		if err != nil {
			return err
		}
		if preview != nil {
			upgradeGroup = strings.TrimSpace(sub.UpgradeGroup)
		}
		if err := upsertSubscriptionTopUpTx(tx, order); err != nil {
			return err
		}
		order.FulfillmentStatus = commerceschema.SubscriptionOrderFulfillmentCompleted
		if err := tx.Save(order).Error; err != nil {
			return err
		}
		logUserID = order.UserId
		logPlanTitle = plan.Title
		logMoney = order.Money
		logPaymentMethod = order.PaymentMethod
		return nil
	})
	if err != nil {
		return err
	}
	if upgradeGroup != "" && logUserID > 0 {
		_ = identitystore.UpdateUserGroupCache(logUserID, upgradeGroup)
	}
	if logUserID > 0 {
		auditapp.RecordLog(logUserID, auditschema.LogTypeTopup, fmt.Sprintf("订阅购买成功，套餐: %s，支付金额: %.2f，支付方式: %s", logPlanTitle, logMoney, logPaymentMethod))
	}
	return nil
}

// ExpireSubscriptionOrder marks a pending subscription order as expired.
func ExpireSubscriptionOrder(tradeNo string, expectedPaymentProvider string) error {
	if strings.TrimSpace(tradeNo) == "" {
		return errors.New("tradeNo is empty")
	}

	return platformdb.DB.Transaction(func(tx *gorm.DB) error {
		order := &commerceschema.SubscriptionOrder{}
		if err := tx.Set("gorm:query_option", "FOR UPDATE").Where(subscriptionTradeNoColumn()+" = ?", tradeNo).First(order).Error; err != nil {
			return commerceschema.ErrSubscriptionOrderNotFound
		}
		if expectedPaymentProvider != "" && order.PaymentProvider != expectedPaymentProvider {
			return commerceschema.ErrPaymentMethodMismatch
		}
		if order.Status != constant.TopUpStatusPending {
			return nil
		}

		order.Status = constant.TopUpStatusExpired
		order.CompleteTime = platformruntime.GetTimestamp()
		if err := tx.Save(order).Error; err != nil {
			return err
		}
		return nil
	})
}

// CancelPendingSubscriptionOrder expires a user's unpaid subscription order.
func CancelPendingSubscriptionOrder(userID int, tradeNo string) error {
	if userID <= 0 || strings.TrimSpace(tradeNo) == "" {
		return errors.New("invalid subscription order cancellation")
	}
	return platformdb.DB.Transaction(func(tx *gorm.DB) error {
		order := &commerceschema.SubscriptionOrder{}
		if err := tx.Set("gorm:query_option", "FOR UPDATE").Where(subscriptionTradeNoColumn()+" = ? AND user_id = ?", tradeNo, userID).First(order).Error; err != nil {
			return commerceschema.ErrSubscriptionOrderNotFound
		}
		if order.Status != constant.TopUpStatusPending {
			return nil
		}
		order.Status = constant.TopUpStatusExpired
		order.CompleteTime = platformruntime.GetTimestamp()
		if err := tx.Save(order).Error; err != nil {
			return err
		}
		return nil
	})
}

func upsertSubscriptionTopUpTx(tx *gorm.DB, order *commerceschema.SubscriptionOrder) error {
	if tx == nil || order == nil {
		return errors.New("invalid subscription order")
	}

	now := platformruntime.GetTimestamp()
	topup := &commerceschema.TopUp{}
	if err := tx.Where("trade_no = ?", order.TradeNo).First(topup).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			topup = &commerceschema.TopUp{
				UserId:        order.UserId,
				Amount:        0,
				Money:         order.Money,
				TradeNo:       order.TradeNo,
				PaymentMethod: order.PaymentMethod,
				CreateTime:    order.CreateTime,
				CompleteTime:  now,
				Status:        constant.TopUpStatusSuccess,
			}
			return tx.Create(topup).Error
		}
		return err
	}

	topup.Money = order.Money
	if topup.PaymentMethod == "" {
		topup.PaymentMethod = order.PaymentMethod
	} else if topup.PaymentMethod != order.PaymentMethod {
		return commerceschema.ErrPaymentMethodMismatch
	}
	if topup.CreateTime == 0 {
		topup.CreateTime = order.CreateTime
	}
	topup.CompleteTime = now
	topup.Status = constant.TopUpStatusSuccess
	return tx.Save(topup).Error
}

func subscriptionTradeNoColumn() string {
	if platformdb.UsingPostgreSQL {
		return `"trade_no"`
	}
	return "`trade_no`"
}
