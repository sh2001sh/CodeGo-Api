package domain

import (
	"encoding/json"
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
	platformruntime "github.com/sh2001sh/CodeGo-Api/internal/platform/runtime"
	workflowschema "github.com/sh2001sh/CodeGo-Api/internal/workflow/schema"
	"gorm.io/gorm/clause"
	"time"
)

func GetTaskByPublicTaskID(taskID string) (*workflowschema.Task, error) {
	if taskID == "" {
		return nil, nil
	}
	task := &workflowschema.Task{}
	if err := platformdb.DB.Where("task_id = ?", taskID).First(task).Error; err != nil {
		return nil, err
	}
	return task, nil
}

func GetTaskWorkflowByPublicTaskID(publicTaskID string) (*workflowschema.WorkflowTaskWorkflow, error) {
	if publicTaskID == "" {
		return nil, nil
	}
	record := &workflowschema.WorkflowTaskWorkflow{}
	if err := platformdb.DB.Where("public_task_id = ?", publicTaskID).First(record).Error; err != nil {
		return nil, err
	}
	return record, nil
}

func UpsertTaskWorkflow(record *workflowschema.WorkflowTaskWorkflow) error {
	if record == nil || record.PublicTaskID == "" {
		return nil
	}
	if record.WorkflowID == "" {
		record.WorkflowID = platformruntime.GetUUID()
	}
	if len(record.ResultMeta) == 0 {
		record.ResultMeta = json.RawMessage(`{}`)
	}
	now := time.Now()
	return platformdb.DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "public_task_id"}},
		DoUpdates: clause.Assignments(map[string]any{
			"request_id":           record.RequestID,
			"account_id":           record.AccountID,
			"provider_code":        record.ProviderCode,
			"channel_id":           record.ChannelID,
			"reservation_id":       record.ReservationID,
			"task_kind":            record.TaskKind,
			"temporal_workflow_id": record.TemporalWorkflowID,
			"temporal_run_id":      record.TemporalRunID,
			"status":               record.Status,
			"terminal_state":       record.TerminalState,
			"timeout_at":           record.TimeoutAt,
			"result_url":           record.ResultURL,
			"result_meta":          record.ResultMeta,
			"updated_at":           now,
		}),
	}).Create(record).Error
}

func UpdateTaskWorkflowFields(publicTaskID string, fields map[string]any) error {
	if publicTaskID == "" || len(fields) == 0 {
		return nil
	}
	fields["updated_at"] = time.Now()
	return platformdb.DB.Model(&workflowschema.WorkflowTaskWorkflow{}).
		Where("public_task_id = ?", publicTaskID).
		Updates(fields).Error
}

func InsertTaskWorkflowSnapshot(snapshot *workflowschema.WorkflowTaskSnapshot) error {
	if snapshot == nil || snapshot.WorkflowID == "" {
		return nil
	}
	if snapshot.SnapshotID == "" {
		snapshot.SnapshotID = platformruntime.GetUUID()
	}
	if len(snapshot.RawPayload) == 0 {
		snapshot.RawPayload = json.RawMessage(`{}`)
	}
	return platformdb.DB.Create(snapshot).Error
}

func UpsertTaskTerminalResult(result *workflowschema.WorkflowTaskTerminalResult) error {
	if result == nil || result.WorkflowID == "" {
		return nil
	}
	if result.TerminalResultID == "" {
		result.TerminalResultID = platformruntime.GetUUID()
	}
	if len(result.ResultMeta) == 0 {
		result.ResultMeta = json.RawMessage(`{}`)
	}
	return platformdb.DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "workflow_id"}},
		DoUpdates: clause.Assignments(map[string]any{
			"terminal_state":    result.TerminalState,
			"settlement_status": result.SettlementStatus,
			"result_url":        result.ResultURL,
			"result_meta":       result.ResultMeta,
			"finalized_at":      time.Now(),
		}),
	}).Create(result).Error
}
