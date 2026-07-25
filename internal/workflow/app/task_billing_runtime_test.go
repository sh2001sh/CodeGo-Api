package app

import (
	"context"
	"encoding/json"
	"github.com/glebarez/sqlite"
	"github.com/sh2001sh/CodeGo-Api/constant"
	auditschema "github.com/sh2001sh/CodeGo-Api/internal/audit/schema"
	billingschema "github.com/sh2001sh/CodeGo-Api/internal/billing/schema"
	commerceschema "github.com/sh2001sh/CodeGo-Api/internal/commerce/schema"
	relaycommon "github.com/sh2001sh/CodeGo-Api/internal/gateway/runtime"
	gatewayschema "github.com/sh2001sh/CodeGo-Api/internal/gateway/schema"
	identityschema "github.com/sh2001sh/CodeGo-Api/internal/identity/schema"
	platformcache "github.com/sh2001sh/CodeGo-Api/internal/platform/cache"
	platformconfig "github.com/sh2001sh/CodeGo-Api/internal/platform/config"
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
	workflowdomain "github.com/sh2001sh/CodeGo-Api/internal/workflow/domain"
	workflowschema "github.com/sh2001sh/CodeGo-Api/internal/workflow/schema"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"net/http"
	"os"
	"testing"
	"time"
)

const (
	testBillingSourceWallet       = "wallet"
	testBillingSourceSubscription = "subscription"
)

func TestMain(m *testing.M) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		panic("failed to open test db: " + err.Error())
	}
	sqlDB, err := db.DB()
	if err != nil {
		panic("failed to get sql.DB: " + err.Error())
	}
	sqlDB.SetMaxOpenConns(1)

	platformdb.DB = db
	platformdb.LogDB = db
	platformdb.UsingSQLite = true
	platformcache.RedisEnabled = false
	platformconfig.BatchUpdateEnabled = false
	platformconfig.LogConsumeEnabled = true

	if err := db.AutoMigrate(
		&workflowschema.Task{},
		&identityschema.User{},
		&identityschema.Token{},
		&auditschema.Log{},
		&gatewayschema.Channel{},
		&commerceschema.TopUp{},
		&billingschema.BillingAccount{},
		&billingschema.BillingBalanceSnapshot{},
		&billingschema.BillingLedgerEntry{},
		&billingschema.BillingReservation{},
		&billingschema.BillingSettlement{},
		&billingschema.BillingOutboxEvent{},
		&commerceschema.SubscriptionPlan{},
		&commerceschema.UserSubscription{},
	); err != nil {
		panic("failed to migrate: " + err.Error())
	}

	os.Exit(m.Run())
}

func truncateTables(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		platformdb.DB.Exec("DELETE FROM tasks")
		platformdb.DB.Exec("DELETE FROM users")
		platformdb.DB.Exec("DELETE FROM tokens")
		platformdb.DB.Exec("DELETE FROM logs")
		platformdb.DB.Exec("DELETE FROM channels")
		platformdb.DB.Exec("DELETE FROM top_ups")
		platformdb.DB.Exec("DELETE FROM billing_settlements")
		platformdb.DB.Exec("DELETE FROM billing_reservations")
		platformdb.DB.Exec("DELETE FROM billing_ledger_entries")
		platformdb.DB.Exec("DELETE FROM billing_balance_snapshots")
		platformdb.DB.Exec("DELETE FROM billing_accounts")
		platformdb.DB.Exec("DELETE FROM subscription_plans")
		platformdb.DB.Exec("DELETE FROM user_subscriptions")
	})
}

func seedUser(t *testing.T, id int, quota int) {
	t.Helper()
	user := &identityschema.User{Id: id, Username: "test_user", Quota: quota, Status: constant.UserStatusEnabled}
	require.NoError(t, platformdb.DB.Create(user).Error)
}

