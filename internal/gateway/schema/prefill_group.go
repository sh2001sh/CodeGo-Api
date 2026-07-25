package schema

import (
	"database/sql/driver"
	"encoding/json"

	platformencoding "github.com/sh2001sh/CodeGo-Api/internal/platform/encodingx"
	"gorm.io/gorm"
)

// JSONValue is a raw JSON value that supports database driver byte and string results.
type JSONValue json.RawMessage

func (j JSONValue) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return []byte(j), nil
}

func (j *JSONValue) Scan(value any) error {
	switch v := value.(type) {
	case nil:
		*j = nil
	case []byte:
		*j = append((*j)[:0], v...)
	case string:
		*j = append((*j)[:0], v...)
	default:
		bytes, err := platformencoding.Marshal(v)
		if err != nil {
			return err
		}
		*j = JSONValue(bytes)
	}
	return nil
}

func (j JSONValue) MarshalJSON() ([]byte, error) {
	if j == nil {
		return []byte("null"), nil
	}
	return j, nil
}

func (j *JSONValue) UnmarshalJSON(data []byte) error {
	*j = append((*j)[:0], data...)
	return nil
}

// PrefillGroup stores reusable model, tag, and endpoint groups for gateway configuration.
type PrefillGroup struct {
	Id          int            `json:"id"`
	Name        string         `json:"name" gorm:"size:64;not null;uniqueIndex:uk_prefill_name,where:deleted_at IS NULL"`
	Type        string         `json:"type" gorm:"size:32;index;not null"`
	Items       JSONValue      `json:"items" gorm:"type:json"`
	Description string         `json:"description,omitempty" gorm:"type:varchar(255)"`
	CreatedTime int64          `json:"created_time" gorm:"bigint"`
	UpdatedTime int64          `json:"updated_time" gorm:"bigint"`
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`
}
