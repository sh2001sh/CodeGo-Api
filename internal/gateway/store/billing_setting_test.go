package store

import (
	"testing"

	"github.com/sh2001sh/CodeGo-Api/internal/billing/domain/billingexpr"
)

func TestDefaultGPTPricingExpressions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		model      string
		shortCost  float64
		longCost   float64
		longPriced bool
	}{
		{model: "gpt-5.6-sol", shortCost: 41.75, longCost: 68.5, longPriced: true},
		{model: "gpt-5.6-terra", shortCost: 20.875, longCost: 34.25, longPriced: true},
		{model: "gpt-5.6-luna", shortCost: 8.35, longCost: 13.7, longPriced: true},
		{model: "gpt-5.5", shortCost: 35.5, longCost: 56, longPriced: true},
		{model: "gpt-5.5-pro", shortCost: 210, longCost: 330, longPriced: true},
		{model: "gpt-5.4", shortCost: 17.75, longCost: 28, longPriced: true},
		{model: "gpt-5.4-mini", shortCost: 5.325},
		{model: "gpt-5.4-nano", shortCost: 1.47},
		{model: "gpt-5.4-pro", shortCost: 210, longCost: 330, longPriced: true},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			expr, ok := GetBillingExpr(tt.model)
			if !ok {
				t.Fatal("missing default billing expression")
			}
			if err := SmokeTestBillingExpr(expr); err != nil {
				t.Fatalf("invalid billing expression: %v", err)
			}

			shortCost, shortTrace, err := billingexpr.RunExpr(expr, billingexpr.TokenParams{P: 1, C: 1, CR: 1, CC: 1, Len: 272000})
			if err != nil {
				t.Fatalf("short context expression failed: %v", err)
			}
			if shortCost != tt.shortCost || shortTrace.MatchedTier != "short_context" {
				t.Fatalf("short context = %g (%s), want %g (short_context)", shortCost, shortTrace.MatchedTier, tt.shortCost)
			}

			longCost, longTrace, err := billingexpr.RunExpr(expr, billingexpr.TokenParams{P: 1, C: 1, CR: 1, CC: 1, Len: 272001})
			if err != nil {
				t.Fatalf("long context expression failed: %v", err)
			}
			if tt.longPriced {
				if longCost != tt.longCost || longTrace.MatchedTier != "long_context" {
					t.Fatalf("long context = %g (%s), want %g (long_context)", longCost, longTrace.MatchedTier, tt.longCost)
				}
				return
			}
			if longCost != tt.shortCost || longTrace.MatchedTier != "short_context" {
				t.Fatalf("flat price = %g (%s), want %g (short_context)", longCost, longTrace.MatchedTier, tt.shortCost)
			}
		})
	}
}

func TestGPT54SnapshotUsesCurrentPricing(t *testing.T) {
	t.Parallel()

	for _, model := range []string{"gpt-5.4-2026-03-05", "gpt-5.4-pro-2026-03-05"} {
		if mode := GetBillingMode(model); mode != BillingModeTieredExpr {
			t.Fatalf("%s billing mode = %q, want %q", model, mode, BillingModeTieredExpr)
		}
		if _, ok := GetBillingExpr(model); !ok {
			t.Fatalf("%s is missing its billing expression", model)
		}
	}
}

func TestMergeMissingDefaultBillingRulesPreservesOverrides(t *testing.T) {
	setting := BillingSetting{
		BillingMode: map[string]string{
			"gpt-5.6-sol": "ratio",
		},
		BillingExpr: map[string]string{
			"gpt-5.6-sol": `tier("custom", p * 1)`,
		},
	}

	mergeMissingDefaultBillingRules(&setting)

	if setting.BillingMode["gpt-5.6-sol"] != "ratio" {
		t.Fatalf("explicit billing mode was overwritten: %q", setting.BillingMode["gpt-5.6-sol"])
	}
	if setting.BillingExpr["gpt-5.6-sol"] != `tier("custom", p * 1)` {
		t.Fatalf("explicit billing expression was overwritten: %q", setting.BillingExpr["gpt-5.6-sol"])
	}
	if setting.BillingMode["gpt-5.6-terra"] != BillingModeTieredExpr {
		t.Fatalf("missing default billing mode was not restored: %q", setting.BillingMode["gpt-5.6-terra"])
	}
	if setting.BillingExpr["gpt-5.6-terra"] == "" {
		t.Fatal("missing default billing expression was not restored")
	}
}
