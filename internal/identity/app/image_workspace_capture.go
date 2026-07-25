package app

import (
	"encoding/base64"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/sh2001sh/CodeGo-Api/dto"
	identitydomain "github.com/sh2001sh/CodeGo-Api/internal/identity/domain"
	platformencoding "github.com/sh2001sh/CodeGo-Api/internal/platform/encodingx"
	"github.com/sh2001sh/CodeGo-Api/internal/platform/filex"
	platformhttpx "github.com/sh2001sh/CodeGo-Api/internal/platform/httpx"
	platformruntime "github.com/sh2001sh/CodeGo-Api/internal/platform/runtime"
	platformtext "github.com/sh2001sh/CodeGo-Api/internal/platform/textx"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type ImageWorkspaceRequestMeta struct {
	UserID       int
	SessionID    string
	BatchID      string
	SourceItemID int
	Model        string
	Prompt       string
	Size         string
	Quality      string
	Count        int
}

type PersistedImageWorkspaceItem struct {
	Record *identitydomain.ImageWorkspaceItem
	URL    string
}

// BuildImageWorkspaceMetaFromRequest parses one image workspace relay request into persisted metadata.
func BuildImageWorkspaceMetaFromRequest(c *gin.Context) (*ImageWorkspaceRequestMeta, error) {
	if c == nil {
		return nil, fmt.Errorf("context is nil")
	}
	meta := &ImageWorkspaceRequestMeta{
		UserID:       c.GetInt("id"),
		SessionID:    strings.TrimSpace(c.Query("session_id")),
		BatchID:      strings.TrimSpace(c.Query("batch_id")),
		SourceItemID: platformtext.String2Int(c.Query("source_item_id")),
	}
	if meta.SessionID == "" {
		meta.SessionID = fmt.Sprintf("imgs_%d_%s", platformruntime.GetTimestamp(), platformruntime.GetRandomString(8))
	}
	if meta.BatchID == "" {
		meta.BatchID = fmt.Sprintf("imgb_%d_%s", platformruntime.GetTimestamp(), platformruntime.GetRandomString(8))
	}

	if strings.Contains(c.ContentType(), "multipart/form-data") {
		if _, err := c.MultipartForm(); err != nil {
			return nil, err
		}
		meta.Model = c.PostForm("model")
		meta.Prompt = c.PostForm("prompt")
		meta.Size = c.PostForm("size")
		meta.Quality = c.PostForm("quality")
		meta.Count = maxImageWorkspaceInt(platformtext.String2Int(c.PostForm("n")), 1)
		return meta, nil
	}

	var imageRequest dto.ImageRequest
	if err := platformhttpx.UnmarshalBodyReusable(c, &imageRequest); err != nil {
		return nil, err
	}
	meta.Model = imageRequest.Model
	meta.Prompt = imageRequest.Prompt
	meta.Size = imageRequest.Size
	meta.Quality = imageRequest.Quality
	meta.Count = 1
	if imageRequest.N != nil && *imageRequest.N > 0 {
		meta.Count = int(*imageRequest.N)
	}
	return meta, nil
}

// PersistImageWorkspaceResponse stores relay-generated images into the workspace domain.
func PersistImageWorkspaceResponse(c *gin.Context, meta *ImageWorkspaceRequestMeta, responseBody []byte) ([]*PersistedImageWorkspaceItem, error) {
	if meta == nil || len(responseBody) == 0 {
		return nil, nil
	}

	var response dto.ImageResponse
	if err := platformencoding.Unmarshal(responseBody, &response); err != nil {
		return nil, err
	}
	if len(response.Data) == 0 {
		return nil, nil
	}

	rootDir := GetImageWorkspaceDir()
	if err := EnsureImageWorkspaceDir(); err != nil {
		return nil, err
	}

	now := platformruntime.GetTimestamp()
	expiresAt := now + GetImageWorkspaceRetentionSeconds()
	paramsMap := map[string]any{
		"size":    meta.Size,
		"quality": meta.Quality,
		"count":   meta.Count,
	}
	paramsJSON := platformtext.MapToJsonStr(paramsMap)

	records := make([]*identitydomain.ImageWorkspaceItem, 0, len(response.Data))
	for idx, imageData := range response.Data {
		filePath, mimeType, originalURL, err := persistImageWorkspaceFile(rootDir, meta.BatchID, idx, imageData)
		if err != nil {
			records = append(records, &identitydomain.ImageWorkspaceItem{
				UserId:        meta.UserID,
				SessionId:     meta.SessionID,
				BatchId:       meta.BatchID,
				SourceItemId:  meta.SourceItemID,
				Model:         meta.Model,
				Prompt:        meta.Prompt,
				RevisedPrompt: imageData.RevisedPrompt,
				RequestParams: paramsJSON,
				ImageIndex:    idx,
				Status:        identitydomain.ImageWorkspaceStatusFailed,
				OriginalURL:   originalURL,
				ErrorMessage:  err.Error(),
				ExpiresAt:     expiresAt,
			})
			continue
		}
		records = append(records, &identitydomain.ImageWorkspaceItem{
			UserId:        meta.UserID,
			SessionId:     meta.SessionID,
			BatchId:       meta.BatchID,
			SourceItemId:  meta.SourceItemID,
			Model:         meta.Model,
			Prompt:        meta.Prompt,
			RevisedPrompt: imageData.RevisedPrompt,
			RequestParams: paramsJSON,
			ImageIndex:    idx,
			Status:        identitydomain.ImageWorkspaceStatusReady,
			MimeType:      mimeType,
			FilePath:      filePath,
			OriginalURL:   originalURL,
			ExpiresAt:     expiresAt,
		})
	}

	if err := createImageWorkspaceItems(records); err != nil {
		return nil, err
	}

	items := make([]*PersistedImageWorkspaceItem, 0, len(records))
	for _, record := range records {
		items = append(items, &PersistedImageWorkspaceItem{
			Record: record,
			URL:    BuildImageWorkspaceAssetURL(record.Id),
		})
	}
	return items, nil
}

func persistImageWorkspaceFile(rootDir string, batchID string, imageIndex int, imageData dto.ImageData) (string, string, string, error) {
	if strings.TrimSpace(imageData.Url) != "" {
		return persistImageWorkspaceURL(rootDir, batchID, imageIndex, imageData.Url)
	}
	if strings.TrimSpace(imageData.B64Json) != "" {
		return persistImageWorkspaceBase64(rootDir, batchID, imageIndex, imageData.B64Json)
	}
	return "", "", "", fmt.Errorf("empty image data")
}

func persistImageWorkspaceURL(rootDir string, batchID string, imageIndex int, sourceURL string) (string, string, string, error) {
	mimeType, base64Data, err := filex.GetImageFromURL(sourceURL)
	if err != nil {
		return "", "", sourceURL, err
	}
	raw, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return "", "", sourceURL, err
	}
	filePath, saveErr := saveImageWorkspaceBytes(rootDir, batchID, imageIndex, mimeType, raw)
	return filePath, mimeType, sourceURL, saveErr
}