func seedToken(t *testing.T, id int, userID int, key string, remainQuota int) {
	t.Helper()
	token := &identityschema.Token{
		Id:          id,
		UserId:      userID,
		Key:         key,
		Name:        "test_token",
		Status:      constant.TokenStatusEnabled,
		RemainQuota: remainQuota,
		UsedQuota:   0,
	}
	require.NoError(t, platformdb.DB.Create(token).Error)
}

func seedSubscriptionPlan(t *testing.T, id int) {
	t.Helper()
	plan := &commerceschema.SubscriptionPlan{
		Id:               id,
		Title:            "Test月卡",
		Subtitle:         "月卡",
		PriceAmount:      99,
		Currency:         "CNY",
		DurationUnit:     commerceschema.SubscriptionDurationMonth,
		DurationValue:    1,
		Enabled:          true,
		TotalAmount:      100000,
		PeriodAmount:     100000,
		QuotaResetPeriod: commerceschema.SubscriptionResetMonthly,
	}
	require.NoError(t, platformdb.DB.Create(plan).Error)
}

func seedSubscription(t *testing.T, id int, userID int, planID int, amountTotal int64, amountUsed int64) {
	t.Helper()
	sub := &commerceschema.UserSubscription{
		Id:          id,
		UserId:      userID,
		PlanId:      planID,
		AmountTotal: amountTotal,
		AmountUsed:  amountUsed,
		Status:      "active",
		StartTime:   time.Now().Unix(),
		EndTime:     time.Now().Add(30 * 24 * time.Hour).Unix(),
	}
	require.NoError(t, platformdb.DB.Create(sub).Error)
}

func seedChannel(t *testing.T, id int) {
	t.Helper()
	ch := &gatewayschema.Channel{Id: id, Name: "test_channel", Key: "sk-test", Status: constant.ChannelStatusEnabled}
	require.NoError(t, platformdb.DB.Create(ch).Error)
}

func makeTask(userID, channelID, quota, tokenID int, billingSource string, subscriptionID int) *workflowschema.Task {
	return &workflowschema.Task{
		TaskID:    "task_" + time.Now().Format("150405.000"),
		UserId:    userID,
		ChannelId: channelID,
		Quota:     quota,
		Status:    workflowschema.TaskStatus(workflowschema.TaskStatusInProgress),
		Group:     "default",
		Data:      json.RawMessage(`{}`),
		CreatedAt: time.Now().Unix(),
		UpdatedAt: time.Now().Unix(),
		Properties: workflowschema.Properties{
			OriginModelName: "test-model",
		},
		PrivateData: workflowschema.TaskPrivateData{
			BillingSource:  billingSource,
			SubscriptionId: subscriptionID,
			TokenId:        tokenID,
			BillingContext: &workflowschema.TaskBillingContext{
				ModelPrice:      0.02,
				GroupRatio:      1.0,
				OriginModelName: "test-model",
			},
		},
	}
}

func getUserQuota(t *testing.T, id int) int {
	t.Helper()
	var user identityschema.User
	require.NoError(t, platformdb.DB.Select("quota").Where("id = ?", id).First(&user).Error)
	return user.Quota
}

func getTokenRemainQuota(t *testing.T, id int) int {
	t.Helper()
	var token identityschema.Token
	require.NoError(t, platformdb.DB.Select("remain_quota").Where("id = ?", id).First(&token).Error)
	return token.RemainQuota
}

func getTokenUsedQuota(t *testing.T, id int) int {
	t.Helper()
	var token identityschema.Token
	require.NoError(t, platformdb.DB.Select("used_quota").Where("id = ?", id).First(&token).Error)
	return token.UsedQuota
}

func getSubscriptionUsed(t *testing.T, id int) int64 {
	t.Helper()
	var sub commerceschema.UserSubscription
	require.NoError(t, platformdb.DB.Select("amount_used").Where("id = ?", id).First(&sub).Error)
	return sub.AmountUsed
}

