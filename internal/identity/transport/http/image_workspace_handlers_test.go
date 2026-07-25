package http

import (
	"github.com/gin-gonic/gin"
	"github.com/sh2001sh/CodeGo-Api/constant"
	gatewayschema "github.com/sh2001sh/CodeGo-Api/internal/gateway/schema"
	identitydomain "github.com/sh2001sh/CodeGo-Api/internal/identity/domain"
	identityschema "github.com/sh2001sh/CodeGo-Api/internal/identity/schema"
	platformencoding "github.com/sh2001sh/CodeGo-Api/internal/platform/encodingx"
	platformruntime "github.com/sh2001sh/CodeGo-Api/internal/platform/runtime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestGetImageWorkspaceModelsReturnsImageGenerationModels(t *testing.T) {
	db := setupIdentityHTTPTestDB(t)

	user := &identityschema.User{Id: 1, Username: "image-model-user", Password: "password123", DisplayName: "Image Model User", Role: constant.RoleCommonUser, Status: constant.UserStatusEnabled, Group: "default"}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}
	abilities := []gatewayschema.Ability{
		{Group: "default", Model: "gpt-image-1", Enabled: true},
		{Group: "default", Model: "gpt-5.5", Enabled: true},
	}
	for _, ability := range abilities {
		record := ability
		if err := db.Create(&record).Error; err != nil {
			t.Fatalf("failed to seed ability: %v", err)
		}
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/user/image-workspace/models", nil, 1)
	GetImageWorkspaceModels(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}
	var models []string
	if err := platformencoding.Unmarshal(response.Data, &models); err != nil {
		t.Fatalf("failed to decode image workspace models: %v", err)
	}
	if len(models) != 1 || models[0] != "gpt-image-1" {
		t.Fatalf("expected only image models, got %#v", models)
	}
}

func TestGetImageWorkspaceItemsReturnsWorkspaceLinksForReadyAssets(t *testing.T) {
	db := setupIdentityHTTPTestDB(t)

	user := &identityschema.User{Id: 1, Username: "image-items-user", Password: "password123", DisplayName: "Image Items User", Role: constant.RoleCommonUser, Status: constant.UserStatusEnabled, Group: "default"}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}

	tempDir := t.TempDir()
	readyPath := filepath.Join(tempDir, "ready.png")
	if err := os.WriteFile(readyPath, []byte("pngdata"), 0o600); err != nil {
		t.Fatalf("failed to write image fixture: %v", err)
	}

	now := platformruntime.GetTimestamp()
	items := []*identitydomain.ImageWorkspaceItem{
		{
			UserId:    1,
			SessionId: "sess-1",
			BatchId:   "batch-1",
			Model:     "gpt-image-1",
			Prompt:    "draw cat",
			Status:    identitydomain.ImageWorkspaceStatusReady,
			FilePath:  readyPath,
			ExpiresAt: now + 3600,
			CreatedAt: now,
		},
		{
			UserId:    1,
			SessionId: "sess-1",
			BatchId:   "batch-1",
			Model:     "gpt-image-1",
			Prompt:    "draw dog",
			Status:    identitydomain.ImageWorkspaceStatusFailed,
			ExpiresAt: now + 3600,
			CreatedAt: now - 1,
		},
	}
	if err := db.Create(&items).Error; err != nil {
		t.Fatalf("failed to seed image workspace items: %v", err)
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/user/image-workspace/items?p=1&page_size=20", nil, 1)
	GetImageWorkspaceItems(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}
	var payload []identityImageWorkspaceItem
	if err := platformencoding.Unmarshal(response.Data, &payload); err != nil {
		t.Fatalf("failed to decode image workspace items: %v", err)
	}
	if len(payload) != 2 {
		t.Fatalf("expected 2 items, got %d", len(payload))
	}
	if payload[0].ImageURL == "" || payload[0].DownloadURL == "" {
		t.Fatalf("expected ready item URLs, got %#v", payload[0])
	}
	if payload[1].ImageURL != "" || payload[1].DownloadURL != "" {
		t.Fatalf("expected failed item URLs to be empty, got %#v", payload[1])
	}
}

func TestGetImageWorkspaceItemContentStreamsFile(t *testing.T) {
	db := setupIdentityHTTPTestDB(t)

	user := &identityschema.User{Id: 1, Username: "image-content-user", Password: "password123", DisplayName: "Image Content User", Role: constant.RoleCommonUser, Status: constant.UserStatusEnabled, Group: "default"}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}

	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "image.png")
	expectedBody := []byte("image-binary-data")
	if err := os.WriteFile(filePath, expectedBody, 0o600); err != nil {
		t.Fatalf("failed to write image fixture: %v", err)
	}

	item := &identitydomain.ImageWorkspaceItem{
		UserId:    1,
		SessionId: "sess-content",
		BatchId:   "batch-content",
		Model:     "gpt-image-1",
		Prompt:    "draw sunset",
		Status:    identitydomain.ImageWorkspaceStatusReady,
		MimeType:  "image/png",
		FilePath:  filePath,
		ExpiresAt: platformruntime.GetTimestamp() + 3600,
	}
	if err := db.Create(item).Error; err != nil {
		t.Fatalf("failed to seed image workspace item: %v", err)
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/user/image-workspace/items/"+strconv.Itoa(item.Id)+"/content", nil, 1)
	ctx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(item.Id)}}
	GetImageWorkspaceItemContent(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200 response, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	if recorder.Body.String() != string(expectedBody) {
		t.Fatalf("expected streamed body %q, got %q", string(expectedBody), recorder.Body.String())
	}
	if contentType := recorder.Header().Get("Content-Type"); contentType != "image/png" {
		t.Fatalf("expected image/png content type, got %q", contentType)
	}
}

type identityImageWorkspaceItem struct {
	Id          int    `json:"id"`
	ImageURL    string `json:"image_url"`
	DownloadURL string `json:"download_url"`
	Status      string `json:"status"`
}
