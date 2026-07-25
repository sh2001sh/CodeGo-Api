package http

import (
	"github.com/gin-gonic/gin"
	"github.com/sh2001sh/CodeGo-Api/constant"
	gatewayschema "github.com/sh2001sh/CodeGo-Api/internal/gateway/schema"
	gatewaystore "github.com/sh2001sh/CodeGo-Api/internal/gateway/store"
	platformencoding "github.com/sh2001sh/CodeGo-Api/internal/platform/encodingx"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCopyChannelReturnsPersistedID(t *testing.T) {
	db := setupModelListControllerTestDB(t)

	tag := "alpha"
	priority := int64(9)
	weight := uint(7)
	origin := gatewayschema.Channel{
		Type:        1,
		Key:         "secret-key",
		Status:      constant.ChannelStatusEnabled,
		Name:        "origin",
		Weight:      &weight,
		CreatedTime: 100,
		Balance:     88.5,
		Models:      "gpt-4o,gpt-4.1",
		Group:       "default",
		UsedQuota:   321,
		Priority:    &priority,
		Tag:         &tag,
	}
	require.NoError(t, db.Create(&origin).Error)
	require.NoError(t, gatewaystore.AddChannelAbilities(&origin, nil))

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = gin.Params{{Key: "id", Value: "1"}}
	ctx.Request = httptest.NewRequest(http.MethodPost, "/channel/copy/1?suffix=_dup&reset_balance=true", nil)

	CopyChannel(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	body := recorder.Body.String()
	require.Contains(t, body, "\"success\":true")

	var payload struct {
		Success bool `json:"success"`
		Data    struct {
			ID int `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, platformencoding.Unmarshal(recorder.Body.Bytes(), &payload))
	require.True(t, payload.Success)
	require.NotZero(t, payload.Data.ID)
	require.NotEqual(t, origin.Id, payload.Data.ID)

	cloned, err := gatewaystore.LoadChannelByID(payload.Data.ID, true)
	require.NoError(t, err)
	require.Equal(t, "origin_dup", cloned.Name)
	require.Equal(t, origin.Key, cloned.Key)
	require.Zero(t, cloned.Balance)
	require.Zero(t, cloned.UsedQuota)

	var abilities []gatewayschema.Ability
	require.NoError(t, db.Where("channel_id = ?", payload.Data.ID).Find(&abilities).Error)
	require.NotEmpty(t, abilities)
}

func TestEditTagChannelsRejectsInvalidParamOverride(t *testing.T) {
	setupModelListControllerTestDB(t)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(
		http.MethodPut,
		"/channel/tag",
		strings.NewReader(`{"tag":"alpha","param_override":"{"}`),
	)
	ctx.Request.Header.Set("Content-Type", "application/json")

	EditTagChannels(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{"success":false,"message":"参数覆盖必须是合法的 JSON 格式"}`, recorder.Body.String())
}

func TestGetTagModelsReturnsLongestModelsString(t *testing.T) {
	db := setupModelListControllerTestDB(t)

	tag := "shared"
	require.NoError(t, db.Create(&[]gatewayschema.Channel{
		{
			Type:   1,
			Key:    "k1",
			Status: constant.ChannelStatusEnabled,
			Name:   "short",
			Models: "gpt-4o",
			Group:  "default",
			Tag:    &tag,
		},
		{
			Type:   1,
			Key:    "k2",
			Status: constant.ChannelStatusEnabled,
			Name:   "long",
			Models: "gpt-4o,gpt-4.1,o3",
			Group:  "default",
			Tag:    &tag,
		},
	}).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/channel/tag/models?tag=shared", nil)

	GetTagModels(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{"success":true,"message":"","data":"gpt-4o,gpt-4.1,o3"}`, recorder.Body.String())
}