func getLastLog(t *testing.T) *auditschema.Log {
	t.Helper()
	var log auditschema.Log
	err := platformdb.LogDB.Order("id desc").First(&log).Error
	if err != nil {
		return nil
	}
	return &log
}

func countLogs(t *testing.T) int64 {
	t.Helper()
	var count int64
	platformdb.LogDB.Model(&auditschema.Log{}).Count(&count)
	return count
}

func TestRecordTaskRefund_Wallet(t *testing.T) {
	truncateTables(t)
	ctx := context.Background()

	const userID, tokenID, channelID = 1, 1, 1
	const initQuota, preConsumed = 10000, 3000
	const tokenRemain = 5000

	seedUser(t, userID, initQuota)
	seedToken(t, tokenID, userID, "sk-test-key", tokenRemain)
	seedChannel(t, channelID)

	task := makeTask(userID, channelID, preConsumed, tokenID, testBillingSourceWallet, 0)

	recordTaskRefund(ctx, task, "task failed: upstream error")

	assert.Equal(t, initQuota+preConsumed, getUserQuota(t, userID))
	assert.Equal(t, tokenRemain+preConsumed, getTokenRemainQuota(t, tokenID))
	assert.Equal(t, -preConsumed, getTokenUsedQuota(t, tokenID))

	log := getLastLog(t)
	require.NotNil(t, log)
	assert.Equal(t, auditschema.LogTypeRefund, log.Type)
	assert.Equal(t, preConsumed, log.Quota)
	assert.Equal(t, "test-model", log.ModelName)
}

func TestRecordTaskRefund_Subscription(t *testing.T) {
	truncateTables(t)
	ctx := context.Background()

	const userID, tokenID, channelID, subID = 2, 2, 2, 1
	const planID = 1001
	const preConsumed = 2000
	const subTotal, subUsed int64 = 100000, 50000
	const tokenRemain = 8000

	seedUser(t, userID, 0)
	seedToken(t, tokenID, userID, "sk-sub-key", tokenRemain)
	seedChannel(t, channelID)
	seedSubscriptionPlan(t, planID)
	seedSubscription(t, subID, userID, planID, subTotal, subUsed)

	task := makeTask(userID, channelID, preConsumed, tokenID, testBillingSourceSubscription, subID)

	recordTaskRefund(ctx, task, "subscription task failed")

	assert.Equal(t, subUsed-int64(preConsumed), getSubscriptionUsed(t, subID))
	assert.Equal(t, tokenRemain+preConsumed, getTokenRemainQuota(t, tokenID))

	log := getLastLog(t)
	require.NotNil(t, log)
	assert.Equal(t, auditschema.LogTypeRefund, log.Type)
}

func TestRecordTaskRefund_ZeroQuota(t *testing.T) {
	truncateTables(t)
	ctx := context.Background()

	const userID = 3
	seedUser(t, userID, 5000)

	task := makeTask(userID, 0, 0, 0, testBillingSourceWallet, 0)

	recordTaskRefund(ctx, task, "zero quota task")

	assert.Equal(t, 5000, getUserQuota(t, userID))
	assert.Equal(t, int64(0), countLogs(t))
}

func TestRecordTaskRefund_NoToken(t *testing.T) {
	truncateTables(t)
	ctx := context.Background()

	const userID, channelID = 4, 4
	const initQuota, preConsumed = 10000, 1500

	seedUser(t, userID, initQuota)
	seedChannel(t, channelID)

	task := makeTask(userID, channelID, preConsumed, 0, testBillingSourceWallet, 0)

	recordTaskRefund(ctx, task, "no token task failed")

	assert.Equal(t, initQuota+preConsumed, getUserQuota(t, userID))

	log := getLastLog(t)
	require.NotNil(t, log)
	assert.Equal(t, auditschema.LogTypeRefund, log.Type)
}

