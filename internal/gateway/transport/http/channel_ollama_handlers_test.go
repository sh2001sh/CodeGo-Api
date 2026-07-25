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

func createNonOllamaChannel(t *testing.T) {
	t.Helper()
	db := setupModelListControllerTestDB(t)
	weight := uint(1)
	priority := int64(1)
	channel := gatewayschema.Channel{
		Id:       1,
		Type:     1,
		Key:      "secret",
		Status:   constant.ChannelStatusEnabled,
		Name:     "not-ollama",
		Weight:   &weight,
		Models:   "gpt-4o",
		Group:    "default",
		Priority: &priority,
	}
	require.NoError(t, db.Create(&channel).Error)
	require.NoError(t, gatewaystore.AddChannelAbilities(&channel, nil))
}

func TestPullOllamaModelRejectsNonOllamaChannel(t *testing.T) {
	createNonOllamaChannel(t)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(
		http.MethodPost,
		"/api/channel/ollama/pull",
		strings.NewReader(`{"channel_id":1,"model_name":"llama3"}`),
	)
	ctx.Request.Header.Set("Content-Type", "application/json")

	PullOllamaModel(ctx)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.JSONEq(t, `{"success":false,"message":"Failed to pull model: This operation is only supported for Ollama channels"}`, recorder.Body.String())
}

func TestDeleteOllamaModelValidatesRequest(t *testing.T) {
	setupModelListControllerTestDB(t)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(
		http.MethodDelete,
		"/api/channel/ollama/delete",
		strings.NewReader(`{"channel_id":0,"model_name":""}`),
	)
	ctx.Request.Header.Set("Content-Type", "application/json")

	DeleteOllamaModel(ctx)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.JSONEq(t, `{"success":false,"message":"Channel ID and model name are required"}`, recorder.Body.String())
}

func TestOllamaVersionRejectsNonOllamaChannel(t *testing.T) {
	createNonOllamaChannel(t)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = gin.Params{{Key: "id", Value: "1"}}
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/channel/ollama/version/1", nil)

	OllamaVersion(ctx)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.JSONEq(t, `{"success":false,"message":"This operation is only supported for Ollama channels"}`, recorder.Body.String())
}

func TestPullOllamaModelStreamValidatesRequest(t *testing.T) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(
		http.MethodPost,
		"/api/channel/ollama/pull/stream",
		strings.NewReader(`{"channel_id":0,"model_name":""}`),
	)
	ctx.Request.Header.Set("Content-Type", "application/json")

	PullOllamaModelStream(ctx)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.JSONEq(t, `{"success":false,"message":"Channel ID and model name are required"}`, recorder.Body.String())
}
