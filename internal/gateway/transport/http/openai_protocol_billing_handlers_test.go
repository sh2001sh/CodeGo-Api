package http

import (
	"github.com/gin-gonic/gin"
	"github.com/sh2001sh/CodeGo-Api/constant"
	identityschema "github.com/sh2001sh/CodeGo-Api/internal/identity/schema"
	platformconfig "github.com/sh2001sh/CodeGo-Api/internal/platform/config"
	platformruntime "github.com/sh2001sh/CodeGo-Api/internal/platform/runtime"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetOpenAIProtocolSubscriptionReturnsUserBillingSnapshot(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	originalDisplayTokenStatEnabled := platformconfig.DisplayTokenStatEnabled
	platformconfig.DisplayTokenStatEnabled = false
	t.Cleanup(func() {
		platformconfig.DisplayTokenStatEnabled = originalDisplayTokenStatEnabled
	})

	user := &identityschema.User{
		Id:          1,
		Username:    "billing-user",
		Password:    "password123",
		DisplayName: "Billing User",
		Role:        constant.RoleCommonUser,
		Status:      constant.UserStatusEnabled,
		Group:       "default",
		Quota:       int(platformruntime.QuotaPerUnit * 3),
		UsedQuota:   int(platformruntime.QuotaPerUnit * 2),
	}
	require.NoError(t, db.Create(user).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v1/dashboard/billing/subscription", nil)
	ctx.Set("id", user.Id)

	GetOpenAIProtocolSubscription(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{"object":"billing_subscription","has_payment_method":true,"soft_limit_usd":5,"hard_limit_usd":5,"system_hard_limit_usd":5,"access_until":0}`, recorder.Body.String())
}

func TestGetOpenAIProtocolSubscriptionReturnsUnlimitedTokenBalance(t *testing.T) {
	db := setupModelListControllerTestDB(t)

	originalDisplayTokenStatEnabled := platformconfig.DisplayTokenStatEnabled
	platformconfig.DisplayTokenStatEnabled = true
	t.Cleanup(func() {
		platformconfig.DisplayTokenStatEnabled = originalDisplayTokenStatEnabled
	})

	user := &identityschema.User{
		Id:          1,
		Username:    "billing-token-user",
		Password:    "password123",
		DisplayName: "Billing Token User",
		Role:        constant.RoleCommonUser,
		Status:      constant.UserStatusEnabled,
		Group:       "default",
	}
	require.NoError(t, db.Create(user).Error)
	token := &identityschema.Token{
		Id:             1,
		UserId:         user.Id,
		Name:           "token-billing",
		Key:            "token-billing-key",
		Status:         constant.TokenStatusEnabled,
		CreatedTime:    1,
		AccessedTime:   1,
		ExpiredTime:    123456,
		RemainQuota:    100,
		UsedQuota:      50,
		UnlimitedQuota: true,
		Group:          "default",
	}
	require.NoError(t, db.Create(token).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v1/dashboard/billing/subscription", nil)
	ctx.Set("token_id", token.Id)

	GetOpenAIProtocolSubscription(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{"object":"billing_subscription","has_payment_method":true,"soft_limit_usd":100000000,"hard_limit_usd":100000000,"system_hard_limit_usd":100000000,"access_until":123456}`, recorder.Body.String())
}

func TestGetOpenAIProtocolUsageReturnsUserUsage(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	originalDisplayTokenStatEnabled := platformconfig.DisplayTokenStatEnabled
	platformconfig.DisplayTokenStatEnabled = false
	t.Cleanup(func() {
		platformconfig.DisplayTokenStatEnabled = originalDisplayTokenStatEnabled
	})

	user := &identityschema.User{
		Id:          1,
		Username:    "usage-user",
		Password:    "password123",
		DisplayName: "Usage User",
		Role:        constant.RoleCommonUser,
		Status:      constant.UserStatusEnabled,
		Group:       "default",
		UsedQuota:   int(platformruntime.QuotaPerUnit * 2),
	}
	require.NoError(t, db.Create(user).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v1/dashboard/billing/usage", nil)
	ctx.Set("id", user.Id)

	GetOpenAIProtocolUsage(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{"object":"list","total_usage":200}`, recorder.Body.String())
}
