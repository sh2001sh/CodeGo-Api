package domain

import (
	platformruntime "github.com/sh2001sh/CodeGo-Api/internal/platform/runtime"
	"gorm.io/gorm"
)

const (
	ImageWorkspaceStatusReady   = "ready"
	ImageWorkspaceStatusExpired = "expired"
	ImageWorkspaceStatusFailed  = "failed"
)

// ImageWorkspaceItem is a persisted image-generation asset owned by a user.
type ImageWorkspaceItem struct {
	Id            int            `json:"id" gorm:"primaryKey"`
	UserId        int            `json:"user_id" gorm:"index;not null"`
	SessionId     string         `json:"session_id" gorm:"type:varchar(64);index;not null"`
	BatchId       string         `json:"batch_id" gorm:"type:varchar(64);index;not null"`
	SourceItemId  int            `json:"source_item_id" gorm:"index;default:0"`
	Model         string         `json:"model" gorm:"type:varchar(128);not null"`
	Prompt        string         `json:"prompt" gorm:"type:text"`
	RevisedPrompt string         `json:"revised_prompt" gorm:"type:text"`
	RequestParams string         `json:"request_params" gorm:"type:text"`
	ImageIndex    int            `json:"image_index" gorm:"default:0"`
	Status        string         `json:"status" gorm:"type:varchar(32);index;not null"`
	MimeType      string         `json:"mime_type" gorm:"type:varchar(128);default:''"`
	FilePath      string         `json:"file_path" gorm:"type:text"`
	OriginalURL   string         `json:"original_url" gorm:"type:text"`
	ErrorMessage  string         `json:"error_message" gorm:"type:text"`
	ExpiresAt     int64          `json:"expires_at" gorm:"bigint;index"`
	CreatedAt     int64          `json:"created_at" gorm:"bigint;index"`
	UpdatedAt     int64          `json:"updated_at" gorm:"bigint"`
	DeletedAt     gorm.DeletedAt `json:"-" gorm:"index"`
}

func (item *ImageWorkspaceItem) BeforeCreate(_ *gorm.DB) error {
	now := platformruntime.GetTimestamp()
	if item.CreatedAt == 0 {
		item.CreatedAt = now
	}
	if item.UpdatedAt == 0 {
		item.UpdatedAt = now
	}
	return nil
}

func (item *ImageWorkspaceItem) BeforeUpdate(_ *gorm.DB) error {
	item.UpdatedAt = platformruntime.GetTimestamp()
	return nil
}
