package schema

import (
	"strings"
	"time"

	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
	platformruntime "github.com/sh2001sh/CodeGo-Api/internal/platform/runtime"
	"gorm.io/gorm"
)

const (
	RequestExecutionStatusRecorded         = "recorded"
	RequestExecutionStatusProviderComplete = "provider_completed"
	RequestExecutionStatusSettled          = "settled"
)

type RequestExecution struct {
	ExecutionID     string    `gorm:"column:execution_id;primaryKey;size:64"`
	RequestID       string    `gorm:"column:request_id;size:64;uniqueIndex"`
	TraceID         string    `gorm:"column:trace_id;size:64;index"`
	UserID          int       `gorm:"column:user_id;index"`
	TokenID         int       `gorm:"column:token_id;index"`
	AccountID       string    `gorm:"column:account_id;size:64;index"`
	ReservationID   string    `gorm:"column:reservation_id;size:64;index"`
	SettlementID    string    `gorm:"column:settlement_id;size:64;index"`
	RoutePlanID     string    `gorm:"column:route_plan_id;size:64;index"`
	Status          string    `gorm:"column:status;size:32;index"`
	ActualAmount    int64     `gorm:"column:actual_amount"`
	UsageEvidenceID string    `gorm:"column:usage_evidence_id;size:64;index"`
	CreatedAt       time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt       time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (RequestExecution) TableName() string {
	if platformdb.UsingPostgreSQL {
		return "gateway.request_executions"
	}
	return "gateway_request_executions"
}

func (record *RequestExecution) BeforeCreate(_ *gorm.DB) error {
	if strings.TrimSpace(record.ExecutionID) == "" {
		record.ExecutionID = platformruntime.GetUUID()
	}
	if strings.TrimSpace(record.Status) == "" {
		record.Status = RequestExecutionStatusRecorded
	}
	return nil
}

type GatewayRoutePlan struct {
	RoutePlanID string    `gorm:"column:route_plan_id;primaryKey;size:64"`
	RequestID   string    `gorm:"column:request_id;size:64;uniqueIndex"`
	TraceID     string    `gorm:"column:trace_id;size:64;index"`
	Status      string    `gorm:"column:status;size:32"`
	CreatedAt   time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt   time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (GatewayRoutePlan) TableName() string {
	if platformdb.UsingPostgreSQL {
		return "gateway.route_plans"
	}
	return "gateway_route_plans"
}

type ExecutionAttempt struct {
	AttemptID   string    `gorm:"column:attempt_id;primaryKey;size:64"`
	ExecutionID string    `gorm:"column:execution_id;size:64;index"`
	TraceID     string    `gorm:"column:trace_id;size:64;index"`
	AttemptNo   int       `gorm:"column:attempt_no"`
	Status      string    `gorm:"column:status;size:32"`
	CreatedAt   time.Time `gorm:"column:created_at;autoCreateTime"`
}

func (ExecutionAttempt) TableName() string {
	if platformdb.UsingPostgreSQL {
		return "gateway.execution_attempts"
	}
	return "gateway_execution_attempts"
}

func (attempt *ExecutionAttempt) BeforeCreate(_ *gorm.DB) error {
	if strings.TrimSpace(attempt.AttemptID) == "" {
		attempt.AttemptID = platformruntime.GetUUID()
	}
	return nil
}

type UsageEvidence struct {
	UsageEvidenceID string    `gorm:"column:usage_evidence_id;primaryKey;size:64"`
	ExecutionID     string    `gorm:"column:execution_id;size:64;index"`
	RequestID       string    `gorm:"column:request_id;size:64;uniqueIndex"`
	TraceID         string    `gorm:"column:trace_id;size:64;index"`
	ActualAmount    int64     `gorm:"column:actual_amount"`
	CreatedAt       time.Time `gorm:"column:created_at;autoCreateTime"`
}

func (UsageEvidence) TableName() string {
	if platformdb.UsingPostgreSQL {
		return "gateway.usage_evidence"
	}
	return "gateway_usage_evidence"
}

func (evidence *UsageEvidence) BeforeCreate(_ *gorm.DB) error {
	if strings.TrimSpace(evidence.UsageEvidenceID) == "" {
		evidence.UsageEvidenceID = platformruntime.GetUUID()
	}
	return nil
}
