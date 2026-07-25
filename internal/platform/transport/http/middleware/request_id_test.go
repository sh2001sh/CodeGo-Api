package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/sh2001sh/CodeGo-Api/constant"
	"github.com/stretchr/testify/require"
)

func TestRequestIDCreatesAndPropagatesTraceID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestId())
	router.GET("/", func(c *gin.Context) {
		require.NotEmpty(t, c.GetString(constant.RequestIdKey))
		require.Equal(t, c.GetString(constant.TraceIdKey), c.Request.Context().Value(constant.TraceIdKey))
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set(constant.TraceIdKey, "trace-from-client")
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusNoContent, recorder.Code)
	require.NotEmpty(t, recorder.Header().Get(constant.RequestIdKey))
	require.Equal(t, "trace-from-client", recorder.Header().Get(constant.TraceIdKey))
}
