package app

import (
	"testing"

	platformhttpx "github.com/sh2001sh/CodeGo-Api/internal/platform/httpx"
	"github.com/stretchr/testify/require"
)

func TestWorkflowHTTPClientUsesProtectedPlatformClient(t *testing.T) {
	platformhttpx.InitHTTPClient()

	require.Same(t, platformhttpx.GetHTTPClient(), GetHttpClient())
	client, err := GetHttpClientWithProxy("")
	require.NoError(t, err)
	require.Same(t, platformhttpx.GetHTTPClient(), client)
}