func TestSettleTaskQuotaDelta_PositiveDelta(t *testing.T) {
	truncateTables(t)
	ctx := context.Background()

	const userID, tokenID, channelID = 10, 10, 10
	const initQuota, preConsumed = 10000, 2000
	const actualQuota = 3000
	const tokenRemain = 5000

	seedUser(t, userID, initQuota)
	seedToken(t, tokenID, userID, "sk-recalc-pos", tokenRemain)
	seedChannel(t, channelID)

	task := makeTask(userID, channelID, preConsumed, tokenID, testBillingSourceWallet, 0)
	require.NoError(t, platformdb.DB.Create(task).Error)

	settleTaskQuotaDelta(ctx, task, actualQuota, "adaptor adjustment")

	assert.Equal(t, initQuota-(actualQuota-preConsumed), getUserQuota(t, userID))
	assert.Equal(t, tokenRemain-(actualQuota-preConsumed), getTokenRemainQuota(t, tokenID))
	assert.Equal(t, actualQuota, task.Quota)

	var reloaded workflowschema.Task
	require.NoError(t, platformdb.DB.First(&reloaded, task.ID).Error)
	assert.Equal(t, actualQuota, reloaded.Quota)

	log := getLastLog(t)
	require.NotNil(t, log)
	assert.Equal(t, auditschema.LogTypeConsume, log.Type)
	assert.Equal(t, actualQuota-preConsumed, log.Quota)
}

func TestSettleTaskQuotaDelta_NegativeDelta(t *testing.T) {
	truncateTables(t)
	ctx := context.Background()

	const userID, tokenID, channelID = 11, 11, 11
	const initQuota, preConsumed = 10000, 5000
	const actualQuota = 3000
	const tokenRemain = 5000

	seedUser(t, userID, initQuota)
	seedToken(t, tokenID, userID, "sk-recalc-neg", tokenRemain)
	seedChannel(t, channelID)

	task := makeTask(userID, channelID, preConsumed, tokenID, testBillingSourceWallet, 0)

	settleTaskQuotaDelta(ctx, task, actualQuota, "adaptor adjustment")

	assert.Equal(t, initQuota+(preConsumed-actualQuota), getUserQuota(t, userID))
	assert.Equal(t, tokenRemain+(preConsumed-actualQuota), getTokenRemainQuota(t, tokenID))
	assert.Equal(t, actualQuota, task.Quota)

	log := getLastLog(t)
	require.NotNil(t, log)
	assert.Equal(t, auditschema.LogTypeRefund, log.Type)
	assert.Equal(t, preConsumed-actualQuota, log.Quota)
}

func TestSettleTaskQuotaDelta_ZeroDelta(t *testing.T) {
	truncateTables(t)
	ctx := context.Background()

	const userID = 12
	const initQuota, preConsumed = 10000, 3000

	seedUser(t, userID, initQuota)

	task := makeTask(userID, 0, preConsumed, 0, testBillingSourceWallet, 0)

	settleTaskQuotaDelta(ctx, task, preConsumed, "exact match")

	assert.Equal(t, initQuota, getUserQuota(t, userID))
	assert.Equal(t, int64(0), countLogs(t))
}

func TestSettleTaskQuotaDelta_ActualQuotaZero(t *testing.T) {
	truncateTables(t)
	ctx := context.Background()

	const userID = 13
	const initQuota = 10000

	seedUser(t, userID, initQuota)

	task := makeTask(userID, 0, 5000, 0, testBillingSourceWallet, 0)

	settleTaskQuotaDelta(ctx, task, 0, "zero actual")

	assert.Equal(t, initQuota, getUserQuota(t, userID))
	assert.Equal(t, int64(0), countLogs(t))
}

