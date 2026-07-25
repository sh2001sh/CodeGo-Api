package defaultweb

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	platformhttp "github.com/sh2001sh/CodeGo-Api/internal/platform/transport/http"
)

func TestThemeAssetsServeStaticFilesAndReactPublicRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	platformhttp.RegisterControlRuntimeRoutes(engine, ThemeAssets(DefaultIndexPage()))

	tests := []struct {
		path       string
		wantStatus int
		wantType   string
		wantBody   string
		wantTarget string
	}{
		{path: "/code-go-logo.svg", wantStatus: http.StatusOK, wantType: "image/svg+xml"},
		{path: "/pricing", wantStatus: http.StatusOK, wantType: "text/html", wantBody: "id=\"root\""},
		{path: "/pricing/", wantStatus: http.StatusOK, wantType: "text/html", wantBody: "id=\"root\""},
	}

	for _, test := range tests {
		req := httptest.NewRequest(http.MethodGet, test.path, nil)
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)

		if rec.Code != test.wantStatus {
			t.Fatalf("%s status = %d, want %d", test.path, rec.Code, test.wantStatus)
		}
		if test.wantType != "" && !strings.HasPrefix(rec.Header().Get("Content-Type"), test.wantType) {
			t.Fatalf("%s content type = %q, want prefix %q", test.path, rec.Header().Get("Content-Type"), test.wantType)
		}
		if test.wantBody != "" && !strings.Contains(rec.Body.String(), test.wantBody) {
			t.Fatalf("%s body is missing %q", test.path, test.wantBody)
		}
		if target := rec.Header().Get("Location"); target != test.wantTarget {
			t.Fatalf("%s location = %q, want %q", test.path, target, test.wantTarget)
		}
	}
}
