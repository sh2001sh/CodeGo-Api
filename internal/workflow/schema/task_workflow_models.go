package workflowschema

import (
	"encoding/json"
	"time"

	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
)

type WorkflowTaskWorkflow struct {
	WorkflowID         string          `json:"workflow_id" gorm:"column:workflow_id;primaryKey;size:64"`
	PublicTaskID       string          `json:"public_task_id" gorm:"column:public_task_id;size:191;uniqueIndex"`
	RequestID          string          `json:"request_id" gorm:"column:request_id;size:191"`
	AccountID          string          `json:"account_id" gorm:"column:account_id;size:64"`
	ProviderCode       string          `json:"provider_code" gorm:"column:provider_code;size:64;index"`
	ChannelID          int64           `json:"channel_id" gorm:"column:channel_id;index"`
	ReservationID      string          `json:"reservation_id" gorm:"column:reservation_id;size:64"`
	TaskKind           string          `json:"task_kind" gorm:"column:task_kind;size:64"`
	TemporalWorkflowID string          `json:"temporal_workflow_id" gorm:"column:temporal_workflow_id;size:255"`
	TemporalRunID      string          `json:"temporal_run_id" gorm:"column:temporal_run_id;size:255"`
	Status             string          `json:"status" gorm:"column:status;size:32;index"`
	TerminalState      string          `json:"terminal_state" gorm:"column:terminal_state;size:32"`
	TimeoutAt          *time.Time      `json:"timeout_at" gorm:"column:timeout_at"`
	ResultURL          string          `json:"result_url" gorm:"column:result_url;type:text"`
	ResultMeta         json.RawMessage `json:"result_meta" gorm:"column:result_meta;type:json"`
	CreatedAt          time.Time       `json:"created_at" gorm:"column:created_at;autoCreateTime"`
	UpdatedAt          time.Time       `json:"updated_at" gorm:"column:updated_at;autoUpdateTime"`
}

func (WorkflowTaskWorkflow) TableName() string {
	if platformdb.UsingPostgreSQL {
		return "workflow.task_workflows"
	}
	return "workflow_task_workflows"
}

type WorkflowTaskSnapshot struct {
	SnapshotID       string          `json:"snapshot_id" gorm:"column:snapshot_id;primaryKey;size:64"`
	WorkflowID       string          `json:"workflow_id" gorm:"column:workflow_id;size:64;index"`
	ProviderState    string          `json:"provider_state" gorm:"column:provider_state;size:64"`
	ProviderProgress int             `json:"provider_progress" gorm:"column:provider_progress"`
	RawPayload       json.RawMessage `json:"raw_payload" gorm:"column:raw_payload;type:json"`
	ResultURL        string          `json:"result_url" gorm:"column:result_url;type:text"`
	FailureReason    string          `json:"failure_reason" gorm:"column:failure_reason;type:text"`
	CreatedAt        time.Time       `json:"created_at" gorm:"column:created_at;autoCreateTime"`
}

func (WorkflowTaskSnapshot) TableName() string {
	if platformdb.UsingPostgreSQL {
		return "workflow.task_snapshots"
	}
	return "workflow_task_snapshots"
}

type WorkflowTaskTerminalResult struct {
	TerminalResultID string          `json:"terminal_result_id" gorm:"column:terminal_result_id;primaryKey;size:64"`
	WorkflowID       string          `json:"workflow_id" gorm:"column:workflow_id;size:64;uniqueIndex"`
	TerminalState    string          `json:"terminal_state" gorm:"column:terminal_state;size:32"`
	SettlementStatus string          `json:"settlement_status" gorm:"column:settlement_status;size:32"`
	ResultURL        string          `json:"result_url" gorm:"column:result_url;type:text"`
	ResultMeta       json.RawMessage `json:"result_meta" gorm:"column:result_meta;type:json"`
	FinalizedAt      time.Time       `json:"finalized_at" gorm:"column:finalized_at;autoCreateTime"`
}

func (WorkflowTaskTerminalResult) TableName() string {
	if platformdb.UsingPostgreSQL {
		return "workflow.task_terminal_results"
	}
	return "workflow_task_terminal_results"
}
