package httpx

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	platformconfig "github.com/sh2001sh/CodeGo-Api/internal/platform/config"
	"github.com/stretchr/testify/require"
)

func TestOutboundClientsApplyResponseHeaderTimeoutWithoutGlobalRequestTimeout(t *testing.T) {
	previousHeaderTimeout := platformconfig.RelayResponseHeaderTimeout
	previousRelayTimeout := platformconfig.RelayTimeout
	t.Cleanup(func() {
		platformconfig.RelayResponseHeaderTimeout = previousHeaderTimeout
		platformconfig.RelayTimeout = previousRelayTimeout
		ResetProxyClientCache()
		InitHTTPClient()
	})

	platformconfig.RelayResponseHeaderTimeout = 45
	platformconfig.RelayTimeout = 0
	ResetProxyClientCache()
	InitHTTPClient()

	assertClientResponseHeaderTimeout(t, GetHTTPClient())

	httpProxyClient, err := NewProxyHTTPClient("http://127.0.0.1:8080")
	require.NoError(t, err)
	assertClientResponseHeaderTimeout(t, httpProxyClient)

	socksProxyClient, err := NewProxyHTTPClient("socks5://127.0.0.1:1080")
	require.NoError(t, err)
	assertClientResponseHeaderTimeout(t, socksProxyClient)
}

func TestResponseHeaderTimeoutStopsStalledUpstreamBeforeStreamDelivery(t *testing.T) {
	previousHeaderTimeout := platformconfig.RelayResponseHeaderTimeout
	previousRelayTimeout := platformconfig.RelayTimeout
	t.Cleanup(func() {
		platformconfig.RelayResponseHeaderTimeout = previousHeaderTimeout
		platformconfig.RelayTimeout = previousRelayTimeout
		InitHTTPClient()
	})

	platformconfig.RelayResponseHeaderTimeout = 1
	platformconfig.RelayTimeout = 0
	InitHTTPClient()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		select {
		case <-request.Context().Done():
		case <-time.After(2 * time.Second):
		}
	}))
	defer server.Close()

	startedAt := time.Now()
	_, err := GetHTTPClient().Get(server.URL)
	require.Error(t, err)
	require.Less(t, time.Since(startedAt), 1500*time.Millisecond)
}

func assertClientResponseHeaderTimeout(t *testing.T, client *http.Client) {
	t.Helper()
	require.NotNil(t, client)
	require.Equal(t, time.Duration(0), client.Timeout)
	transport, ok := client.Transport.(*http.Transport)
	require.True(t, ok)
	require.Equal(t, 45*time.Second, transport.ResponseHeaderTimeout)
	require.Equal(t, 45*time.Second, transport.TLSHandshakeTimeout)
}
