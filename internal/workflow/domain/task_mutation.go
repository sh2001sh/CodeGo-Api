package domain

import (
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
	workflowschema "github.com/sh2001sh/CodeGo-Api/internal/workflow/schema"
)

// InsertTask persists a newly created async task record.
func InsertTask(task *workflowschema.Task) error {
	if task == nil {
		return nil
	}
	return platformdb.DB.Create(task).Error
}

// SaveTask persists all mutable task fields without a status CAS guard.
func SaveTask(task *workflowschema.Task) error {
	if task == nil {
		return nil
	}
	return platformdb.DB.Save(task).Error
}

// UpdateTaskQuota writes back the settled quota for an async task.
func UpdateTaskQuota(task *workflowschema.Task) error {
	if task == nil {
		return nil
	}
	return platformdb.DB.Model(task).Update("quota", task.Quota).Error
}
