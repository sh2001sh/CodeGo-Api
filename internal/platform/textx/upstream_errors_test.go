package textx

import "testing"

func TestSanitizeUpstreamQuotaErrorMessage(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "sanitize upstream 403 quota leak",
			input:    "status_code=403, 预扣费额度失败, 用户剩余额度: 0.000750, 需要预扣费额度: 0.002364 (request id: 202606170140313551875538268d9d6mB3fntBc)",
			expected: UpstreamQuotaGenericMessage,
		},
		{
			name:     "sanitize upstream insufficient balance leak",
			input:    "status_code=403, insufficient balance, remaining balance: 0.000750, required balance: 0.002364 (request id: abc)",
			expected: UpstreamQuotaGenericMessage,
		},
		{
			name:     "sanitize plain upstream insufficient balance leak",
			input:    "insufficient balance",
			expected: UpstreamQuotaGenericMessage,
		},
		{
			name:     "sanitize upstream Chinese quota leak",
			input:    "用户额度不足, 剩余额度: -0.038392 (request id: abc)",
			expected: UpstreamQuotaGenericMessage,
		},
		{
			name:     "sanitize upstream no available channel",
			input:    "No available channel for model gpt-5.6-luna under group plus (distributor) (request id: abc)",
			expected: UpstreamQuotaGenericMessage,
		},
		{
			name:     "sanitize upstream model unavailable",
			input:    "model gpt-5.6-luna is temporarily unavailable",
			expected: UpstreamQuotaGenericMessage,
		},
		{
			name:     "keep local site balance message",
			input:    "站内余额不足, 当前余额: 0.000750, 本次所需: 0.002364 (request id: abc)",
			expected: "站内余额不足, 当前余额: 0.000750, 本次所需: 0.002364 (request id: abc)",
		},
		{
			name:     "keep ordinary error",
			input:    "请求参数错误",
			expected: "请求参数错误",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := SanitizeUpstreamQuotaErrorMessage(tc.input); got != tc.expected {
				t.Fatalf("expected %q, got %q", tc.expected, got)
			}
		})
	}
}

func TestIsUpstreamQuotaLeakMessage(t *testing.T) {
	t.Parallel()

	if !IsUpstreamQuotaLeakMessage("status_code=403, 预扣费额度失败, 用户剩余额度: 0.000750, 需要预扣费额度: 0.002364 (request id: abc)") {
		t.Fatal("expected upstream quota leak message to be detected")
	}
	if !IsUpstreamQuotaLeakMessage("status_code=403, insufficient balance, remaining balance: 0.000750, required balance: 0.002364 (request id: abc)") {
		t.Fatal("expected upstream balance leak message to be detected")
	}
	if !IsUpstreamQuotaLeakMessage("insufficient balance") {
		t.Fatal("expected plain upstream balance message to be detected")
	}
	if !IsUpstreamQuotaLeakMessage("用户额度不足, 剩余额度: -0.038392 (request id: abc)") {
		t.Fatal("expected Chinese upstream balance message to be detected")
	}
	if IsUpstreamQuotaLeakMessage("用户额度不足, 剩余额度: 0.000750") {
		t.Fatal("expected local user quota message not to be treated as upstream leak")
	}
	if IsUpstreamQuotaLeakMessage("insufficient_user_quota") {
		t.Fatal("expected local error code not to be treated as upstream leak")
	}
	if IsUpstreamQuotaLeakMessage("站内余额不足, 当前余额: 0.000750, 本次所需: 0.002364 (request id: abc)") {
		t.Fatal("expected local site balance message not to be treated as upstream leak")
	}
}

func TestIsUpstreamProviderUnavailableMessage(t *testing.T) {
	t.Parallel()

	if !IsUpstreamProviderUnavailableMessage("No available channel for model gpt-5.6-luna under group plus") {
		t.Fatal("expected no available channel to be detected")
	}
	if !IsUpstreamProviderUnavailableMessage("model gpt-5.6-luna is not supported") {
		t.Fatal("expected unavailable model to be detected")
	}
	if IsUpstreamProviderUnavailableMessage("站内余额不足, 当前余额: 0.000750") {
		t.Fatal("expected local site balance message not to be detected")
	}
}

func TestSanitizeUpstreamQuotaErrorMessageKeeps403QuotaLeakMessage(t *testing.T) {
	t.Parallel()

	input := "status_code=403, 预扣费额度失败, 用户剩余额度: 0.000750, 需要预扣费额度: 0.002364 (request id: abc)"
	if got := SanitizeUpstreamQuotaErrorMessage(input); got != UpstreamQuotaGenericMessage {
		t.Fatalf("expected sanitized upstream quota message, got %q", got)
	}
}
