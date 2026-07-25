package types

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsRemoteProviderError(t *testing.T) {
	require.False(t, IsRemoteProviderError(nil))
	require.True(t, IsRemoteProviderError(NewOpenAIError(errors.New("upstream"), ErrorCodeBadResponseStatusCode, http.StatusServiceUnavailable)))
	require.True(t, IsRemoteProviderError(WithClaudeError(ClaudeError{Message: "upstream"}, http.StatusServiceUnavailable)))
	require.False(t, IsRemoteProviderError(NewError(errors.New("local"), ErrorCodeInvalidRequest)))
}
