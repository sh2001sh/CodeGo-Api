package app

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	commercestore "github.com/sh2001sh/CodeGo-Api/internal/commerce/paymentsettings"
	"github.com/sh2001sh/CodeGo-Api/internal/platform/logger"
	platformstore "github.com/sh2001sh/CodeGo-Api/internal/platform/store"
)

var ErrPaymentComplianceConfirmationRequired = errors.New("请确认合规声明")

// PaymentComplianceConfirmation is returned after the compliance terms are confirmed.
type PaymentComplianceConfirmation struct {
	Confirmed    bool   `json:"confirmed"`
	TermsVersion string `json:"terms_version"`
	ConfirmedAt  int64  `json:"confirmed_at"`
	ConfirmedBy  int    `json:"confirmed_by"`
}

// ConfirmPaymentCompliance persists the payment compliance confirmation metadata.
func ConfirmPaymentCompliance(ctx context.Context, userID int, clientIP string) (*PaymentComplianceConfirmation, error) {
	now := time.Now().Unix()
	result := &PaymentComplianceConfirmation{
		Confirmed:    true,
		TermsVersion: commercestore.CurrentComplianceTermsVersion,
		ConfirmedAt:  now,
		ConfirmedBy:  userID,
	}

	updates := map[string]string{
		"payment_setting.compliance_confirmed":     "true",
		"payment_setting.compliance_terms_version": result.TermsVersion,
		"payment_setting.compliance_confirmed_at":  strconv.FormatInt(result.ConfirmedAt, 10),
		"payment_setting.compliance_confirmed_by":  strconv.Itoa(result.ConfirmedBy),
		"payment_setting.compliance_confirmed_ip":  clientIP,
	}
	for key, value := range updates {
		if err := platformstore.UpdateOption(key, value); err != nil {
			return nil, err
		}
	}

	logger.LogInfo(ctx, fmt.Sprintf(
		"payment compliance confirmed user_id=%d ip=%s terms_version=%s confirmed_at=%d",
		userID,
		clientIP,
		result.TermsVersion,
		result.ConfirmedAt,
	))
	return result, nil
}
