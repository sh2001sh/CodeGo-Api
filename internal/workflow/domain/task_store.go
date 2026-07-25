package domain

import (
	"errors"

	"github.com/sh2001sh/CodeGo-Api/constant"
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
	workflowschema "github.com/sh2001sh/CodeGo-Api/internal/workflow/schema"
	"gorm.io/gorm"
)

// TaskListQuery defines workflow task list filters shared by workflow readers.
type TaskListQuery struct {
	Platform       constant.TaskPlatform
	ChannelID      string
	TaskID         string
	UserID         string
	Action         string
	Status         string
	StartTimestamp int64
	EndTimestamp   int64
	UserIDs        []int
}

// ListTasks returns admin-visible tasks matching the query.
func ListTasks(startIdx int, pageSize int, query TaskListQuery) []*workflowschema.Task {
	var tasks []*workflowschema.Task
	dbQuery := buildTaskQuery(platformdb.DB, query)
	if err := dbQuery.Order("id desc").Limit(pageSize).Offset(startIdx).Find(&tasks).Error; err != nil {
		return nil
	}
	return tasks
}

// CountTasks returns the total number of admin-visible tasks matching the query.
func CountTasks(query TaskListQuery) int64 {
	var total int64
	dbQuery := buildTaskQuery(platformdb.DB.Model(&workflowschema.Task{}), query)
	_ = dbQuery.Count(&total).Error
	return total
}

// ListUserTasks returns user-visible tasks matching the query.
func ListUserTasks(userID int, startIdx int, pageSize int, query TaskListQuery) []*workflowschema.Task {
	var tasks []*workflowschema.Task
	dbQuery := buildUserTaskQuery(userID, query)
	if err := dbQuery.Omit("channel_id").Order("id desc").Limit(pageSize).Offset(startIdx).Find(&tasks).Error; err != nil {
		return nil
	}
	return tasks
}

// CountUserTasks returns the total number of tasks matching the query for one user.
func CountUserTasks(userID int, query TaskListQuery) int64 {
	var total int64
	dbQuery := buildUserTaskQuery(userID, query).Model(&workflowschema.Task{})
	_ = dbQuery.Count(&total).Error
	return total
}

// GetTimedOutUnfinishedTasks returns unfinished tasks older than the cutoff.
func GetTimedOutUnfinishedTasks(cutoffUnix int64, limit int) []*workflowschema.Task {
	var tasks []*workflowschema.Task
	err := platformdb.DB.Where("progress != ?", "100%").
		Where("status NOT IN ?", []string{string(TaskStatusFailure), string(TaskStatusSuccess)}).
		Where("submit_time < ?", cutoffUnix).
		Order("submit_time").
		Limit(limit).
		Find(&tasks).Error
	if err != nil {
		return nil
	}
	return tasks
}

// GetAllUnfinishedSyncTasks returns all unfinished async tasks up to the limit.
func GetAllUnfinishedSyncTasks(limit int) []*workflowschema.Task {
	var tasks []*workflowschema.Task
	err := platformdb.DB.Where("progress != ?", "100%").
		Where("status != ?", TaskStatusFailure).
		Where("status != ?", TaskStatusSuccess).
		Limit(limit).
		Order("id").
		Find(&tasks).Error
	if err != nil {
		return nil
	}
	return tasks
}

// GetTaskByID returns one task owned by the user, looked up by public task ID.
func GetTaskByID(userID int, taskID string) (*workflowschema.Task, bool, error) {
	if taskID == "" {
		return nil, false, nil
	}
	var task *workflowschema.Task
	err := platformdb.DB.Where("user_id = ? and task_id = ?", userID, taskID).First(&task).Error
	exist, err := recordExists(err)
	if err != nil {
		return nil, false, err
	}
	return task, exist, err
}

// GetTasksByIDs returns all tasks owned by the user for the given public task IDs.
func GetTasksByIDs(userID int, taskIDs []any) ([]*workflowschema.Task, error) {
	if len(taskIDs) == 0 {
		return nil, nil
	}
	var tasks []*workflowschema.Task
	err := platformdb.DB.Where("user_id = ? and task_id in (?)", userID, taskIDs).Find(&tasks).Error
	if err != nil {
		return nil, err
	}
	return tasks, nil
}

// BulkUpdateTasksByID updates tasks by primary key without a CAS guard.
func BulkUpdateTasksByID(ids []int64, params map[string]any) error {
	if len(ids) == 0 {
		return nil
	}
	return platformdb.DB.Model(&workflowschema.Task{}).
		Where("id in (?)", ids).
		Updates(params).Error
}

func buildTaskQuery(db *gorm.DB, query TaskListQuery) *gorm.DB {
	if query.ChannelID != "" {
		db = db.Where("channel_id = ?", query.ChannelID)
	}
	if query.Platform != "" {
		db = db.Where("platform = ?", query.Platform)
	}
	if query.UserID != "" {
		db = db.Where("user_id = ?", query.UserID)
	}
	if len(query.UserIDs) != 0 {
		db = db.Where("user_id in (?)", query.UserIDs)
	}
	if query.TaskID != "" {
		db = db.Where("task_id = ?", query.TaskID)
	}
	if query.Action != "" {
		db = db.Where("action = ?", query.Action)
	}
	if query.Status != "" {
		db = db.Where("status = ?", query.Status)
	}
	if query.StartTimestamp != 0 {
		db = db.Where("submit_time >= ?", query.StartTimestamp)
	}
	if query.EndTimestamp != 0 {
		db = db.Where("submit_time <= ?", query.EndTimestamp)
	}
	return db
}

func buildUserTaskQuery(userID int, query TaskListQuery) *gorm.DB {
	db := platformdb.DB.Where("user_id = ?", userID)
	if query.TaskID != "" {
		db = db.Where("task_id = ?", query.TaskID)
	}
	if query.Action != "" {
		db = db.Where("action = ?", query.Action)
	}
	if query.Status != "" {
		db = db.Where("status = ?", query.Status)
	}
	if query.Platform != "" {
		db = db.Where("platform = ?", query.Platform)
	}
	if query.StartTimestamp != 0 {
		db = db.Where("submit_time >= ?", query.StartTimestamp)
	}
	if query.EndTimestamp != 0 {
		db = db.Where("submit_time <= ?", query.EndTimestamp)
	}
	return db
}

func recordExists(err error) (bool, error) {
	if err == nil {
		return true, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	return false, err
}
