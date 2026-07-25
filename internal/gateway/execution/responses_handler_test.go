package execution

import (
	"testing"

	platformhttpx "github.com/sh2001sh/CodeGo-Api/internal/platform/httpx"
	"github.com/stretchr/testify/require"
)

func TestForceResponsesStreamBodyAddsStreamTrue(t *testing.T) {
	storage, err := platformhttpx.CreateBodyStorage([]byte(`{"model":"gpt-5","input":"hello"}`))
	require.NoError(t, err)
	defer storage.Close()

	body, err := forceResponsesStreamBody(storage)

	require.NoError(t, err)
	require.JSONEq(t, `{"model":"gpt-5","input":"hello","stream":true}`, string(body))
}

func TestForceResponsesStreamBodyOverridesStreamFalse(t *testing.T) {
	storage, err := platformhttpx.CreateBodyStorage([]byte(`{"model":"gpt-5","input":"hello","stream":false}`))
	require.NoError(t, err)
	defer storage.Close()

	body, err := forceResponsesStreamBody(storage)

	require.NoError(t, err)
	require.JSONEq(t, `{"model":"gpt-5","input":"hello","stream":true}`, string(body))
}
