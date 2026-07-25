package app

import (
	commercedomain "github.com/sh2001sh/CodeGo-Api/internal/commerce/domain"
	commercestore "github.com/sh2001sh/CodeGo-Api/internal/commerce/paymentsettings"
	commerceschema "github.com/sh2001sh/CodeGo-Api/internal/commerce/schema"
	platformconfig "github.com/sh2001sh/CodeGo-Api/internal/platform/config"
	platformgeneral "github.com/sh2001sh/CodeGo-Api/internal/platform/general"
	platformruntime "github.com/sh2001sh/CodeGo-Api/internal/platform/runtime"
	"github.com/shopspring/decimal"
	"strconv"
	"strings"
	"sync"
)

func IsPaymentComplianceConfirmed() bool {
	return commercestore.IsPaymentComplianceConfirmed()
}

func IsStripeTopUpEnabled() bool {
	if !IsPaymentComplianceConfirmed() {
		return false
	}
	return strings.TrimSpace(commercestore.StripeApiSecret) != "" &&
		strings.TrimSpace(commercestore.StripeWebhookSecret) != "" &&
		strings.TrimSpace(commercestore.StripePriceId) != ""
}

func IsCreemTopUpEnabled() bool {
	if !IsPaymentComplianceConfirmed() {
		return false
	}
	products := strings.TrimSpace(commercestore.CreemProducts)
	return strings.TrimSpace(commercestore.CreemApiKey) != "" &&
		products != "" &&
		products != "[]"
}

func IsCreemWebhookEnabled() bool {
	return IsCreemTopUpEnabled() && strings.TrimSpace(commercestore.CreemWebhookSecret) != ""
}

func GetPayMoney(amount int64, group string) float64 {
	dAmount := decimal.NewFromInt(amount)
	if platformgeneral.GetQuotaDisplayType() == platformgeneral.QuotaDisplayTypeTokens {
		dAmount = dAmount.Div(decimal.NewFromFloat(platformruntime.QuotaPerUnit))
	}

	topupGroupRatio := commercedomain.GetTopupGroupRatio(group)
	if topupGroupRatio == 0 {
		topupGroupRatio = 1
	}

	discount := 1.0
	if ds, ok := commercestore.GetPaymentSetting().AmountDiscount[int(amount)]; ok && ds > 0 {
		discount = ds
	}

	return dAmount.
		Mul(decimal.NewFromFloat(commercestore.Price)).
		Mul(decimal.NewFromFloat(topupGroupRatio)).
		Mul(decimal.NewFromFloat(discount)).
		InexactFloat64()
}

func GetMinTopup() int64 {
	minTopup := commercestore.MinTopUp
	if platformgeneral.GetQuotaDisplayType() == platformgeneral.QuotaDisplayTypeTokens {
		minTopup = int(decimal.NewFromInt(int64(minTopup)).
			Mul(decimal.NewFromFloat(platformruntime.QuotaPerUnit)).
			IntPart())
	}
	return int64(minTopup)
}

func GetTopupMinAmount() int64 {
	return GetMinTopup()
}

func GetTopupPayMoney(amount int64, group string) float64 {
	return GetPayMoney(amount, group)
}

func NormalizeStoredTopupAmount(amount int64) int64 {
	if platformgeneral.GetQuotaDisplayType() != platformgeneral.QuotaDisplayTypeTokens {
		return amount
	}
	return decimal.NewFromInt(amount).
		Div(decimal.NewFromFloat(platformruntime.QuotaPerUnit)).
		IntPart()
}

var orderLocks sync.Map
var createLock sync.Mutex

type refCountedMutex struct {
	mu       sync.Mutex
	refCount int
}

func LockOrder(tradeNo string) {
	createLock.Lock()
	var rcm *refCountedMutex
	if value, ok := orderLocks.Load(tradeNo); ok {
		rcm = value.(*refCountedMutex)
	} else {
		rcm = &refCountedMutex{}
		orderLocks.Store(tradeNo, rcm)
	}
	rcm.refCount++
	createLock.Unlock()
	rcm.mu.Lock()
}

func UnlockOrder(tradeNo string) {
	value, ok := orderLocks.Load(tradeNo)
	if !ok {
		return
	}
	rcm := value.(*refCountedMutex)
	rcm.mu.Unlock()

	createLock.Lock()
	rcm.refCount--
	if rcm.refCount == 0 {
		orderLocks.Delete(tradeNo)
	}
	createLock.Unlock()
}

func BuildStripePayMethod() map[string]string {
	return map[string]string{
		"name":      "Stripe",
		"type":      commerceschema.PaymentMethodStripe,
		"color":     "rgba(var(--semi-purple-5), 1)",
		"min_topup": strconv.Itoa(commercestore.StripeMinTopUp),
	}
}

func BuildPaymentReturnPath(suffix string) string {
	base := strings.TrimRight(platformconfig.ServerAddress, "/")
	return base + platformconfig.ThemeAwarePath(suffix)
}

func GetStripeMinTopup() int64 {
	minTopup := commercestore.StripeMinTopUp
	if platformgeneral.GetQuotaDisplayType() == platformgeneral.QuotaDisplayTypeTokens {
		minTopup = minTopup * int(platformruntime.QuotaPerUnit)
	}
	return int64(minTopup)
}

func GetStripePayMoney(amount float64, group string) float64 {
	originalAmount := amount
	if platformgeneral.GetQuotaDisplayType() == platformgeneral.QuotaDisplayTypeTokens {
		amount = amount / platformruntime.QuotaPerUnit
	}
	topupGroupRatio := commercedomain.GetTopupGroupRatio(group)
	if topupGroupRatio == 0 {
		topupGroupRatio = 1
	}
	discount := 1.0
	if ds, ok := commercestore.GetPaymentSetting().AmountDiscount[int(originalAmount)]; ok && ds > 0 {
		discount = ds
	}
	return amount * commercestore.StripeUnitPrice * topupGroupRatio * discount
}
