package app

import (
	"context"
	"errors"
	"fmt"
	"github.com/sh2001sh/CodeGo-Api/constant"
	commercedomain "github.com/sh2001sh/CodeGo-Api/internal/commerce/domain"
	commercestore "github.com/sh2001sh/CodeGo-Api/internal/commerce/paymentsettings"
	commerceschema "github.com/sh2001sh/CodeGo-Api/internal/commerce/schema"
	platformgeneral "github.com/sh2001sh/CodeGo-Api/internal/platform/general"
	platformsecurity "github.com/sh2001sh/CodeGo-Api/internal/platform/security"
	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/checkout/session"
	"github.com/thanhpk/randstr"
	"net/url"
	"strings"
	"time"
)

type SubscriptionPurchaseFields struct {
	PlanID         int    `json:"plan_id"`
	PaymentMethod  string `json:"payment_method"`
	PurchaseType   string `json:"purchase_type"`
	SubscriptionID int    `json:"subscription_id"`
	Quota          int64  `json:"quota"`
}

type SubscriptionStripePayRequest struct {
	PlanID         int    `json:"plan_id"`
	PurchaseType   string `json:"purchase_type"`
	SubscriptionID int    `json:"subscription_id"`
	Quota          int64  `json:"quota"`
}

type SubscriptionCreemPayRequest struct {
	PlanID         int    `json:"plan_id"`
	PurchaseType   string `json:"purchase_type"`
	SubscriptionID int    `json:"subscription_id"`
	Quota          int64  `json:"quota"`
}

type SubscriptionCheckoutPayload struct {
	OrderID   string  `json:"order_id"`
	AmountDue float64 `json:"amount_due"`
	Action    string  `json:"action"`
}

type SubscriptionStripeCheckoutPayload struct {
	SubscriptionCheckoutPayload
	PayLink string `json:"pay_link"`
}

type SubscriptionCreemCheckoutPayload struct {
	SubscriptionCheckoutPayload
	CheckoutURL string `json:"checkout_url"`
}

func NormalizeSubscriptionPurchaseFields(_ int, req SubscriptionPurchaseFields) (string, error) {
	return commercedomain.NormalizeSubscriptionPurchaseType(req.PurchaseType), nil
}

func ApplySubscriptionPurchaseFields(order *commerceschema.SubscriptionOrder, purchaseType string) {
	if order == nil {
		return
	}
	order.PurchaseType = commercedomain.NormalizeSubscriptionPurchaseType(purchaseType)
}

func PrepareSubscriptionPurchase(userID int, req SubscriptionPurchaseFields) (*commerceschema.SubscriptionPlan, *commercedomain.SubscriptionPurchasePreview, string, error) {
	plan, err := GetSubscriptionPlanByID(req.PlanID)
	if err != nil {
		return nil, nil, "", err
	}
	if !plan.Enabled {
		return nil, nil, "", errors.New("plan is disabled")
	}
	if plan.InternalOnly {
		return nil, nil, "", errors.New("internal plan cannot be purchased")
	}

	purchaseType, err := NormalizeSubscriptionPurchaseFields(userID, req)
	if err != nil {
		return nil, nil, "", err
	}
	preview, err := ResolveSubscriptionPurchasePreview(userID, plan)
	if err != nil {
		return nil, nil, "", err
	}
	if preview.Action == commerceschema.SubscriptionPurchaseActionDisabled {
		return nil, nil, "", errors.New(preview.DisabledReason)
	}
	if plan.MaxPurchasePerUser > 0 {
		count, err := CountUserSubscriptionsByPlan(userID, plan.Id)
		if err != nil {
			return nil, nil, "", err
		}
		if count >= int64(plan.MaxPurchasePerUser) {
			return nil, nil, "", errors.New("purchase limit reached")
		}
	}
	return plan, preview, purchaseType, nil
}

func CreateSubscriptionStripePayment(userID int, req SubscriptionStripePayRequest) (*SubscriptionStripeCheckoutPayload, error) {
	plan, preview, purchaseType, err := PrepareSubscriptionPurchase(userID, SubscriptionPurchaseFields{
		PlanID:       req.PlanID,
		PurchaseType: req.PurchaseType,
	})
	if err != nil {
		return nil, err
	}
	if preview.Action == commerceschema.SubscriptionPurchaseActionUpgrade && preview.AmountDue != plan.PriceAmount {
		return nil, errors.New("subscription upgrades are currently supported via Stripe or Creem")
	}
	if !strings.HasPrefix(commercestore.StripeApiSecret, "sk_") && !strings.HasPrefix(commercestore.StripeApiSecret, "rk_") {
		return nil, errors.New("Stripe is not configured correctly")
	}
	if commercestore.StripeWebhookSecret == "" {
		return nil, errors.New("Stripe webhook is not configured")
	}

	user, err := loadCommerceUserByID(userID, false)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, errors.New("user not found")
	}

	reference := fmt.Sprintf("sub-stripe-ref-%d-%d-%s", user.Id, time.Now().UnixMilli(), randstr.String(4))
	referenceID := "sub_ref_" + platformsecurity.Sha1([]byte(reference))
	order := &commerceschema.SubscriptionOrder{
		UserId:          userID,
		PlanId:          plan.Id,
		Money:           preview.AmountDue,
		TradeNo:         referenceID,
		PaymentMethod:   commerceschema.PaymentMethodStripe,
		PaymentProvider: commerceschema.PaymentProviderStripe,
		CreateTime:      time.Now().Unix(),
		Status:          constant.TopUpStatusPending,
	}
	ApplySubscriptionPurchaseFields(order, purchaseType)
	if err := CreatePendingSubscriptionOrder(order, preview.BaseAmountDue); err != nil {
		return nil, errors.New("failed to create order")
	}
	payLink, err := genStripeSubscriptionLink(referenceID, user.StripeCustomer, user.Email, plan.Title, order.Money)
	if err != nil {
		_ = ExpireSubscriptionOrder(referenceID, commerceschema.PaymentProviderStripe)
		return nil, errors.New("failed to create payment")
	}
	return &SubscriptionStripeCheckoutPayload{
		SubscriptionCheckoutPayload: SubscriptionCheckoutPayload{
			OrderID:   referenceID,
			AmountDue: order.Money,
			Action:    preview.Action,
		},
		PayLink: payLink,
	}, nil
}

