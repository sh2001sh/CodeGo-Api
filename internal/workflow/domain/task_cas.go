package domain

import (
	"bytes"
	"encoding/json"

	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
	workflowschema "github.com/sh2001sh/CodeGo-Api/internal/workflow/schema"
)

// TaskSnapshot captures the fields that drive task terminal-state transitions.
type TaskSnapshot struct {
	Status     TaskStatus
	Progress   string
	StartTime  int64
	FinishTime int64
	FailReason string
	ResultURL  string
	Data       json.RawMessage
}

// Equal reports whether two task snapshots describe the same externally relevant state.
func (s TaskSnapshot) Equal(other TaskSnapshot) bool {
	return s.Status == other.Status &&
		s.Progress == other.Progress &&
		s.StartTime == other.StartTime &&
		s.FinishTime == other.FinishTime &&
		s.FailReason == other.FailReason &&
		s.ResultURL == other.ResultURL &&
		bytes.Equal(s.Data, other.Data)
}

// TakeTaskSnapshot returns the CAS snapshot used by workflow terminal-state transitions.
func TakeTaskSnapshot(task *workflowschema.Task) TaskSnapshot {
	if task == nil {
		return TaskSnapshot{}
	}
	return TaskSnapshot{
		Status:     task.Status,
		Progress:   task.Progress,
		StartTime:  task.StartTime,
		FinishTime: task.FinishTime,
		FailReason: task.FailReason,
		ResultURL:  task.PrivateData.ResultURL,
		Data:       task.Data,
	}
}

// UpdateTaskWithStatus performs a conditional UPDATE guarded by fromStatus (CAS).
// Returns true only when this caller wins the task transition.
func UpdateTaskWithStatus(task *workflowschema.Task, fromStatus TaskStatus) (bool, error) {
	if task == nil {
		return false, nil
	}
	result := platformdb.DB.Model(task).Where("status = ?", fromStatus).Select("*").Updates(task)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}
