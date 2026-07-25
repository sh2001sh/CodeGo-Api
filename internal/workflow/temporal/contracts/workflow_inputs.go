package contracts

import (
	"encoding/json"
	"time"
)

type AsyncTaskWorkflowInput struct {
	WorkflowVersion string          `json:"workflow_version"`
	PublicTaskID    string          `json:"public_task_id"`
	RequestID       string          `json:"request_id"`
	AccountID       string          `json:"account_id"`
	ProviderCode    string          `json:"provider_code"`
	ChannelID       int64           `json:"channel_id"`
	ReservationID   string          `json:"reservation_id"`
	TaskKind        string          `json:"task_kind"`
	SubmitPayload   json.RawMessage `json:"submit_payload"`
	TimeoutAt       time.Time       `json:"timeout_at"`
}

type RequestSettlementWorkflowInput struct {
	WorkflowVersion string          `json:"workflow_version"`
	RequestID       string          `json:"request_id"`
	TraceID         string          `json:"trace_id"`
	UserID          int             `json:"user_id"`
	TokenID         int             `json:"token_id"`
	AccountID       string          `json:"account_id"`
	ReservationID   string          `json:"reservation_id"`
	SettlementID    string          `json:"settlement_id"`
	UsageEvidenceID string          `json:"usage_evidence_id"`
	ReservedAmount  int64           `json:"reserved_amount"`
	ActualAmount    int64           `json:"actual_amount"`
	RoutePlanID     string          `json:"route_plan_id"`
	RequestPayload  json.RawMessage `json:"request_payload"`
}

type OrderFulfillmentWorkflowInput struct {
	WorkflowVersion string  `json:"workflow_version"`
	OrderID         string  `json:"order_id"`
	ProductID       string  `json:"product_id"`
	PaymentProvider string  `json:"payment_provider"`
	Amount          float64 `json:"amount"`
}

type SubscriptionResetWorkflowInput struct {
	WorkflowVersion string `json:"workflow_version"`
	ResetReason     string `json:"reset_reason"`
	SubscriptionID  string `json:"subscription_id"`
	RequestedBy     string `json:"requested_by"`
}
