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

func createMultiKeyTestChannel(t *testing.T) gatewayschema.Channel {
	t.Helper()

	db := setupModelListControllerTestDB(t)
	weight := uint(1)
	priority := int64(1)
	channel := gatewayschema.Channel{
		Id:       1,
		Type:     1,
		Key:      "key-111111111\nkey-222222222\nauto-333333333",
		Status:   constant.ChannelStatusEnabled,
		Name:     "multi-key",
		Weight:   &weight,
		Models:   "gpt-4o",
		Group:    "default",
		Priority: &priority,
		ChannelInfo: gatewayschema.ChannelInfo{
			IsMultiKey:   true,
			MultiKeySize: 3,
			MultiKeyMode: constant.MultiKeyModeRandom,
			MultiKeyStatusList: map[int]int{
				1: constant.ChannelStatusManuallyDisabled,
				2: constant.ChannelStatusAutoDisabled,
			},
			MultiKeyDisabledTime: map[int]int64{
				1: 100,
				2: 200,
			},
			MultiKeyDisabledReason: map[int]string{
				1: "manual",
				2: "auto",
			},
		},
	}
	require.NoError(t, db.Create(&channel).Error)
	require.NoError(t, gatewaystore.AddChannelAbilities(&channel, nil))
	return channel
}

func TestManageMultiKeysGetStatus(t *testing.T) {
	createMultiKeyTestChannel(t)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(
		http.MethodPost,
		"/api/channel/multi_key/manage",
		strings.NewReader(`{"channel_id":1,"action":"get_key_status","page":1,"page_size":10}`),
	)
	ctx.Request.Header.Set("Content-Type", "application/json")

	ManageMultiKeys(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var payload struct {
		Success bool `json:"success"`
		Data    struct {
			Keys []struct {
				Index      int    `json:"index"`
				Status     int    `json:"status"`
				KeyPreview string `json:"key_preview"`
			} `json:"keys"`
			EnabledCount        int `json:"enabled_count"`
			ManualDisabledCount int `json:"manual_disabled_count"`
			AutoDisabledCount   int `json:"auto_disabled_count"`
		} `json:"data"`
	}
	require.NoError(t, platformencoding.Unmarshal(recorder.Body.Bytes(), &payload))
	require.True(t, payload.Success)
	require.Len(t, payload.Data.Keys, 3)
	require.Equal(t, 1, payload.Data.EnabledCount)
	require.Equal(t, 1, payload.Data.ManualDisabledCount)
	require.Equal(t, 1, payload.Data.AutoDisabledCount)
	require.Equal(t, "key-111111...", payload.Data.Keys[0].KeyPreview)
}

func TestManageMultiKeysDeleteDisabledKeys(t *testing.T) {
	createMultiKeyTestChannel(t)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(
		http.MethodPost,
		"/api/channel/multi_key/manage",
		strings.NewReader(`{"channel_id":1,"action":"delete_disabled_keys"}`),
	)
	ctx.Request.Header.Set("Content-Type", "application/json")

	ManageMultiKeys(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{"success":true,"message":"已删除 1 个自动禁用的密钥","data":1}`, recorder.Body.String())

	channel, err := gatewaystore.LoadChannelByID(1, true)
	require.NoError(t, err)
	require.Equal(t, "key-111111111\nkey-222222222", channel.Key)
	require.Equal(t, 2, channel.ChannelInfo.MultiKeySize)
	require.Equal(t, constant.ChannelStatusManuallyDisabled, channel.ChannelInfo.MultiKeyStatusList[1])
	_, exists := channel.ChannelInfo.MultiKeyStatusList[2]
	require.False(t, exists)
}

func TestManageMultiKeysRejectsDeletingLastKey(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	weight := uint(1)
	priority := int64(1)
	channel := gatewayschema.Channel{
		Id:       1,
		Type:     1,
		Key:      "solo-key",
		Status:   constant.ChannelStatusEnabled,
		Name:     "solo",
		Weight:   &weight,
		Models:   "gpt-4o",
		Group:    "default",
		Priority: &priority,
		ChannelInfo: gatewayschema.ChannelInfo{
			IsMultiKey:   true,
			MultiKeySize: 1,
			MultiKeyMode: constant.MultiKeyModeRandom,
		},
	}
	require.NoError(t, db.Create(&channel).Error)
	require.NoError(t, gatewaystore.AddChannelAbilities(&channel, nil))

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(
		http.MethodPost,
		"/api/channel/multi_key/manage",
		strings.NewReader(`{"channel_id":1,"action":"delete_key","key_index":0}`),
	)
	ctx.Request.Header.Set("Content-Type", "application/json")

	ManageMultiKeys(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{"success":false,"message":"不能删除最后一个密钥"}`, recorder.Body.String())
}