func TestSettleTaskQuotaDelta_SubscriptionNegativeDelta(t *testing.T) {
	truncateTables(t)
	ctx := context.Background()

	const userID, tokenID, channelID, subID = 14, 14, 14, 2
	const planID = 1002
	const preConsumed = 5000
	const actualQuota = 2000
	const subTotal, subUsed int64 = 100000, 50000
	const tokenRemain = 8000

	seedUser(t, userID, 0)
	seedToken(t, tokenID, userID, "sk-sub-recalc", tokenRemain)
	seedChannel(t, channelID)
	seedSubscriptionPlan(t, planID)
	seedSubscription(t, subID, userID, planID, subTotal, subUsed)

	task := makeTask(userID, channelID, preConsumed, tokenID, testBillingSourceSubscription, subID)

	settleTaskQuotaDelta(ctx, task, actualQuota, "subscription over-charge")

	assert.Equal(t, subUsed-int64(preConsumed-actualQuota), getSubscriptionUsed(t, subID))
	assert.Equal(t, tokenRemain+(preConsumed-actualQuota), getTokenRemainQuota(t, tokenID))
	assert.Equal(t, actualQuota, task.Quota)

	log := getLastLog(t)
	require.NotNil(t, log)
	assert.Equal(t, auditschema.LogTypeRefund, log.Type)
}

func simulatePollBilling(ctx context.Context, task *workflowschema.Task, newStatus workflowschema.TaskStatus, actualQuota int) {
	snap := workflowdomain.TakeTaskSnapshot(task)

	shouldRefund := false
	shouldSettle := false
	quota := task.Quota

	task.Status = newStatus
	switch string(newStatus) {
	case workflowschema.TaskStatusSuccess:
		task.Progress = "100%"
		task.FinishTime = 9999
		shouldSettle = true
	case workflowschema.TaskStatusFailure:
		task.Progress = "100%"
		task.FinishTime = 9999
		task.FailReason = "upstream error"
		if quota != 0 {
			shouldRefund = true
		}
	default:
		task.Progress = "50%"
	}

	isDone := task.Status == workflowschema.TaskStatus(workflowschema.TaskStatusSuccess) || task.Status == workflowschema.TaskStatus(workflowschema.TaskStatusFailure)
	if isDone && snap.Status != task.Status {
		won, err := workflowdomain.UpdateTaskWithStatus(task, snap.Status)
		if err != nil || !won {
			shouldRefund = false
			shouldSettle = false
		}
	} else if !snap.Equal(workflowdomain.TakeTaskSnapshot(task)) {
		_, _ = workflowdomain.UpdateTaskWithStatus(task, snap.Status)
	}

	if shouldSettle && actualQuota > 0 {
		settleTaskQuotaDelta(ctx, task, actualQuota, "test settle")
	}
	if shouldRefund {
		recordTaskRefund(ctx, task, task.FailReason)
	}
}

func TestCASGuardedRefund_Win(t *testing.T) {
	truncateTables(t)
	ctx := context.Background()

	const userID, tokenID, channelID = 20, 20, 20
	const initQuota, preConsumed = 10000, 4000
	const tokenRemain = 6000

	seedUser(t, userID, initQuota)
	seedToken(t, tokenID, userID, "sk-cas-refund-win", tokenRemain)
	seedChannel(t, channelID)

	task := makeTask(userID, channelID, preConsumed, tokenID, testBillingSourceWallet, 0)
	task.Status = workflowschema.TaskStatus(workflowschema.TaskStatusInProgress)
	require.NoError(t, platformdb.DB.Create(task).Error)

	simulatePollBilling(ctx, task, workflowschema.TaskStatus(workflowschema.TaskStatusFailure), 0)

	var reloaded workflowschema.Task
	require.NoError(t, platformdb.DB.First(&reloaded, task.ID).Error)
	assert.EqualValues(t, workflowschema.TaskStatusFailure, reloaded.Status)
	assert.Equal(t, initQuota+preConsumed, getUserQuota(t, userID))
	assert.Equal(t, tokenRemain+preConsumed, getTokenRemainQuota(t, tokenID))

	log := getLastLog(t)
	require.NotNil(t, log)
	assert.Equal(t, auditschema.LogTypeRefund, log.Type)
}

