package bootstrap

import (
	"context"
	auditprojection "github.com/sh2001sh/CodeGo-Api/internal/audit/projection"
	billingapp "github.com/sh2001sh/CodeGo-Api/internal/billing/app"
	commerceapp "github.com/sh2001sh/CodeGo-Api/internal/commerce/app"
	identityapp "github.com/sh2001sh/CodeGo-Api/internal/identity/app"
	platformconfig "github.com/sh2001sh/CodeGo-Api/internal/platform/config"
	platformobservability "github.com/sh2001sh/CodeGo-Api/internal/platform/observability"
)

func RunLedgerWorker() {
	if err := prepareRuntime("ledger-worker"); err != nil {
		return
	}
	defer closeDatabase()

	if !platformconfig.IsMasterNode {
		platformobservability.FatalLog("ledger-worker requires master node role")
		return
	}

	startLedgerWorkerBackgroundTasks()
	startDiagnostics()

	platformobservability.SysLog("ledger worker maintenance loops started")
	select {}
}

func startLedgerWorkerBackgroundTasks() {
	startOptionSyncLoop()
	billingapp.StartLedgerWorker(context.Background())
	billingapp.StartOperationalSLOMonitor(context.Background())
	auditprojection.StartReadModelWorker(context.Background())
	commerceapp.StartSubscriptionMaintenanceTask()
	identityapp.StartImageWorkspaceCleanupTask()
}
