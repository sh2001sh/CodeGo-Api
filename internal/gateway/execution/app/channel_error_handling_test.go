package app

import (
	"errors"
	"net/http"
	"testing"

	"github.com/sh2001sh/CodeGo-Api/types"
	"github.com/stretchr/testify/require"
)

func TestIsModelUnavailableError(t *testing.T) {
	require.True(t, IsModelUnavailableError(types.NewOpenAIError(
		errors.New("The model does not exist"), types.ErrorCodeModelNotFound, http.StatusNotFound,
	)))
	require.True(t, IsModelUnavailableError(types.NewOpenAIError(
		errors.New("model not supported"), types.ErrorCodeBadResponseStatusCode, http.StatusBadRequest,
	)))
	require.False(t, IsModelUnavailableError(types.NewOpenAIError(
		errors.New("invalid API key"), types.ErrorCodeBadResponseStatusCode, http.StatusUnauthorized,
	)))
	require.False(t, IsModelUnavailableError(types.NewOpenAIError(
		errors.New("resource not found"), types.ErrorCodeBadResponseStatusCode, http.StatusNotFound,
	)))
}

func TestIsModelScopedUpstreamFailure(t *testing.T) {
	require.True(t, IsModelScopedUpstreamFailure(types.NewOpenAIError(
		errors.New("insufficient_user_quota"), types.ErrorCodeBadResponseStatusCode, http.StatusServiceUnavailable,
	)))
	require.True(t, IsModelScopedUpstreamFailure(types.NewOpenAIError(
		errors.New("upstream balance exhausted"), types.ErrorCodeBadResponseStatusCode, http.StatusServiceUnavailable,
	)))
	require.True(t, IsModelScopedUpstreamFailure(types.NewOpenAIError(
		errors.New("model unavailable"), types.ErrorCodeModelNotFound, http.StatusNotFound,
	)))
	require.True(t, IsModelScopedUpstreamFailure(types.NewOpenAIError(
		errors.New("upstream timeout"), types.ErrorCodeBadResponseStatusCode, http.StatusServiceUnavailable,
	)))
	require.True(t, IsModelScopedUpstreamFailure(types.NewOpenAIError(
		errors.New("insufficient_user_quota"), types.ErrorCodeBadResponseStatusCode, http.StatusForbidden,
	)))
	require.False(t, IsModelScopedUpstreamFailure(types.NewOpenAIError(
		errors.New("access denied"), types.ErrorCodeBadResponseStatusCode, http.StatusForbidden,
	)))
}

func TestClassifyUpstreamFailure(t *testing.T) {
	testCases := []struct {
		name     string
		err      *types.APIError
		expected upstreamFailureClass
	}{
		{
			name:     "upstream account exhaustion",
			err:      types.NewOpenAIError(errors.New("upstream balance exhausted"), types.ErrorCodeBadResponseStatusCode, http.StatusForbidden),
			expected: upstreamFailureAccountExhausted,
		},
		{
			name:     "response header timeout",
			err:      types.NewErrorWithStatusCode(errors.New("timeout awaiting response headers"), types.ErrorCodeChannelResponseTimeExceeded, http.StatusGatewayTimeout),
			expected: upstreamFailureTransient,
		},
		{
			name:     "closed upstream stream",
			err:      types.NewOpenAIError(errors.New("responses stream closed before response.completed"), types.ErrorCodeBadResponseStatusCode, http.StatusInternalServerError),
			expected: upstreamFailureTransient,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			require.Equal(t, testCase.expected, classifyUpstreamFailure(testCase.err))
		})
	}
}
