package taskx

import (
	"fmt"
	"net/http"
	"testing"

	platformtext "github.com/sh2001sh/CodeGo-Api/internal/platform/textx"
	"github.com/sh2001sh/CodeGo-Api/types"
	"github.com/stretchr/testify/require"
)

func TestTaskErrorWrapperSanitizesUpstreamQuotaLeak(t *testing.T) {
	t.Parallel()

	err := fmt.Errorf("status_code=403, 预扣费额度失败, 用户剩余额度: 0.000750, 需要预扣费额度: 0.002364 (request id: 202606170140313551875538268d9d6mB3fntBc)")

	taskErr := TaskErrorWrapper(err, "bad_response_status_code", http.StatusForbidden)

	require.Equal(t, platformtext.UpstreamQuotaGenericMessage, taskErr.Message)
}

func TestTaskErrorWrapperSanitizesUpstreamBalanceLeak(t *testing.T) {
	t.Parallel()

	err := fmt.Errorf("status_code=403, insufficient balance, remaining balance: 0.000750, required balance: 0.002364 (request id: abc)")

	taskErr := TaskErrorWrapper(err, "bad_response_status_code", http.StatusForbidden)

	require.Equal(t, platformtext.UpstreamQuotaGenericMessage, taskErr.Message)
}

func TestTaskErrorWrapperSanitizesChineseUpstreamBalanceLeak(t *testing.T) {
	t.Parallel()

	taskErr := TaskErrorWrapper(
		fmt.Errorf("用户额度不足, 剩余额度: -0.038392 (request id: abc)"),
		"bad_response_status_code",
		http.StatusForbidden,
	)

	require.Equal(t, platformtext.UpstreamQuotaGenericMessage, taskErr.Message)
}

func TestTaskErrorFromAPIErrorKeepsLocalQuotaMessage(t *testing.T) {
	t.Parallel()

	apiErr := types.NewErrorWithStatusCode(
		fmt.Errorf("用户额度不足, 剩余额度: 0.000750"),
		types.ErrorCodeInsufficientUserQuota,
		http.StatusForbidden,
	)

	taskErr := TaskErrorFromAPIError(apiErr)

	require.NotNil(t, taskErr)
	require.Equal(t, "用户额度不足, 剩余额度: 0.000750", taskErr.Message)
	require.True(t, taskErr.LocalError)
}

func TestTaskErrorFromAPIErrorKeepsLocalQuotaMessageWithRequestID(t *testing.T) {
	t.Parallel()

	apiErr := types.NewErrorWithStatusCode(
		fmt.Errorf("站内余额不足, 当前余额: 0.000750, 本次所需: 0.002364 (request id: abc)"),
		types.ErrorCodeInsufficientUserQuota,
		http.StatusForbidden,
	)

	taskErr := TaskErrorFromAPIError(apiErr)

	require.NotNil(t, taskErr)
	require.Equal(t, "站内余额不足, 当前余额: 0.000750, 本次所需: 0.002364 (request id: abc)", taskErr.Message)
}
