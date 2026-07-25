package http

import (
	"bytes"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/sh2001sh/CodeGo-Api/constant"
	auditschema "github.com/sh2001sh/CodeGo-Api/internal/audit/schema"
	commerceapp "github.com/sh2001sh/CodeGo-Api/internal/commerce/app"
	commerceschema "github.com/sh2001sh/CodeGo-Api/internal/commerce/schema"
	identityschema "github.com/sh2001sh/CodeGo-Api/internal/identity/schema"
	platformencoding "github.com/sh2001sh/CodeGo-Api/internal/platform/encodingx"
	platformruntime "github.com/sh2001sh/CodeGo-Api/internal/platform/runtime"
	stdhttp "net/http"
	"net/http/httptest"
	"testing"
)

func TestBindAdminSubscriptionCreatesUserSubscription(t *testing.T) {
	db := setupCommerceHTTPTestDB(t)
	confirmTopupComplianceForTest(t)

	user := &identityschema.User{
		Id:          7,
		Username:    "admin-bind-target",
		Password:    "password123",
		DisplayName: "Admin Bind Target",
		Role:        constant.RoleCommonUser,
		Status:      constant.UserStatusEnabled,
		Group:       "default",
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}
	plan := &commerceschema.SubscriptionPlan{
		Id:            9,
		Title:         "Pro月卡",
		Enabled:       true,
		PriceAmount:   60,
		DurationUnit:  commerceschema.SubscriptionDurationMonth,
		DurationValue: 1,
		TotalAmount:   100,
	}
	if err := db.Create(plan).Error; err != nil {
		t.Fatalf("failed to seed plan: %v", err)
	}

	ctx, recorder := newCommerceContext(t, stdhttp.MethodPost, "/api/subscription/admin/bind", map[string]any{
		"user_id": user.Id,
		"plan_id": plan.Id,
	}, 1)
	bindAdminSubscription(ctx)

	response := decodeCommerceResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected admin bind subscription success, got %#v", response)
	}

	var count int64
	if err := db.Model(&commerceschema.UserSubscription{}).Where("user_id = ? AND plan_id = ?", user.Id, plan.Id).Count(&count).Error; err != nil {
		t.Fatalf("failed to count user subscriptions: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected one created user subscription, got %d", count)
	}
}

func TestResetAdminUserSubscriptionQuotaReturnsPayload(t *testing.T) {
	db := setupCommerceHTTPTestDB(t)
	user := &identityschema.User{
		Id:          11,
		Username:    "admin-reset-target",
		Password:    "password123",
		DisplayName: "Admin Reset Target",
		Role:        constant.RoleCommonUser,
		Status:      constant.UserStatusEnabled,
		Group:       "default",
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}
	plan := &commerceschema.SubscriptionPlan{
		Id:                 12,
		Title:              "Standard月卡",
		Enabled:            true,
		PriceAmount:        30,
		DurationUnit:       commerceschema.SubscriptionDurationMonth,
		DurationValue:      1,
		TotalAmount:        100,
		PeriodAmount:       100,
		QuotaResetPeriod:   commerceschema.SubscriptionResetMonthly,
		MaxPurchasePerUser: 1,
	}
	if err := db.Create(plan).Error; err != nil {
		t.Fatalf("failed to seed plan: %v", err)
	}
	subscription := &commerceschema.UserSubscription{
		Id:           13,
		UserId:       user.Id,
		PlanId:       plan.Id,
		AmountTotal:  100,
		AmountUsed:   40,
		PeriodAmount: 100,
		PeriodUsed:   20,
		StartTime:    platformruntime.GetTimestamp() - 60,
		EndTime:      platformruntime.GetTimestamp() + 3600,
		Status:       "active",
	}
	if err := db.Create(subscription).Error; err != nil {
		t.Fatalf("failed to seed subscription: %v", err)
	}

	body, err := platformencoding.Marshal(commerceapp.AdminResetUserSubscriptionQuotaRequest{AdvanceResetTime: true})
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", subscription.Id)}}
	ctx.Request = httptest.NewRequest(stdhttp.MethodPost, fmt.Sprintf("/api/subscription/admin/user_subscriptions/%d/reset", subscription.Id), bytes.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set("id", 1001)
	ctx.Set("username", "admin-user")

	resetAdminUserSubscriptionQuota(ctx)

	response := decodeCommerceResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected admin reset quota success, got %#v", response)
	}

	var reloaded commerceschema.UserSubscription
	if err := db.First(&reloaded, subscription.Id).Error; err != nil {
		t.Fatalf("failed to reload subscription: %v", err)
	}
	if reloaded.AmountUsed != 0 || reloaded.PeriodUsed != 0 {
		t.Fatalf("expected quota usage reset, got amount=%d period=%d", reloaded.AmountUsed, reloaded.PeriodUsed)
	}

	var logCount int64
	if err := db.Model(&auditschema.Log{}).Where("user_id = ? AND type = ?", user.Id, auditschema.LogTypeManage).Count(&logCount).Error; err != nil {
		t.Fatalf("failed to count admin manage logs: %v", err)
	}
	if logCount == 0 {
		t.Fatalf("expected admin reset quota log to be recorded")
	}
}
