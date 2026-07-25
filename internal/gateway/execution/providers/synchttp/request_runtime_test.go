package synchttp

import (
	"errors"
	"fmt"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

type timeoutError struct{}

func (timeoutError) Error() string   { return "timeout awaiting response headers" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return true }

func TestIsUpstreamResponseTimeout(t *testing.T) {
	require.True(t, isUpstreamResponseTimeout(timeoutError{}))
	require.True(t, isUpstreamResponseTimeout(fmt.Errorf("wrapped: %w", timeoutError{})))
	require.True(t, isUpstreamResponseTimeout(errors.New("net/http: timeout awaiting response headers")))
	require.False(t, isUpstreamResponseTimeout(&net.DNSError{IsTimeout: false}))
	require.False(t, isUpstreamResponseTimeout(errors.New("upstream returned bad gateway")))
}
