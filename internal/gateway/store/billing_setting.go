package store

import (
	"fmt"

	"github.com/samber/lo"
	"github.com/sh2001sh/CodeGo-Api/internal/billing/domain/billingexpr"
	"github.com/sh2001sh/CodeGo-Api/setting/config"
)

const (
	BillingModeRatio      = "ratio"
	BillingModeTieredExpr = "tiered_expr"
	BillingModeField      = "billing_mode"
	BillingExprField      = "billing_expr"
)

type BillingSetting struct {
	BillingMode map[string]string `json:"billing_mode"`
	BillingExpr map[string]string `json:"billing_expr"`
}

var defaultBillingMode = map[string]string{
	"gpt-5.6-sol":   BillingModeTieredExpr,
	"gpt-5.6-terra": BillingModeTieredExpr,
	"gpt-5.6-luna":  BillingModeTieredExpr,
	"gpt-5.5":       BillingModeTieredExpr,
	"gpt-5.5-pro":   BillingModeTieredExpr,
	"gpt-5.4":       BillingModeTieredExpr,
	"gpt-5.4-mini":  BillingModeTieredExpr,
	"gpt-5.4-nano":  BillingModeTieredExpr,
	"gpt-5.4-pro":   BillingModeTieredExpr,
}

var defaultBillingExpr = map[string]string{
	"gpt-5.6-sol":   `len <= 272000 ? tier("short_context", p * 5 + c * 30 + cr * 0.5 + cc * 6.25) : tier("long_context", p * 10 + c * 45 + cr * 1 + cc * 12.5)`,
	"gpt-5.6-terra": `len <= 272000 ? tier("short_context", p * 2.5 + c * 15 + cr * 0.25 + cc * 3.125) : tier("long_context", p * 5 + c * 22.5 + cr * 0.5 + cc * 6.25)`,
	"gpt-5.6-luna":  `len <= 272000 ? tier("short_context", p * 1 + c * 6 + cr * 0.1 + cc * 1.25) : tier("long_context", p * 2 + c * 9 + cr * 0.2 + cc * 2.5)`,
	"gpt-5.5":       `len <= 272000 ? tier("short_context", p * 5 + c * 30 + cr * 0.5) : tier("long_context", p * 10 + c * 45 + cr * 1)`,
	"gpt-5.5-pro":   `len <= 272000 ? tier("short_context", p * 30 + c * 180) : tier("long_context", p * 60 + c * 270)`,
	"gpt-5.4":       `len <= 272000 ? tier("short_context", p * 2.5 + c * 15 + cr * 0.25) : tier("long_context", p * 5 + c * 22.5 + cr * 0.5)`,
	"gpt-5.4-mini":  `tier("short_context", p * 0.75 + c * 4.5 + cr * 0.075)`,
	"gpt-5.4-nano":  `tier("short_context", p * 0.2 + c * 1.25 + cr * 0.02)`,
	"gpt-5.4-pro":   `len <= 272000 ? tier("short_context", p * 30 + c * 180) : tier("long_context", p * 60 + c * 270)`,
}

var billingSetting = newDefaultBillingSetting()

func newDefaultBillingSetting() BillingSetting {
	return BillingSetting{
		BillingMode: lo.Assign(defaultBillingMode),
		BillingExpr: lo.Assign(defaultBillingExpr),
	}
}

// RestoreMissingDefaultBillingRules preserves explicit persisted overrides while
// backfilling built-in tiered rules absent from legacy configuration records.
func RestoreMissingDefaultBillingRules() {
	mergeMissingDefaultBillingRules(&billingSetting)
}

func mergeMissingDefaultBillingRules(setting *BillingSetting) {
	if setting.BillingMode == nil {
		setting.BillingMode = make(map[string]string)
	}
	if setting.BillingExpr == nil {
		setting.BillingExpr = make(map[string]string)
	}
	for model, mode := range defaultBillingMode {
		if _, ok := setting.BillingMode[model]; !ok {
			setting.BillingMode[model] = mode
		}
	}
	for model, expression := range defaultBillingExpr {
		if _, ok := setting.BillingExpr[model]; !ok {
			setting.BillingExpr[model] = expression
		}
	}
}

func init() {
	config.GlobalConfig.Register("billing_setting", &billingSetting)
}

func GetBillingMode(model string) string {
	model = FormatMatchingModelName(model)
	if mode, ok := billingSetting.BillingMode[model]; ok {
		return mode
	}
	return BillingModeRatio
}

func GetBillingExpr(model string) (string, bool) {
	model = FormatMatchingModelName(model)
	expr, ok := billingSetting.BillingExpr[model]
	return expr, ok
}

func GetBillingModeCopy() map[string]string {
	return lo.Assign(billingSetting.BillingMode)
}

func GetBillingExprCopy() map[string]string {
	return lo.Assign(billingSetting.BillingExpr)
}

func GetPricingSyncData(base map[string]any) map[string]any {
	extra := make(map[string]any, 2)
	if modes := GetBillingModeCopy(); len(modes) > 0 {
		extra[BillingModeField] = modes
	}
	if exprs := GetBillingExprCopy(); len(exprs) > 0 {
		extra[BillingExprField] = exprs
	}
	return lo.Assign(base, extra)
}

func SmokeTestBillingExpr(exprStr string) error {
	return smokeTestBillingExpr(exprStr)
}

func smokeTestBillingExpr(exprStr string) error {
	vectors := []billingexpr.TokenParams{
		{P: 0, C: 0, Len: 0},
		{P: 1000, C: 1000, Len: 1000},
		{P: 100000, C: 100000, Len: 100000},
		{P: 1000000, C: 1000000, Len: 1000000},
	}
	requests := []billingexpr.RequestInput{
		{},
		{
			Headers: map[string]string{
				"anthropic-beta": "fast-mode-2026-02-01",
			},
			Body: []byte(`{"service_tier":"fast","stream_options":{"include_usage":true},"messages":[1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17,18,19,20,21]}`),
		},
	}

	for _, v := range vectors {
		for _, request := range requests {
			result, _, err := billingexpr.RunExprWithRequest(exprStr, v, request)
			if err != nil {
				return fmt.Errorf("vector {p=%g, c=%g}: run failed: %w", v.P, v.C, err)
			}
			if result < 0 {
				return fmt.Errorf("vector {p=%g, c=%g}: result %f < 0", v.P, v.C, result)
			}
		}
	}
	return nil
}
