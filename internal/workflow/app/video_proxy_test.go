package app

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestEnsureAPIKeyAppendsOnce(t *testing.T) {
	require.Equal(t, "https://example.com/video?key=abc", ensureAPIKey("https://example.com/video", "abc"))
	require.Equal(t, "https://example.com/video?foo=bar&key=abc", ensureAPIKey("https://example.com/video?foo=bar", "abc"))
	require.Equal(t, "https://example.com/video?key=existing", ensureAPIKey("https://example.com/video?key=existing", "abc"))
}

func TestWriteVideoDataURLWritesDecodedVideo(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	payload := base64.StdEncoding.EncodeToString([]byte("video-bytes"))

	err := writeVideoDataURL(ctx, "data:video/mp4;base64,"+payload)

	require.NoError(t, err)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "video/mp4", recorder.Header().Get("Content-Type"))
	require.Equal(t, "video-bytes", recorder.Body.String())
}
