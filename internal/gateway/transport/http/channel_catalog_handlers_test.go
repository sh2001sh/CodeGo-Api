package http

import (
	"github.com/gin-gonic/gin"
	"github.com/sh2001sh/CodeGo-Api/constant"
	gatewayschema "github.com/sh2001sh/CodeGo-Api/internal/gateway/schema"
	gatewaystore "github.com/sh2001sh/CodeGo-Api/internal/gateway/store"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetAllChannelsSanitizesMultiKeyChannelInfo(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	weight := uint(1)
	priority := int64(1)
	channel := gatewayschema.Channel{
		Id:       1,
		Type:     1,
		Key:      "secret",
		Status:   constant.ChannelStatusEnabled,
		Name:     "listed",
		Weight:   &weight,
		Models:   "gpt-4o",
		Group:    "default",
		Priority: &priority,
		ChannelInfo: gatewayschema.ChannelInfo{
			IsMultiKey:   true,
			MultiKeySize: 1,
			MultiKeyMode: constant.MultiKeyModeRandom,
			MultiKeyDisabledReason: map[int]string{
				0: "hidden",
			},
			MultiKeyDisabledTime: map[int]int64{
				0: 123,
			},
		},
	}
	require.NoError(t, db.Create(&channel).Error)
	require.NoError(t, gatewaystore.AddChannelAbilities(&channel, nil))

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/channel?p=1&page_size=20", nil)

	GetAllChannels(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	body := recorder.Body.String()
	require.Contains(t, body, "\"success\":true")
	require.NotContains(t, body, "hidden")
	require.NotContains(t, body, "\"multi_key_disabled_time\"")
}

func TestAddChannelBatchPrefixDoesNotAccumulate(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(
		http.MethodPost,
		"/api/channel",
		strings.NewReader(`{
			"mode":"batch",
			"batch_add_set_key_prefix_2_name":true,
			"channel":{
				"type":1,
				"key":"alpha123456\nbeta654321",
				"name":"batch",
				"models":"gpt-4o",
				"group":"default",
				"status":1
			}
		}`),
	)
	ctx.Request.Header.Set("Content-Type", "application/json")

	AddChannel(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{"success":true,"message":""}`, recorder.Body.String())

	var channels []gatewayschema.Channel
	require.NoError(t, db.Order("id asc").Find(&channels).Error)
	require.Len(t, channels, 2)
	require.Equal(t, "batch alpha123", channels[0].Name)
	require.Equal(t, "batch beta6543", channels[1].Name)
}

func TestUpdateChannelAppendDedupesKeys(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	weight := uint(1)
	priority := int64(1)
	channel := gatewayschema.Channel{
		Id:       1,
		Type:     1,
		Key:      "first-key\nsecond-key",
		Status:   constant.ChannelStatusEnabled,
		Name:     "editable",
		Weight:   &weight,
		Models:   "gpt-4o",
		Group:    "default",
		Priority: &priority,
		ChannelInfo: gatewayschema.ChannelInfo{
			IsMultiKey:   true,
			MultiKeySize: 2,
			MultiKeyMode: constant.MultiKeyModeRandom,
		},
	}
	require.NoError(t, db.Create(&channel).Error)
	require.NoError(t, gatewaystore.AddChannelAbilities(&channel, nil))

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(
		http.MethodPut,
		"/api/channel",
		strings.NewReader(`{
			"id":1,
			"type":1,
			"name":"editable",
			"models":"gpt-4o",
			"group":"default",
			"status":1,
			"key":"second-key\nthird-key",
			"key_mode":"append"
		}`),
	)
	ctx.Request.Header.Set("Content-Type", "application/json")

	UpdateChannel(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"success":true`)
	require.NotContains(t, recorder.Body.String(), `"key":"first-key`)

	updated, err := gatewaystore.LoadChannelByID(1, true)
	require.NoError(t, err)
	require.Equal(t, "first-key\nsecond-key\nthird-key", updated.Key)
}
