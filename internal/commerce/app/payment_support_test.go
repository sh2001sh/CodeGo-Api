package app

import (
	"testing"

	commercestore "github.com/sh2001sh/CodeGo-Api/internal/commerce/paymentsettings"
	"github.com/stretchr/testify/require"
)

func confirmPaymentComplianceForTest(t *testing.T) {
	t.Helper()
	paymentSetting := commercestore.GetPaymentSetting()
	originalConfirmed := paymentSetting.ComplianceConfirmed
	originalTermsVersion := paymentSetting.ComplianceTermsVersion
	t.Cleanup(func() {
		paymentSetting.ComplianceConfirmed = originalConfirmed
		paymentSetting.ComplianceTermsVersion = originalTermsVersion
	})
	paymentSetting.ComplianceConfirmed = true
	paymentSetting.ComplianceTermsVersion = commercestore.CurrentComplianceTermsVersion
}

func TestStripeTopUpEnabledRequiresTopUpAndWebhookConfig(t *testing.T) {
	confirmPaymentComplianceForTest(t)
	originalAPISecret := commercestore.StripeApiSecret
	originalWebhookSecret := commercestore.StripeWebhookSecret
	originalPriceID := commercestore.StripePriceId
	t.Cleanup(func() {
		commercestore.StripeApiSecret = originalAPISecret
		commercestore.StripeWebhookSecret = originalWebhookSecret
		commercestore.StripePriceId = originalPriceID
	})

	commercestore.StripeWebhookSecret = ""
	commercestore.StripeApiSecret = "sk_test_123"
	commercestore.StripePriceId = "price_123"
	require.False(t, IsStripeTopUpEnabled())

	commercestore.StripeWebhookSecret = "whsec_test"
	require.True(t, IsStripeTopUpEnabled())

	commercestore.StripePriceId = ""
	require.False(t, IsStripeTopUpEnabled())
}

func TestCreemTopUpEnabledRequiresTopUpConfig(t *testing.T) {
	confirmPaymentComplianceForTest(t)
	originalAPIKey := commercestore.CreemApiKey
	originalProducts := commercestore.CreemProducts
	t.Cleanup(func() {
		commercestore.CreemApiKey = originalAPIKey
		commercestore.CreemProducts = originalProducts
	})

	commercestore.CreemApiKey = "creem_api_key"
	commercestore.CreemProducts = `[{"productId":"prod_123"}]`
	require.True(t, IsCreemTopUpEnabled())

	commercestore.CreemProducts = "[]"
	require.False(t, IsCreemTopUpEnabled())
}
