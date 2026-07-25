package http

import (
	"github.com/gin-gonic/gin"
	"github.com/sh2001sh/CodeGo-Api/constant"
	gatewayschema "github.com/sh2001sh/CodeGo-Api/internal/gateway/schema"
	gatewaystore "github.com/sh2001sh/CodeGo-Api/internal/gateway/store"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUpdateChannelBalanceReturnsBalance(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/dashboard/billing/subscription":
			_, _ = w.Write([]byte(`{"has_payment_method":true,"hard_limit_usd":20}`))
		case "/v1/dashboard/billing/usage":
			_, _ = w.Write([]byte(`{"total_usage":500}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	baseURL := server.URL
	channel := gatewayschema.Channel{
		Id:       1,
		Type:     constant.ChannelTypeOpenAI,
		Key:      "test-key",
		Status:   constant.ChannelStatusEnabled,
		Name:     "balance-test",
		BaseURL:  &baseURL,
		Group:    "default",
		Models:   "gpt-4o",
		Balance:  0,
		TestTime: 0,
	}
	require.NoError(t, db.Create(&channel).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = gin.Params{{Key: "id", Value: "1"}}
	ctx.Request = httptest.NewRequest(http.MethodGet, "/channel/update_balance/1", nil)

	UpdateChannelBalance(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{"success":true,"message":"","balance":15}`, recorder.Body.String())

	updated, err := gatewaystore.LoadChannelByID(1, true)
	require.NoError(t, err)
	require.Equal(t, 15.0, updated.Balance)
}

func TestUpdateChannelBalanceRejectsMultiKeyChannel(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	channel := gatewayschema.Channel{
		Id:     1,
		Type:   constant.ChannelTypeOpenAI,
		Key:    "key-a\nkey-b",
		Status: constant.ChannelStatusEnabled,
		Name:   "multi-key",
		Group:  "default",
		Models: "gpt-4o",
		ChannelInfo: gatewayschema.ChannelInfo{
			IsMultiKey:   true,
			MultiKeySize: 2,
		},
	}
	require.NoError(t, db.Create(&channel).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = gin.Params{{Key: "id", Value: "1"}}
	ctx.Request = httptest.NewRequest(http.MethodGet, "/channel/update_balance/1", nil)

	UpdateChannelBalance(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{"success":false,"message":"多密钥渠道不支持余额查询"}`, recorder.Body.String())
}