func persistImageWorkspaceBase64(rootDir string, batchID string, imageIndex int, encoded string) (string, string, string, error) {
	mimeType, base64Data, err := filex.DecodeBase64FileData(encoded)
	if err != nil {
		return "", "", "", err
	}
	raw, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		raw, err = base64.RawStdEncoding.DecodeString(base64Data)
		if err != nil {
			return "", "", "", err
		}
	}
	if err != nil {
		return "", "", "", err
	}
	filePath, saveErr := saveImageWorkspaceBytes(rootDir, batchID, imageIndex, mimeType, raw)
	return filePath, mimeType, "", saveErr
}

func saveImageWorkspaceBytes(rootDir string, batchID string, imageIndex int, mimeType string, raw []byte) (string, error) {
	ext := imageWorkspaceExtensionFromMimeType(mimeType)
	dayDir := time.Now().Format("20060102")
	dir := filepath.Join(rootDir, dayDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	fileName := fmt.Sprintf("%s_%d%s", batchID, imageIndex, ext)
	filePath := filepath.Join(dir, fileName)
	if err := os.WriteFile(filePath, raw, 0644); err != nil {
		return "", err
	}
	return filePath, nil
}

func imageWorkspaceExtensionFromMimeType(mimeType string) string {
	switch strings.ToLower(strings.TrimSpace(mimeType)) {
	case "image/jpg":
		return ".jpg"
	case "image/jpeg":
		return ".jpeg"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	default:
		return ".png"
	}
}

func maxImageWorkspaceInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
}
