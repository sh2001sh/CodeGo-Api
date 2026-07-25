package billingschema

import (
	"encoding/json"
	"strings"
	"time"

	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
	platformruntime "github.com/sh2001sh/CodeGo-Api/internal/platform/runtime"
	"gorm.io/gorm"
)

const (
	BillingOutboxStatusPending   = "pending"
	BillingOutboxStatusPublished = "published"
)

// BillingOutboxEvent records a ledger state change for asynchronous processing.
type BillingOutboxEvent struct {
	EventID        string          `json:"event_id" gorm:"column:event_id;primaryKey;size:64"`
	AccountID      string          `json:"account_id" gorm:"column:account_id;size:64;index:idx_billing_outbox_status_created"`
	AggregateType  string          `json:"aggregate_type" gorm:"column:aggregate_type;size:32"`
	AggregateID    string          `json:"aggregate_id" gorm:"column:aggregate_id;size:64"`
	EventType      string          `json:"event_type" gorm:"column:event_type;size:64"`
	Payload        json.RawMessage `json:"payload" gorm:"column:payload;type:json"`
	IdempotencyKey string          `json:"idempotency_key" gorm:"column:idempotency_key;size:255;uniqueIndex:uq_billing_outbox_idempotency"`
	Status         string          `json:"status" gorm:"column:status;size:16;index:idx_billing_outbox_status_created"`
	Attempts       int             `json:"attempts" gorm:"column:attempts"`
	LastError      string          `json:"last_error" gorm:"column:last_error;type:text"`
	PublishedAt    *time.Time      `json:"published_at" gorm:"column:published_at;index"`
	CreatedAt      time.Time       `json:"created_at" gorm:"column:created_at;autoCreateTime"`
	UpdatedAt      time.Time       `json:"updated_at" gorm:"column:updated_at;autoUpdateTime"`
}

func (BillingOutboxEvent) TableName() string {
	if platformdb.UsingPostgreSQL {
		return "billing.outbox_events"
	}
	return "billing_outbox_events"
}

func (event *BillingOutboxEvent) BeforeCreate(_ *gorm.DB) error {
	if strings.TrimSpace(event.EventID) == "" {
		event.EventID = platformruntime.GetUUID()
	}
	if strings.TrimSpace(event.Status) == "" {
		event.Status = BillingOutboxStatusPending
	}
	if len(event.Payload) == 0 {
		event.Payload = json.RawMessage(`{}`)
	}
	return nil
}
