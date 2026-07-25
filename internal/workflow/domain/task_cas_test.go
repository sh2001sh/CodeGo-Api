package domain

import (
	"encoding/json"
	"github.com/glebarez/sqlite"
	platformcache "github.com/sh2001sh/CodeGo-Api/internal/platform/cache"
	platformconfig "github.com/sh2001sh/CodeGo-Api/internal/platform/config"
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
	workflowschema "github.com/sh2001sh/CodeGo-Api/internal/workflow/schema"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"os"
	"sync"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		panic("failed to open test db: " + err.Error())
	}
	platformdb.DB = db
	platformdb.LogDB = db
	platformdb.UsingSQLite = true
	platformcache.RedisEnabled = false
	platformconfig.BatchUpdateEnabled = false
	platformconfig.LogConsumeEnabled = true

	sqlDB, err := db.DB()
	if err != nil {
		panic("failed to get sql.DB: " + err.Error())
	}
	sqlDB.SetMaxOpenConns(1)

	if err := db.AutoMigrate(&workflowschema.Task{}); err != nil {
		panic("failed to migrate: " + err.Error())
	}

	os.Exit(m.Run())
}

func truncateTasks(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		platformdb.DB.Exec("DELETE FROM tasks")
	})
}

func insertTask(t *testing.T, task *workflowschema.Task) {
	t.Helper()
	task.CreatedAt = time.Now().Unix()
	task.UpdatedAt = time.Now().Unix()
	require.NoError(t, platformdb.DB.Create(task).Error)
}

func TestTaskSnapshotEqualSame(t *testing.T) {
	s := TaskSnapshot{
		Status:     workflowschema.TaskStatusInProgress,
		Progress:   "50%",
		StartTime:  1000,
		FinishTime: 0,
		FailReason: "",
		ResultURL:  "",
		Data:       json.RawMessage(`{"key":"value"}`),
	}
	assert.True(t, s.Equal(s))
}

func TestTaskSnapshotEqualDifferentFields(t *testing.T) {
	a := TaskSnapshot{Status: workflowschema.TaskStatusInProgress, Progress: "30%", Data: json.RawMessage(`{"a":1}`)}
	b := TaskSnapshot{Status: workflowschema.TaskStatusSuccess, Progress: "60%", Data: json.RawMessage(`{"a":2}`)}
	assert.False(t, a.Equal(b))
}

func TestTaskSnapshotEqualNilVsEmpty(t *testing.T) {
	a := TaskSnapshot{Status: workflowschema.TaskStatusInProgress, Data: nil}
	b := TaskSnapshot{Status: workflowschema.TaskStatusInProgress, Data: json.RawMessage{}}
	assert.True(t, a.Equal(b))
}

func TestTakeTaskSnapshotRoundtrip(t *testing.T) {
	task := &workflowschema.Task{
		Status:     workflowschema.TaskStatusInProgress,
		Progress:   "42%",
		StartTime:  1234,
		FinishTime: 5678,
		FailReason: "timeout",
		PrivateData: workflowschema.TaskPrivateData{
			ResultURL: "https://example.com/result.mp4",
		},
		Data: json.RawMessage(`{"model":"test-model"}`),
	}
	snap := TakeTaskSnapshot(task)
	assert.Equal(t, task.Status, snap.Status)
	assert.Equal(t, task.Progress, snap.Progress)
	assert.Equal(t, task.StartTime, snap.StartTime)
	assert.Equal(t, task.FinishTime, snap.FinishTime)
	assert.Equal(t, task.FailReason, snap.FailReason)
	assert.Equal(t, task.PrivateData.ResultURL, snap.ResultURL)
	assert.JSONEq(t, string(task.Data), string(snap.Data))
}

func TestUpdateTaskWithStatusWin(t *testing.T) {
	truncateTasks(t)

	task := &workflowschema.Task{
		TaskID:   "task_cas_win",
		Status:   workflowschema.TaskStatusInProgress,
		Progress: "50%",
		Data:     json.RawMessage(`{}`),
	}
	insertTask(t, task)

	task.Status = workflowschema.TaskStatusSuccess
	task.Progress = "100%"
	won, err := UpdateTaskWithStatus(task, workflowschema.TaskStatusInProgress)
	require.NoError(t, err)
	assert.True(t, won)

	var reloaded workflowschema.Task
	require.NoError(t, platformdb.DB.First(&reloaded, task.ID).Error)
	assert.EqualValues(t, workflowschema.TaskStatusSuccess, reloaded.Status)
	assert.Equal(t, "100%", reloaded.Progress)
}

func TestUpdateTaskWithStatusLose(t *testing.T) {
	truncateTasks(t)

	task := &workflowschema.Task{
		TaskID: "task_cas_lose",
		Status: workflowschema.TaskStatusFailure,
		Data:   json.RawMessage(`{}`),
	}
	insertTask(t, task)

	task.Status = workflowschema.TaskStatusSuccess
	won, err := UpdateTaskWithStatus(task, workflowschema.TaskStatusInProgress)
	require.NoError(t, err)
	assert.False(t, won)

	var reloaded workflowschema.Task
	require.NoError(t, platformdb.DB.First(&reloaded, task.ID).Error)
	assert.EqualValues(t, workflowschema.TaskStatusFailure, reloaded.Status)
}

func TestUpdateTaskWithStatusConcurrentWinner(t *testing.T) {
	truncateTasks(t)

	task := &workflowschema.Task{
		TaskID: "task_cas_race",
		Status: workflowschema.TaskStatusInProgress,
		Quota:  1000,
		Data:   json.RawMessage(`{}`),
	}
	insertTask(t, task)

	const goroutines = 5
	wins := make([]bool, goroutines)
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			t := &workflowschema.Task{
				ID:        task.ID,
				TaskID:    task.TaskID,
				Status:    workflowschema.TaskStatusSuccess,
				Progress:  "100%",
				Quota:     task.Quota,
				Data:      json.RawMessage(`{}`),
				CreatedAt: task.CreatedAt,
				UpdatedAt: time.Now().Unix(),
			}
			won, err := UpdateTaskWithStatus(t, workflowschema.TaskStatusInProgress)
			if err == nil {
				wins[idx] = won
			}
		}(i)
	}
	wg.Wait()

	winCount := 0
	for _, w := range wins {
		if w {
			winCount++
		}
	}
	assert.Equal(t, 1, winCount)
}