func TestCASGuardedRefund_Lose(t *testing.T) {
	truncateTables(t)
	ctx := context.Background()

	const userID, tokenID, channelID = 21, 21, 21
	const initQuota, preConsumed = 10000, 4000
	const tokenRemain = 6000

	seedUser(t, userID, initQuota)
	seedToken(t, tokenID, userID, "sk-cas-refund-lose", tokenRemain)
	seedChannel(t, channelID)

	task := makeTask(userID, channelID, preConsumed, tokenID, testBillingSourceWallet, 0)
	task.Status = workflowschema.TaskStatus(workflowschema.TaskStatusInProgress)
	require.NoError(t, platformdb.DB.Create(task).Error)

	platformdb.DB.Model(&workflowschema.Task{}).Where("id = ?", task.ID).Update("status", workflowschema.TaskStatusFailure)

	simulatePollBilling(ctx, task, workflowschema.TaskStatus(workflowschema.TaskStatusFailure), 0)

	assert.Equal(t, initQuota, getUserQuota(t, userID))
	assert.Equal(t, tokenRemain, getTokenRemainQuota(t, tokenID))
	assert.Equal(t, int64(0), countLogs(t))
}

func TestCASGuardedSettle_Win(t *testing.T) {
	truncateTables(t)
	ctx := context.Background()

	const userID, tokenID, channelID = 22, 22, 22
	const initQuota, preConsumed = 10000, 5000
	const actualQuota = 3000
	const tokenRemain = 8000

	seedUser(t, userID, initQuota)
	seedToken(t, tokenID, userID, "sk-cas-settle-win", tokenRemain)
	seedChannel(t, channelID)

	task := makeTask(userID, channelID, preConsumed, tokenID, testBillingSourceWallet, 0)
	task.Status = workflowschema.TaskStatus(workflowschema.TaskStatusInProgress)
	require.NoError(t, platformdb.DB.Create(task).Error)

	simulatePollBilling(ctx, task, workflowschema.TaskStatus(workflowschema.TaskStatusSuccess), actualQuota)

	var reloaded workflowschema.Task
	require.NoError(t, platformdb.DB.First(&reloaded, task.ID).Error)
	assert.EqualValues(t, workflowschema.TaskStatusSuccess, reloaded.Status)
	assert.Equal(t, initQuota+(preConsumed-actualQuota), getUserQuota(t, userID))
	assert.Equal(t, tokenRemain+(preConsumed-actualQuota), getTokenRemainQuota(t, tokenID))
	assert.Equal(t, actualQuota, task.Quota)
}

func TestNonTerminalUpdate_NoBilling(t *testing.T) {
	truncateTables(t)
	ctx := context.Background()

	const userID, channelID = 23, 23
	const initQuota, preConsumed = 10000, 3000

	seedUser(t, userID, initQuota)
	seedChannel(t, channelID)

	task := makeTask(userID, channelID, preConsumed, 0, testBillingSourceWallet, 0)
	task.Status = workflowschema.TaskStatus(workflowschema.TaskStatusInProgress)
	task.Progress = "20%"
	require.NoError(t, platformdb.DB.Create(task).Error)

	simulatePollBilling(ctx, task, workflowschema.TaskStatus(workflowschema.TaskStatusInProgress), 0)

	assert.Equal(t, initQuota, getUserQuota(t, userID))
	assert.Equal(t, int64(0), countLogs(t))

	var reloaded workflowschema.Task
	require.NoError(t, platformdb.DB.First(&reloaded, task.ID).Error)
	assert.Equal(t, "50%", reloaded.Progress)
}

type mockAdaptor struct {
	adjustReturn int
}

func (m *mockAdaptor) Init(_ *relaycommon.RelayInfo) {}

func (m *mockAdaptor) FetchTask(string, string, map[string]any, string) (*http.Response, error) {
	return nil, nil
}

func (m *mockAdaptor) ParseTaskResult([]byte) (*relaycommon.TaskInfo, error) { return nil, nil }

