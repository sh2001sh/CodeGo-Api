package contracts

const (
	WorkflowAsyncTask         = "workflow.async_task"
	WorkflowRequestSettlement = "workflow.request_settlement"
	WorkflowOrderFulfillment  = "workflow.order_fulfillment"
	WorkflowSubscriptionReset = "workflow.subscription_reset"
)

const (
	ActivityCreateReservation         = "billing.create_reservation"
	ActivityCreateSettlement          = "billing.create_settlement"
	ActivityRefundReference           = "billing.refund_reference"
	ActivityCreateResetLedgerEntries  = "billing.create_reset_ledger_entries"
	ActivityRefreshAccountSnapshot    = "billing.refresh_account_snapshot"
	ActivitySubmitAsyncTask           = "task.submit_async_task"
	ActivityPollAsyncTaskStatus       = "task.poll_async_task_status"
	ActivityRecordTaskWorkflow        = "task.record_task_workflow"
	ActivityRecordTaskSnapshot        = "task.record_task_snapshot"
	ActivityFinalizeTaskTerminalState = "task.finalize_task_terminal_state"
	ActivityProjectTaskResult         = "task.project_task_result"
	ActivityCreateRequestExecution    = "gateway.create_request_execution"
	ActivityExecuteProviderRequest    = "gateway.execute_provider_request"
	ActivityCollectUsageEvidence      = "gateway.collect_usage_evidence"
	ActivityPublishRequestSettled     = "gateway.publish_request_settled_event"
	ActivityCreateOrderRecord         = "order.create_order_record"
	ActivityValidatePaymentCallback   = "order.validate_payment_callback"
	ActivityMarkOrderPaid             = "order.mark_order_paid"
	ActivityGrantOrderBenefits        = "order.grant_order_benefits"
	ActivityPublishOrderPaidEvent     = "order.publish_order_paid_event"
	ActivityFindResettableSubs        = "subscription.find_resettable_subscriptions"
	ActivityResetUsageProjection      = "subscription.reset_usage_projection"
	ActivityPublishResetAuditEvents   = "subscription.publish_reset_audit_events"
)
