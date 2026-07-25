package app

import (
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sh2001sh/CodeGo-Api/constant"
	identitydomain "github.com/sh2001sh/CodeGo-Api/internal/identity/domain"
	identitystore "github.com/sh2001sh/CodeGo-Api/internal/identity/store"
)

func BuildImageWorkspaceAssetURL(itemID int) string {
	return fmt.Sprintf("/api/user/image-workspace/items/%d/content", itemID)
}

func BuildImageWorkspaceDownloadURL(itemID int) string {
	return fmt.Sprintf("/api/user/image-workspace/items/%d/content?download=1", itemID)
}

func FilterImageModels(models []string) []string {
	if len(models) == 0 {
		return nil
	}
	filtered := make([]string, 0, len(models))
	for _, modelName := range models {
		if modelName == "" {
			continue
		}
		if supportsImageGeneration(modelName) {
			filtered = append(filtered, modelName)
		}
	}
	return filtered
}

func supportsImageGeneration(modelName string) bool {
	for _, endpointType := range identitystore.LoadModelSupportedEndpointTypes(modelName) {
		if endpointType == constant.EndpointTypeImageGeneration {
			return true
		}
	}
	name := strings.ToLower(strings.TrimSpace(modelName))
	return strings.Contains(name, "image") ||
		strings.Contains(name, "dall-e") ||
		strings.Contains(name, "imagen") ||
		strings.Contains(name, "flux")
}

func WriteImageWorkspaceAsset(c *gin.Context, item *identitydomain.ImageWorkspaceItem, download bool) error {
	if item == nil {
		return fmt.Errorf("item is nil")
	}
	body, err := os.ReadFile(item.FilePath)
	if err != nil {
		return err
	}
	contentType := detectImageWorkspaceMimeType(item)
	c.Header("Content-Type", contentType)
	c.Header("Cache-Control", "public, max-age=3600")
	if download {
		ext := filepath.Ext(item.FilePath)
		if ext == "" {
			ext = extensionFromMimeType(contentType)
		}
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"image-%d%s\"", item.Id, ext))
	}
	c.Writer.WriteHeader(http.StatusOK)
	_, err = c.Writer.Write(body)
	return err
}

func detectImageWorkspaceMimeType(item *identitydomain.ImageWorkspaceItem) string {
	if item == nil {
		return "image/png"
	}
	if item.MimeType != "" {
		return item.MimeType
	}
	switch strings.ToLower(filepath.Ext(item.FilePath)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	default:
		return "image/png"
	}
}

func extensionFromMimeType(mimeType string) string {
	if mimeType == "" {
		return ".png"
	}
	if exts, err := mime.ExtensionsByType(mimeType); err == nil && len(exts) > 0 {
		return exts[0]
	}
	switch strings.ToLower(strings.TrimSpace(mimeType)) {
	case "image/jpg":
		return ".jpg"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	default:
		return ".png"
	}
}
