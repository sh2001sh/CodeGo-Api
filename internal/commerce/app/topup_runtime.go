package app

import (
	"errors"
	"fmt"
	"github.com/sh2001sh/CodeGo-Api/constant"
	auditapp "github.com/sh2001sh/CodeGo-Api/internal/audit/app"
	billingapp "github.com/sh2001sh/CodeGo-Api/internal/billing/app"
	commerceschema "github.com/sh2001sh/CodeGo-Api/internal/commerce/schema"
	identityschema "github.com/sh2001sh/CodeGo-Api/internal/identity/schema"
	identitystore "github.com/sh2001sh/CodeGo-Api/internal/identity/store"
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
	"github.com/sh2001sh/CodeGo-Api/internal/platform/logger"
	platformobservability "github.com/sh2001sh/CodeGo-Api/internal/platform/observability"
	platformruntime "github.com/sh2001sh/CodeGo-Api/internal/platform/runtime"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

func calculateTopUpQuotaToAdd(topUp *commerceschema.TopUp) int {
	if topUp == nil {
		return 0
	}
	dQuotaPerUnit := decimal.NewFromFloat(platformruntime.QuotaPerUnit)
	switch topUp.PaymentProvider {
	case commerceschema.PaymentProviderCreem:
		return int(topUp.Amount)
	default:
		return int(decimal.NewFromInt(topUp.Amount).Mul(dQuotaPerUnit).IntPart())
	}
}

func creditTopUpQuotaTx(tx *gorm.DB, topUp *commerceschema.TopUp, quotaToAdd int, customerID string, customerEmail string) error {
	if topUp == nil || topUp.UserId <= 0 {
		return errors.New("invalid topup")
	}
	if quotaToAdd <= 0 {
		return errors.New("invalid topup quota")
	}

	updateFields := map[string]interface{}{}
	if err := billingapp.CreditWalletQuotaTx(tx, topUp.UserId, quotaToAdd, fmt.Sprintf("topup:%s:wallet", topUp.TradeNo), "topup_credit"); err != nil {
		return err
	}
	if topUp.PaymentProvider == commerceschema.PaymentProviderStripe && customerID != "" {
		updateFields["stripe_customer"] = customerID
	}
	if topUp.PaymentProvider == commerceschema.PaymentProviderCreem && customerEmail != "" {
		var user identityschema.User
		if err := tx.Where("id = ?", topUp.UserId).First(&user).Error; err != nil {
			return err
		}
		if user.Email == "" {
			updateFields["email"] = customerEmail
		}
	}
	if len(updateFields) == 0 {
		return nil
	}
	return tx.Model(&identityschema.User{}).Where("id = ?", topUp.UserId).Updates(updateFields).Error
}

func invalidateTopUpUserCache(userID int) {
	if userID <= 0 {
		return
	}
	_ = identitystore.InvalidateUserCache(userID)
}

func topUpWalletLogLabel(topUp *commerceschema.TopUp) string {
	return "额度"
}

// CompleteTopUpByTradeNo completes a pending top-up order and credits quota to the user.
func CompleteTopUpByTradeNo(tradeNo string, expectedPaymentProvider string, actualPaymentMethod string, customerID string, customerEmail string) (*commerceschema.TopUp, int, error) {
	if tradeNo == "" {
		return nil, 0, errors.New("missing trade no")
	}

	var completedTopUp *commerceschema.TopUp
	var quotaToAdd int
	err := platformdb.DB.Transaction(func(tx *gorm.DB) error {
		topUp := &commerceschema.TopUp{}
		if err := tx.Set("gorm:query_option", "FOR UPDATE").Where(topUpTradeNoColumn()+" = ?", tradeNo).First(topUp).Error; err != nil {
			return commerceschema.ErrTopUpNotFound
		}
		if expectedPaymentProvider != "" && topUp.PaymentProvider != expectedPaymentProvider {
			return commerceschema.ErrPaymentMethodMismatch
		}
		if topUp.Status == constant.TopUpStatusSuccess {
			completedTopUp = topUp
			return nil
		}
		if topUp.Status != constant.TopUpStatusPending {
			return commerceschema.ErrTopUpStatusInvalid
		}
		if actualPaymentMethod != "" {
			topUp.PaymentMethod = actualPaymentMethod
		}

		quotaToAdd = calculateTopUpQuotaToAdd(topUp)
		if quotaToAdd <= 0 {
			return errors.New("invalid topup quota")
		}

		topUp.CompleteTime = platformruntime.GetTimestamp()
		topUp.Status = constant.TopUpStatusSuccess
		if err := tx.Save(topUp).Error; err != nil {
			return err
		}
		if err := creditTopUpQuotaTx(tx, topUp, quotaToAdd, customerID, customerEmail); err != nil {
			return err
		}
		completedTopUp = topUp
		return nil
	})
	if err == nil && completedTopUp != nil {
		invalidateTopUpUserCache(completedTopUp.UserId)
	}
	return completedTopUp, quotaToAdd, err
}

// ManualCompleteTopUp force-completes a pending top-up order for an administrator.
func ManualCompleteTopUp(tradeNo string, callerIP string) error {
	if tradeNo == "" {
		return errors.New("未提供订单号")
	}

	var userID int
	var quotaToAdd int
	var payMoney float64
	var paymentMethod string
	err := platformdb.DB.Transaction(func(tx *gorm.DB) error {
		topUp := &commerceschema.TopUp{}
		if err := tx.Set("gorm:query_option", "FOR UPDATE").Where(topUpTradeNoColumn()+" = ?", tradeNo).First(topUp).Error; err != nil {
			return errors.New("充值订单不存在")
		}
		if topUp.Status == constant.TopUpStatusSuccess {
			userID = topUp.UserId
			payMoney = topUp.Money
			paymentMethod = topUp.PaymentMethod
			return nil
		}
		if topUp.Status != constant.TopUpStatusPending {
			return errors.New("订单状态不是待支付，无法补单")
		}

		quotaToAdd = calculateTopUpQuotaToAdd(topUp)
		if quotaToAdd <= 0 {
			return errors.New("无效的充值额度")
		}

		topUp.CompleteTime = platformruntime.GetTimestamp()
		topUp.Status = constant.TopUpStatusSuccess
		if err := tx.Save(topUp).Error; err != nil {
			return err
		}
		if err := creditTopUpQuotaTx(tx, topUp, quotaToAdd, "", ""); err != nil {
			return err
		}

		userID = topUp.UserId
		payMoney = topUp.Money
		paymentMethod = topUp.PaymentMethod
		return nil
	})
	if err != nil {
		return err
	}

	invalidateTopUpUserCache(userID)
	auditapp.RecordTopupLog(userID, fmt.Sprintf("管理员补单成功，充值金额: %v，支付金额：%f", logger.FormatQuota(quotaToAdd), payMoney), callerIP, paymentMethod, "admin")
	return nil
}

// Recharge completes a Stripe top-up order after payment succeeds.
func Recharge(referenceID string, customerID string, callerIP string) error {
	if referenceID == "" {
		return errors.New("未提供支付单号")
	}

	topUp, quotaToAdd, err := CompleteTopUpByTradeNo(referenceID, commerceschema.PaymentProviderStripe, "", customerID, "")
	if err != nil {
		platformobservability.SysError("topup failed: " + err.Error())
		return errors.New("充值失败，请稍后重试")
	}

	auditapp.RecordTopupLog(topUp.UserId, fmt.Sprintf("Stripe topup success, wallet: %s, quota: %v, paid: %.2f", topUpWalletLogLabel(topUp), logger.FormatQuota(quotaToAdd), topUp.Money), callerIP, topUp.PaymentMethod, commerceschema.PaymentMethodStripe)
	return nil
}

// RechargeCreem completes a Creem top-up order after payment succeeds.
func RechargeCreem(referenceID string, customerEmail string, _ string, callerIP string) error {
	if referenceID == "" {
		return errors.New("未提供支付单号")
	}

	topUp, quotaToAdd, err := CompleteTopUpByTradeNo(referenceID, commerceschema.PaymentProviderCreem, "", "", customerEmail)
	if err != nil {
		platformobservability.SysError("creem topup failed: " + err.Error())
		return errors.New("充值失败，请稍后重试")
	}

	auditapp.RecordTopupLog(topUp.UserId, fmt.Sprintf("使用Creem充值成功，充值额度: %v，支付金额：%.2f", quotaToAdd, topUp.Money), callerIP, topUp.PaymentMethod, commerceschema.PaymentMethodCreem)
	return nil
}
