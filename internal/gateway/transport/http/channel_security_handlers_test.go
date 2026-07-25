package http

import (
	"context"
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/sh2001sh/CodeGo-Api/constant"
	gatewayexecutionapp "github.com/sh2001sh/CodeGo-Api/internal/gateway/execution/app"
	gatewayschema "github.com/sh2001sh/CodeGo-Api/internal/gateway/schema"
	gatewaystore "github.com/sh2001sh/CodeGo-Api/internal/gateway/store"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetChannelKeyReturnsKey(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	weight := uint(1)
	priority := int64(1)
	channel := gatewayschema.Channel{
		Id:       1,
		Type:     1,
		Key:      "secret-key",
		Status:   constant.ChannelStatusEnabled,
		Name:     "visible",
		Weight:   &weight,
		Models:   "gpt-4o",
		Group:    "default",
		Priority: &priority,
	}
	require.NoError(t, db.Create(&channel).Error)
	require.NoError(t, gatewaystore.AddChannelAbilities(&channel, nil))

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = gin.Params{{Key: "id", Value: "1"}}
	ctx.Set("id", 1001)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/channel/1/key", nil)

	GetChannelKey(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{"success":true,"message":"获取成功","data":{"key":"secret-key"}}`, recorder.Body.String())
}

func TestRefreshCodexChannelCredentialRejectsInvalidID(t *testing.T) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = gin.Params{{Key: "id", Value: "bad"}}
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/channel/bad/codex/refresh", nil)

	RefreshCodexChannelCredential(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"success":false`)
}

func TestRefreshCodexChannelCredentialMapsFailureMessage(t *testing.T) {
	restore := gatewayexecutionapp.SetRefreshCodexCredentialForTest(func(ctx context.Context, channelID int, opts gatewayexecutionapp.CodexCredentialRefreshOptions) (*gatewayexecutionapp.CodexOAuthKey, *gatewayschema.Channel, error) {
		return nil, nil, errors.New("boom")
	})
	defer restore()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = gin.Params{{Key: "id", Value: "1"}}
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/channel/1/codex/refresh", nil)

	RefreshCodexChannelCredential(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{"success":false,"message":"刷新凭证失败，请稍后重试"}`, recorder.Body.String())
}
