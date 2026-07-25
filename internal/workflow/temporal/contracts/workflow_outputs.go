package contracts

type AsyncTaskWorkflowOutput struct {
	TerminalState    string `json:"terminal_state"`
	SettlementStatus string `json:"settlement_status"`
	ResultURL        string `json:"result_url"`
	FailureReason    string `json:"failure_reason"`
}

type AsyncTaskFinalizeResult struct {
	TerminalState    string `json:"terminal_state"`
	SettlementStatus string `json:"settlement_status"`
	ResultURL        string `json:"result_url"`
	FailureReason    string `json:"failure_reason"`
}

type RequestSettlementWorkflowOutput struct {
	ExecutionStatus string `json:"execution_status"`
	ReservationID   string `json:"reservation_id"`
	SettlementID    string `json:"settlement_id"`
	UsageEvidenceID string `json:"usage_evidence_id"`
}

type OrderFulfillmentWorkflowOutput struct {
	OrderStatus   string `json:"order_status"`
	BenefitStatus string `json:"benefit_status"`
}

type SubscriptionResetWorkflowOutput struct {
	ResetStatus string `json:"reset_status"`
}

type AsyncTaskSubmitResult struct {
	ExternalTaskID string `json:"external_task_id"`
}

type AsyncTaskPollResult struct {
	TerminalState string `json:"terminal_state"`
	ResultURL     string `json:"result_url"`
	FailureReason string `json:"failure_reason"`
	Done          bool   `json:"done"`
}

type RequestExecutionResult struct {
	ExecutionID string `json:"execution_id"`
}

type ReservationResult struct {
	ReservationID string `json:"reservation_id"`
}

type SettlementResult struct {
	SettlementID string `json:"settlement_id"`
}

type UsageEvidenceResult struct {
	UsageEvidenceID string `json:"usage_evidence_id"`
}

type OrderCallbackValidationResult struct {
	Valid bool `json:"valid"`
}
