package temporal

import (
	"fmt"
	platformobservability "github.com/sh2001sh/CodeGo-Api/internal/platform/observability"

	"github.com/sh2001sh/CodeGo-Api/internal/workflow/temporal/contracts"
	"github.com/sh2001sh/CodeGo-Api/internal/workflow/temporal/workflows"
	temporalactivity "go.temporal.io/sdk/activity"
	temporalclient "go.temporal.io/sdk/client"
	temporalworker "go.temporal.io/sdk/worker"
	temporalworkflow "go.temporal.io/sdk/workflow"
)

type workerRegistrar interface {
	RegisterWorkflowWithOptions(w interface{}, options temporalworkflow.RegisterOptions)
	RegisterActivityWithOptions(a interface{}, options temporalactivity.RegisterOptions)
}

type WorkerBootstrap struct {
	Client temporalclient.Client
	Deps   WorkerDependencies
}

type workerSpec struct {
	name      string
	taskQueue string
	register  func(*WorkerBootstrap, workerRegistrar)
}

type WorkerRegistry struct {
	client temporalclient.Client
	cfg    Config
}

// NewWorkerRegistry builds the queue registry used by workflow-worker.
func NewWorkerRegistry(client temporalclient.Client, cfg Config) *WorkerRegistry {
	return &WorkerRegistry{client: client, cfg: cfg}
}

// Run starts all registered Temporal workers and blocks until a process interrupt is received.
func (r *WorkerRegistry) Run(bootstrap *WorkerBootstrap) error {
	specs := r.specs()
	started := make([]temporalworker.Worker, 0, len(specs))
	for _, spec := range specs {
		w := temporalworker.New(r.client, spec.taskQueue, temporalworker.Options{})
		spec.register(bootstrap, w)
		if err := w.Start(); err != nil {
			for i := len(started) - 1; i >= 0; i-- {
				started[i].Stop()
			}
			return fmt.Errorf("start temporal worker %s: %w", spec.name, err)
		}
		started = append(started, w)
		platformobservability.SysLog(fmt.Sprintf("temporal worker started name=%s task_queue=%s", spec.name, spec.taskQueue))
	}

	<-temporalworker.InterruptCh()
	for i := len(started) - 1; i >= 0; i-- {
		started[i].Stop()
	}
	return nil
}

func (r *WorkerRegistry) specs() []workerSpec {
	return []workerSpec{
		{name: "async-tasks", taskQueue: r.cfg.TaskQueueTasks, register: registerTaskWorker},
		{name: "billing", taskQueue: r.cfg.TaskQueueBilling, register: registerBillingWorker},
		{name: "orders", taskQueue: r.cfg.TaskQueueOrders, register: registerOrderWorker},
		{name: "subscriptions", taskQueue: r.cfg.TaskQueueSubscriptions, register: registerSubscriptionWorker},
	}
}

func registerTaskWorker(bootstrap *WorkerBootstrap, registrar workerRegistrar) {
	registrar.RegisterWorkflowWithOptions(workflows.AsyncTaskWorkflow, temporalworkflow.RegisterOptions{Name: contracts.WorkflowAsyncTask})
	registrar.RegisterActivityWithOptions(bootstrap.Deps.TaskActivities.SubmitAsyncTask, temporalactivity.RegisterOptions{Name: contracts.ActivitySubmitAsyncTask})
	registrar.RegisterActivityWithOptions(bootstrap.Deps.TaskActivities.PollAsyncTaskStatus, temporalactivity.RegisterOptions{Name: contracts.ActivityPollAsyncTaskStatus})
	registrar.RegisterActivityWithOptions(bootstrap.Deps.TaskActivities.RecordTaskWorkflow, temporalactivity.RegisterOptions{Name: contracts.ActivityRecordTaskWorkflow})
	registrar.RegisterActivityWithOptions(bootstrap.Deps.TaskActivities.RecordTaskSnapshot, temporalactivity.RegisterOptions{Name: contracts.ActivityRecordTaskSnapshot})
	registrar.RegisterActivityWithOptions(bootstrap.Deps.TaskActivities.FinalizeTaskTerminalState, temporalactivity.RegisterOptions{Name: contracts.ActivityFinalizeTaskTerminalState})
	registrar.RegisterActivityWithOptions(bootstrap.Deps.TaskActivities.ProjectTaskResult, temporalactivity.RegisterOptions{Name: contracts.ActivityProjectTaskResult})
	registrar.RegisterActivityWithOptions(bootstrap.Deps.BillingActivities.RefundReference, temporalactivity.RegisterOptions{Name: contracts.ActivityRefundReference})
}

