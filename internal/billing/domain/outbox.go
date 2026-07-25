package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	billingschema "github.com/sh2001sh/CodeGo-Api/internal/billing/schema"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type OutboxEventInput struct {
	AccountID      string
	AggregateType  string
	AggregateID    string
	EventType      string
	IdempotencyKey string
	Payload        any
}

// RecordOutboxEvent appends an event to the current ledger transaction.
func RecordOutboxEvent(tx *gorm.DB, input OutboxEventInput) error {
	if tx == nil {
		return fmt.Errorf("transaction is required")
	}
	if strings.TrimSpace(input.AccountID) == "" || strings.TrimSpace(input.AggregateType) == "" ||
		strings.TrimSpace(input.AggregateID) == "" || strings.TrimSpace(input.EventType) == "" ||
		strings.TrimSpace(input.IdempotencyKey) == "" {
		return fmt.Errorf("account_id, aggregate_type, aggregate_id, event_type and idempotency_key are required")
	}

	payload, err := json.Marshal(input.Payload)
	if err != nil {
		return fmt.Errorf("marshal billing outbox payload: %w", err)
	}
	event := billingschema.BillingOutboxEvent{
		AccountID:      strings.TrimSpace(input.AccountID),
		AggregateType:  strings.TrimSpace(input.AggregateType),
		AggregateID:    strings.TrimSpace(input.AggregateID),
		EventType:      strings.TrimSpace(input.EventType),
		Payload:        payload,
		IdempotencyKey: strings.TrimSpace(input.IdempotencyKey),
	}
	var existing billingschema.BillingOutboxEvent
	err = tx.Where("idempotency_key = ?", event.IdempotencyKey).First(&existing).Error
	if err == nil {
		return validateExistingOutboxEvent(existing, event)
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	result := tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "idempotency_key"}},
		DoNothing: true,
	}).Create(&event)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected > 0 {
		return nil
	}

	if err := tx.Where("idempotency_key = ?", event.IdempotencyKey).First(&existing).Error; err != nil {
		return err
	}
	return validateExistingOutboxEvent(existing, event)
}

func validateExistingOutboxEvent(existing billingschema.BillingOutboxEvent, event billingschema.BillingOutboxEvent) error {
	if existing.EventType != event.EventType || existing.AggregateID != event.AggregateID || existing.AccountID != event.AccountID {
		return ErrLedgerConflict
	}
	return nil
}