func (m *mockAdaptor) AdjustBillingOnComplete(_ *workflowschema.Task, _ *relaycommon.TaskInfo) int {
	return m.adjustReturn
}

func TestSettleTaskBillingOnComplete_PerCallBillingSkipsAdaptorAdjust(t *testing.T) {
	truncateTables(t)
	ctx := context.Background()

	const userID, tokenID, channelID = 30, 30, 30
	const initQuota, preConsumed = 10000, 5000
	const tokenRemain = 8000

	seedUser(t, userID, initQuota)
	seedToken(t, tokenID, userID, "sk-percall-adaptor", tokenRemain)
	seedChannel(t, channelID)

	task := makeTask(userID, channelID, preConsumed, tokenID, testBillingSourceWallet, 0)
	task.PrivateData.BillingContext.PerCallBilling = true

	adaptor := &mockAdaptor{adjustReturn: 2000}
	taskResult := &relaycommon.TaskInfo{Status: workflowschema.TaskStatusSuccess}

	settleTaskBillingOnComplete(ctx, adaptor, task, taskResult)

	assert.Equal(t, initQuota, getUserQuota(t, userID))
	assert.Equal(t, tokenRemain, getTokenRemainQuota(t, tokenID))
	assert.Equal(t, preConsumed, task.Quota)
	assert.Equal(t, int64(0), countLogs(t))
}

func TestSettleTaskBillingOnComplete_PerCallBillingSkipsTotalTokens(t *testing.T) {
	truncateTables(t)
	ctx := context.Background()

	const userID, tokenID, channelID = 31, 31, 31
	const initQuota, preConsumed = 10000, 4000
	const tokenRemain = 7000

	seedUser(t, userID, initQuota)
	seedToken(t, tokenID, userID, "sk-percall-tokens", tokenRemain)
	seedChannel(t, channelID)

	task := makeTask(userID, channelID, preConsumed, tokenID, testBillingSourceWallet, 0)
	task.PrivateData.BillingContext.PerCallBilling = true

	adaptor := &mockAdaptor{adjustReturn: 0}
	taskResult := &relaycommon.TaskInfo{Status: workflowschema.TaskStatusSuccess, TotalTokens: 9999}

	settleTaskBillingOnComplete(ctx, adaptor, task, taskResult)

	assert.Equal(t, initQuota, getUserQuota(t, userID))
	assert.Equal(t, tokenRemain, getTokenRemainQuota(t, tokenID))
	assert.Equal(t, preConsumed, task.Quota)
	assert.Equal(t, int64(0), countLogs(t))
}

func TestSettleTaskBillingOnComplete_NonPerCallAdaptorAdjustWorks(t *testing.T) {
	truncateTables(t)
	ctx := context.Background()

	const userID, tokenID, channelID = 32, 32, 32
	const initQuota, preConsumed = 10000, 5000
	const adaptorQuota = 3000
	const tokenRemain = 8000

	seedUser(t, userID, initQuota)
	seedToken(t, tokenID, userID, "sk-nonpercall-adj", tokenRemain)
	seedChannel(t, channelID)

	task := makeTask(userID, channelID, preConsumed, tokenID, testBillingSourceWallet, 0)

	adaptor := &mockAdaptor{adjustReturn: adaptorQuota}
	taskResult := &relaycommon.TaskInfo{Status: workflowschema.TaskStatusSuccess}

	settleTaskBillingOnComplete(ctx, adaptor, task, taskResult)

	assert.Equal(t, initQuota+(preConsumed-adaptorQuota), getUserQuota(t, userID))
	assert.Equal(t, tokenRemain+(preConsumed-adaptorQuota), getTokenRemainQuota(t, tokenID))
	assert.Equal(t, adaptorQuota, task.Quota)

	log := getLastLog(t)
	require.NotNil(t, log)
	assert.Equal(t, auditschema.LogTypeRefund, log.Type)
}
