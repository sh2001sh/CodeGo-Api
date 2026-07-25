package app

import (
	"context"
	"time"

	auditapp "github.com/sh2001sh/CodeGo-Api/internal/audit/app"
	auditprojection "github.com/sh2001sh/CodeGo-Api/internal/audit/projection"
	platformobservability "github.com/sh2001sh/CodeGo-Api/internal/platform/observability"
)

const (
	operationalSLOSuccessRate = 95.0
	operationalSLOMinSamples  = 10
)

type OperationalSLO struct {
	Hours                 int      `json:"hours"`
	ModelsObserved        int      `json:"models_observed"`
	ModelsBelowSuccessSLO []string `json:"models_below_success_slo"`
	InconsistentAccounts  int      `json:"inconsistent_accounts"`
}

func BuildOperationalSLO(ctx context.Context, hours int) (OperationalSLO, error) {
	if hours <= 0 {
		hours = 24
	}
	summary, err := auditapp.BuildPerfMetricsSummary(hours)
	if err != nil {
		return OperationalSLO{}, err
	}
	result := OperationalSLO{Hours: hours}
	perf, ok := summary.(auditprojection.SummaryAllResult)
	if ok {
		result.ModelsObserved = len(perf.Models)
		for _, model := range perf.Models {
			if model.RequestCount >= operationalSLOMinSamples && model.SuccessRate < operationalSLOSuccessRate {
				result.ModelsBelowSuccessSLO = append(result.ModelsBelowSuccessSLO, model.ModelName)
			}
		}
	}
	reconciliations, err := ListLedgerReconciliations(ctx, 500)
	if err != nil {
		return result, err
	}
	for _, item := range reconciliations {
		if !item.Consistent {
			result.InconsistentAccounts++
		}
	}
	return result, nil
}

func StartOperationalSLOMonitor(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			slo, err := BuildOperationalSLO(ctx, 24)
			if err != nil {
				platformobservability.SysError("operational slo evaluation failed: " + err.Error())
			} else if len(slo.ModelsBelowSuccessSLO) > 0 || slo.InconsistentAccounts > 0 {
				platformobservability.SysError("operational slo breach detected")
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}
