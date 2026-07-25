package app

import (
	"errors"
	identitydomain "github.com/sh2001sh/CodeGo-Api/internal/identity/domain"
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
	platformruntime "github.com/sh2001sh/CodeGo-Api/internal/platform/runtime"
	"os"
)

func createImageWorkspaceItems(items []*identitydomain.ImageWorkspaceItem) error {
	if len(items) == 0 {
		return nil
	}
	return platformdb.DB.Create(&items).Error
}

func listImageWorkspaceItemsByUser(userID int, sessionID string, offset int, limit int) ([]*identitydomain.ImageWorkspaceItem, int64, error) {
	var (
		items []*identitydomain.ImageWorkspaceItem
		total int64
	)

	tx := platformdb.DB.Model(&identitydomain.ImageWorkspaceItem{}).Where("user_id = ?", userID)
	if sessionID != "" {
		tx = tx.Where("session_id = ?", sessionID)
	}
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := tx.Order("created_at desc, id desc").Offset(offset).Limit(limit).Find(&items).Error
	return items, total, err
}

func getImageWorkspaceItemByUser(userID int, itemID int) (*identitydomain.ImageWorkspaceItem, error) {
	var item identitydomain.ImageWorkspaceItem
	if err := platformdb.DB.Where("id = ? AND user_id = ?", itemID, userID).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func cleanupExpiredImageWorkspaceItems(limit int, now int64) (int, error) {
	if limit <= 0 {
		limit = 100
	}

	var items []*identitydomain.ImageWorkspaceItem
	if err := platformdb.DB.Where("status = ? AND expires_at > 0 AND expires_at <= ?", identitydomain.ImageWorkspaceStatusReady, now).
		Order("expires_at asc, id asc").
		Limit(limit).
		Find(&items).Error; err != nil {
		return 0, err
	}
	if len(items) == 0 {
		return 0, nil
	}

	for _, item := range items {
		if item.FilePath != "" {
			_ = os.Remove(item.FilePath)
		}
	}

	ids := make([]int, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.Id)
	}

	return len(ids), platformdb.DB.Model(&identitydomain.ImageWorkspaceItem{}).
		Where("id IN ?", ids).
		Updates(map[string]any{
			"status":        identitydomain.ImageWorkspaceStatusExpired,
			"file_path":     "",
			"error_message": "expired",
			"updated_at":    platformruntime.GetTimestamp(),
		}).Error
}

func getImageWorkspaceSourceItem(userID int, sourceItemID int) (*identitydomain.ImageWorkspaceItem, error) {
	if sourceItemID <= 0 {
		return nil, errors.New("source item id is required")
	}
	return getImageWorkspaceItemByUser(userID, sourceItemID)
}
