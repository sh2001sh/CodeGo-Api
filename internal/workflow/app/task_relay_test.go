package app

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/sh2001sh/CodeGo-Api/dto"
	"github.com/sh2001sh/CodeGo-Api/types"
	"github.com/stretchr/testify/require"
)

func TestRespondTaskErrorSanitizesUpstreamQuota403(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	taskErr := &dto.TaskError{
		Code:       "bad_response_status_code",
		Message:    "status_code=403, 预扣费额度失败, 用户剩余额度: 0.000750, 需要预扣费额度: 0.002364 (request id: abc)",
		StatusCode: http.StatusForbidden,
	}

	respondTaskError(c, taskErr)

	require.Equal(t, http.StatusForbidden, recorder.Code)
	require.Equal(t, http.StatusForbidden, taskErr.StatusCode)
	require.Equal(t, "当前模型服务暂不可用，请稍后重试", taskErr.Message)
}

func TestRespondTaskErrorKeepsLocalQuota403(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	taskErr := &dto.TaskError{
		Code:       "insufficient_user_quota",
		Message:    "用户额度不足, 剩余额度: 0.000750",
		StatusCode: http.StatusForbidden,
	}

	respondTaskError(c, taskErr)

	require.Equal(t, http.StatusForbidden, recorder.Code)
	require.Equal(t, http.StatusForbidden, taskErr.StatusCode)
}

func TestRespondTaskErrorHidesLocalChannelSelectionDetails(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	taskErr := &dto.TaskError{
		Code:       string(types.ErrorCodeGetChannelFailed),
		Message:    "分组 plus高不稳定分组 下模型 gpt-5.6-luna 的可用渠道不存在（retry）",
		StatusCode: http.StatusInternalServerError,
		LocalError: true,
	}

	respondTaskError(c, taskErr)

	require.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	require.Equal(t, types.ModelUnavailableMessage, taskErr.Message)
	require.NotContains(t, taskErr.Message, "plus高不稳定分组")
}
