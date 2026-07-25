package app

import (
	"errors"
	identitydomain "github.com/sh2001sh/CodeGo-Api/internal/identity/domain"
	platformruntime "github.com/sh2001sh/CodeGo-Api/internal/platform/runtime"
	"gorm.io/gorm"
	"os"
	"strings"
)

var (
	ErrImageWorkspaceItemNotFound = errors.New("image not found")
	ErrImageWorkspaceItemExpired  = errors.New("image has expired")
	ErrImageWorkspaceFileMissing  = errors.New("image file not found")
)

type ImageWorkspaceItemResponse struct {
	Id            int    `json:"id"`
	SessionId     string `json:"session_id"`
	BatchId       string `json:"batch_id"`
	SourceItemId  int    `json:"source_item_id"`
	Model         string `json:"model"`
	Prompt        string `json:"prompt"`
	RevisedPrompt string `json:"revised_prompt"`
	ImageIndex    int    `json:"image_index"`
	Status        string `json:"status"`
	ImageURL      string `json:"image_url"`
	DownloadURL   string `json:"download_url"`
	OriginalURL   string `json:"original_url"`
	ExpiresAt     int64  `json:"expires_at"`
	CreatedAt     int64  `json:"created_at"`
	ErrorMessage  string `json:"error_message"`
}

type ImageWorkspaceItemsPage struct {
	Items []ImageWorkspaceItemResponse `json:"items"`
	Total int64                        `json:"total"`
}

// ListImageWorkspaceModels returns image-generation models available to the
// current user through the requested concrete or automatic group.
func ListImageWorkspaceModels(userID int, group string) ([]string, error) {
	models, err := ListUserModelsForGroup(userID, group)
	if err != nil {
		return nil, err
	}
	models = FilterImageModels(models)
	return models, nil
}

// ListImageWorkspaceItems loads paginated image workspace items for the current user.
func ListImageWorkspaceItems(userID int, sessionID string, page int, pageSize int) (*ImageWorkspaceItemsPage, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	items, total, err := listImageWorkspaceItemsByUser(userID, strings.TrimSpace(sessionID), (page-1)*pageSize, pageSize)
	if err != nil {
		return nil, err
	}

	now := platformruntime.GetTimestamp()
	responseItems := make([]ImageWorkspaceItemResponse, 0, len(items))
	for _, item := range items {
		imageURL := ""
		downloadURL := ""
		if item.Status == identitydomain.ImageWorkspaceStatusReady && item.ExpiresAt > now && item.FilePath != "" {
			imageURL = BuildImageWorkspaceAssetURL(item.Id)
			downloadURL = BuildImageWorkspaceDownloadURL(item.Id)
		}
		responseItems = append(responseItems, ImageWorkspaceItemResponse{
			Id:            item.Id,
			SessionId:     item.SessionId,
			BatchId:       item.BatchId,
			SourceItemId:  item.SourceItemId,
			Model:         item.Model,
			Prompt:        item.Prompt,
			RevisedPrompt: item.RevisedPrompt,
			ImageIndex:    item.ImageIndex,
			Status:        item.Status,
			ImageURL:      imageURL,
			DownloadURL:   downloadURL,
			OriginalURL:   item.OriginalURL,
			ExpiresAt:     item.ExpiresAt,
			CreatedAt:     item.CreatedAt,
			ErrorMessage:  item.ErrorMessage,
		})
	}

	return &ImageWorkspaceItemsPage{
		Items: responseItems,
		Total: total,
	}, nil
}

// LoadImageWorkspaceItemContentSource validates and returns a user-owned image workspace item for streaming.
func LoadImageWorkspaceItemContentSource(userID int, itemID int) (*identitydomain.ImageWorkspaceItem, error) {
	item, err := getImageWorkspaceItemByUser(userID, itemID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrImageWorkspaceItemNotFound
		}
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrImageWorkspaceItemNotFound
		}
		return nil, err
	}

	if item.Status != identitydomain.ImageWorkspaceStatusReady || item.ExpiresAt <= platformruntime.GetTimestamp() || item.FilePath == "" {
		return nil, ErrImageWorkspaceItemExpired
	}
	if _, statErr := os.Stat(item.FilePath); statErr != nil {
		return nil, ErrImageWorkspaceFileMissing
	}

	return item, nil
}