func CreateSubscriptionCreemPayment(userID int, req SubscriptionCreemPayRequest) (*SubscriptionCreemCheckoutPayload, error) {
	plan, preview, purchaseType, err := PrepareSubscriptionPurchase(userID, SubscriptionPurchaseFields{
		PlanID:       req.PlanID,
		PurchaseType: req.PurchaseType,
	})
	if err != nil {
		return nil, err
	}
	if preview.Action == commerceschema.SubscriptionPurchaseActionUpgrade && preview.AmountDue != plan.PriceAmount {
		return nil, errors.New("subscription upgrades are currently supported via Stripe or Creem")
	}
	if plan.CreemProductId == "" {
		return nil, errors.New("Creem product is not configured for this plan")
	}
	if commercestore.CreemWebhookSecret == "" && !commercestore.CreemTestMode {
		return nil, errors.New("Creem webhook is not configured")
	}

	user, err := loadCommerceUserByID(userID, false)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, errors.New("user not found")
	}

	reference := "sub-creem-ref-" + randstr.String(6)
	referenceID := "sub_ref_" + platformsecurity.Sha1([]byte(reference+time.Now().String()+user.Username))
	order := &commerceschema.SubscriptionOrder{
		UserId:          userID,
		PlanId:          plan.Id,
		Money:           preview.AmountDue,
		TradeNo:         referenceID,
		PaymentMethod:   commerceschema.PaymentMethodCreem,
		PaymentProvider: commerceschema.PaymentProviderCreem,
		CreateTime:      time.Now().Unix(),
		Status:          constant.TopUpStatusPending,
	}
	ApplySubscriptionPurchaseFields(order, purchaseType)
	if err := CreatePendingSubscriptionOrder(order, preview.BaseAmountDue); err != nil {
		return nil, errors.New("failed to create order")
	}

	currency := "USD"
	switch platformgeneral.GetSetting().QuotaDisplayType {
	case platformgeneral.QuotaDisplayTypeCNY:
		currency = "CNY"
	case platformgeneral.QuotaDisplayTypeUSD:
		currency = "USD"
	}
	product := &CreemProduct{
		ProductID: plan.CreemProductId,
		Name:      plan.Title,
		Price:     order.Money,
		Currency:  currency,
		Quota:     0,
	}
	checkoutURL, err := GenCreemLink(context.Background(), referenceID, product, user.Email, user.Username, order.Money)
	if err != nil {
		_ = ExpireSubscriptionOrder(referenceID, commerceschema.PaymentProviderCreem)
		return nil, errors.New("failed to create payment")
	}
	return &SubscriptionCreemCheckoutPayload{
		SubscriptionCheckoutPayload: SubscriptionCheckoutPayload{
			OrderID:   referenceID,
			AmountDue: order.Money,
			Action:    preview.Action,
		},
		CheckoutURL: checkoutURL,
	}, nil
}

func CollectSubscriptionPaymentParams(rawURLValues url.Values) map[string]string {
	params := make(map[string]string, len(rawURLValues))
	for key := range rawURLValues {
		params[key] = rawURLValues.Get(key)
	}
	return params
}

func genStripeSubscriptionLink(referenceID string, customerID string, email string, productName string, amountDue float64) (string, error) {
	stripe.Key = commercestore.StripeApiSecret
	unitAmount := StripeMoneyToMinorUnits(amountDue)
	if unitAmount < 1 {
		return "", fmt.Errorf("invalid stripe amount")
	}
	params := &stripe.CheckoutSessionParams{
		ClientReferenceID: stripe.String(referenceID),
		SuccessURL:        stripe.String(BuildPaymentReturnPath("/packages")),
		CancelURL:         stripe.String(BuildPaymentReturnPath("/packages")),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Quantity: stripe.Int64(1),
				PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
					Currency:   stripe.String("usd"),
					UnitAmount: stripe.Int64(unitAmount),
					ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
						Name: stripe.String(productName),
					},
				},
			},
		},
		Mode: stripe.String(string(stripe.CheckoutSessionModePayment)),
	}
	if customerID == "" {
		if email != "" {
			params.CustomerEmail = stripe.String(email)
		}
		params.CustomerCreation = stripe.String(string(stripe.CheckoutSessionCustomerCreationAlways))
	} else {
		params.Customer = stripe.String(customerID)
	}
	result, err := session.New(params)
	if err != nil {
		return "", err
	}
	return result.URL, nil
}