func registerBillingWorker(bootstrap *WorkerBootstrap, registrar workerRegistrar) {
	registrar.RegisterWorkflowWithOptions(workflows.RequestSettlementWorkflow, temporalworkflow.RegisterOptions{Name: contracts.WorkflowRequestSettlement})
	registrar.RegisterActivityWithOptions(bootstrap.Deps.GatewayActivities.CreateRequestExecution, temporalactivity.RegisterOptions{Name: contracts.ActivityCreateRequestExecution})
	registrar.RegisterActivityWithOptions(bootstrap.Deps.GatewayActivities.ExecuteProviderRequest, temporalactivity.RegisterOptions{Name: contracts.ActivityExecuteProviderRequest})
	registrar.RegisterActivityWithOptions(bootstrap.Deps.GatewayActivities.CollectUsageEvidence, temporalactivity.RegisterOptions{Name: contracts.ActivityCollectUsageEvidence})
	registrar.RegisterActivityWithOptions(bootstrap.Deps.GatewayActivities.PublishRequestSettledEvent, temporalactivity.RegisterOptions{Name: contracts.ActivityPublishRequestSettled})
	registrar.RegisterActivityWithOptions(bootstrap.Deps.BillingActivities.CreateReservation, temporalactivity.RegisterOptions{Name: contracts.ActivityCreateReservation})
	registrar.RegisterActivityWithOptions(bootstrap.Deps.BillingActivities.CreateSettlement, temporalactivity.RegisterOptions{Name: contracts.ActivityCreateSettlement})
	registrar.RegisterActivityWithOptions(bootstrap.Deps.BillingActivities.RefreshAccountSnapshot, temporalactivity.RegisterOptions{Name: contracts.ActivityRefreshAccountSnapshot})
}

func registerOrderWorker(bootstrap *WorkerBootstrap, registrar workerRegistrar) {
	registrar.RegisterWorkflowWithOptions(workflows.OrderFulfillmentWorkflow, temporalworkflow.RegisterOptions{Name: contracts.WorkflowOrderFulfillment})
	registrar.RegisterActivityWithOptions(bootstrap.Deps.OrderActivities.CreateOrderRecord, temporalactivity.RegisterOptions{Name: contracts.ActivityCreateOrderRecord})
	registrar.RegisterActivityWithOptions(bootstrap.Deps.OrderActivities.ValidatePaymentCallback, temporalactivity.RegisterOptions{Name: contracts.ActivityValidatePaymentCallback})
	registrar.RegisterActivityWithOptions(bootstrap.Deps.OrderActivities.MarkOrderPaid, temporalactivity.RegisterOptions{Name: contracts.ActivityMarkOrderPaid})
	registrar.RegisterActivityWithOptions(bootstrap.Deps.OrderActivities.GrantOrderBenefits, temporalactivity.RegisterOptions{Name: contracts.ActivityGrantOrderBenefits})
	registrar.RegisterActivityWithOptions(bootstrap.Deps.OrderActivities.PublishOrderPaidEvent, temporalactivity.RegisterOptions{Name: contracts.ActivityPublishOrderPaidEvent})
}

func registerSubscriptionWorker(bootstrap *WorkerBootstrap, registrar workerRegistrar) {
	registrar.RegisterWorkflowWithOptions(workflows.SubscriptionResetWorkflow, temporalworkflow.RegisterOptions{Name: contracts.WorkflowSubscriptionReset})
	registrar.RegisterActivityWithOptions(bootstrap.Deps.OrderActivities.FindResettableSubscriptions, temporalactivity.RegisterOptions{Name: contracts.ActivityFindResettableSubs})
	registrar.RegisterActivityWithOptions(bootstrap.Deps.BillingActivities.CreateResetLedgerEntries, temporalactivity.RegisterOptions{Name: contracts.ActivityCreateResetLedgerEntries})
	registrar.RegisterActivityWithOptions(bootstrap.Deps.OrderActivities.ResetUsageProjection, temporalactivity.RegisterOptions{Name: contracts.ActivityResetUsageProjection})
	registrar.RegisterActivityWithOptions(bootstrap.Deps.OrderActivities.PublishResetAuditEvents, temporalactivity.RegisterOptions{Name: contracts.ActivityPublishResetAuditEvents})
}
