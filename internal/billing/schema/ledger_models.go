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
	BillingAccountStatusActive = "active"

	BillingReservationStatusOpen     = "open"
	BillingReservationStatusSettled  = "settled"
	BillingReservationStatusReleased = "released"
	BillingReservationStatusExpired  = "expired"

	BillingSettlementStatusPending   = "pending"
	BillingSettlementStatusCompleted = "completed"
	BillingSettlementStatusRejected  = "rejected"

	BillingDirectionDebit  = "debit"
	BillingDirectionCredit = "credit"
)

type BillingAccount struct {
	AccountID   string          `json:"account_id" gorm:"column:account_id;primaryKey;size:64"`
	AccountType string          `json:"account_type" gorm:"column:account_type;size:32;index:idx_billing_accounts_type_status;index:idx_billing_accounts_owner_account_unit,unique"`
	OwnerType   string          `json:"owner_type" gorm:"column:owner_type;size:32;index:idx_billing_accounts_owner_account_unit,unique"`
	OwnerID     int64           `json:"owner_id" gorm:"column:owner_id;index:idx_billing_accounts_owner_account_unit,unique;index:idx_billing_accounts_owner"`
	QuotaUnit   string          `json:"quota_unit" gorm:"column:quota_unit;size:32;index:idx_billing_accounts_owner_account_unit,unique"`
	Status      string          `json:"status" gorm:"column:status;size:32;index:idx_billing_accounts_type_status"`
	Version     int64           `json:"version" gorm:"column:version"`
	MetaJSON    json.RawMessage `json:"meta_json" gorm:"column:meta_json;type:json"`
	CreatedAt   time.Time       `json:"created_at" gorm:"column:created_at;autoCreateTime"`
	UpdatedAt   time.Time       `json:"updated_at" gorm:"column:updated_at;autoUpdateTime"`
}

func (BillingAccount) TableName() string {
	if platformdb.UsingPostgreSQL {
		return "billing.accounts"
	}
	return "billing_accounts"
}

func (a *BillingAccount) BeforeCreate(_ *gorm.DB) error {
	if strings.TrimSpace(a.AccountID) == "" {
		a.AccountID = platformruntime.GetUUID()
	}
	if strings.TrimSpace(a.QuotaUnit) == "" {
		a.QuotaUnit = "quota"
	}
	if strings.TrimSpace(a.Status) == "" {
		a.Status = BillingAccountStatusActive
	}
	if len(a.MetaJSON) == 0 {
		a.MetaJSON = json.RawMessage(`{}`)
	}
	return nil
}

type BillingBalanceSnapshot struct {
	AccountID        string    `json:"account_id" gorm:"column:account_id;primaryKey;size:64"`
	AvailableBalance int64     `json:"available_balance" gorm:"column:available_balance"`
	ReservedBalance  int64     `json:"reserved_balance" gorm:"column:reserved_balance"`
	ConsumedTotal    int64     `json:"consumed_total" gorm:"column:consumed_total"`
	RefundedTotal    int64     `json:"refunded_total" gorm:"column:refunded_total"`
	GrantedTotal     int64     `json:"granted_total" gorm:"column:granted_total"`
	UpdatedAt        time.Time `json:"updated_at" gorm:"column:updated_at;autoCreateTime;autoUpdateTime"`
}

func (BillingBalanceSnapshot) TableName() string {
	if platformdb.UsingPostgreSQL {
		return "billing.balance_snapshots"
	}
	return "billing_balance_snapshots"
}

type BillingLedgerEntry struct {
	EntryID        string          `json:"entry_id" gorm:"column:entry_id;primaryKey;size:64"`
	AccountID      string          `json:"account_id" gorm:"column:account_id;size:64;index:idx_billing_ledger_entries_account_created;index"`
	ReferenceType  string          `json:"reference_type" gorm:"column:reference_type;size:32;index:idx_billing_ledger_entries_reference"`
	ReferenceID    string          `json:"reference_id" gorm:"column:reference_id;size:128;index:idx_billing_ledger_entries_reference"`
	EntryType      string          `json:"entry_type" gorm:"column:entry_type;size:32;index"`
	Direction      string          `json:"direction" gorm:"column:direction;size:16"`
	Amount         int64           `json:"amount" gorm:"column:amount"`
	BalanceAfter   *int64          `json:"balance_after" gorm:"column:balance_after"`
	IdempotencyKey string          `json:"idempotency_key" gorm:"column:idempotency_key;size:255;uniqueIndex:uq_billing_ledger_entries_idempotency"`
	ReasonCode     string          `json:"reason_code" gorm:"column:reason_code;size:64"`
	ReasonDetail   string          `json:"reason_detail" gorm:"column:reason_detail;type:text"`
	OperatorType   string          `json:"operator_type" gorm:"column:operator_type;size:32"`
	OperatorID     string          `json:"operator_id" gorm:"column:operator_id;size:128"`
	Metadata       json.RawMessage `json:"metadata" gorm:"column:metadata;type:json"`
	CreatedAt      time.Time       `json:"created_at" gorm:"column:created_at;autoCreateTime"`
}

func (BillingLedgerEntry) TableName() string {
	if platformdb.UsingPostgreSQL {
		return "billing.ledger_entries"
	}
	return "billing_ledger_entries"
}

func (e *BillingLedgerEntry) BeforeCreate(_ *gorm.DB) error {
	if strings.TrimSpace(e.EntryID) == "" {
		e.EntryID = platformruntime.GetUUID()
	}
	if len(e.Metadata) == 0 {
		e.Metadata = json.RawMessage(`{}`)
	}
	return nil
}

type BillingReservation struct {
	ReservationID  string     `json:"reservation_id" gorm:"column:reservation_id;primaryKey;size:64"`
	RequestID      string     `json:"request_id" gorm:"column:request_id;size:64;index"`
	WorkflowID     string     `json:"workflow_id" gorm:"column:workflow_id;size:64;index"`
	AccountID      string     `json:"account_id" gorm:"column:account_id;size:64;index;index:idx_billing_reservations_account_status"`
	ReservedAmount int64      `json:"reserved_amount" gorm:"column:reserved_amount"`
	Status         string     `json:"status" gorm:"column:status;size:32;index;index:idx_billing_reservations_account_status"`
	IdempotencyKey string     `json:"idempotency_key" gorm:"column:idempotency_key;size:255;uniqueIndex:uq_billing_reservations_idempotency"`
	ExpiresAt      *time.Time `json:"expires_at" gorm:"column:expires_at;index"`
	CreatedAt      time.Time  `json:"created_at" gorm:"column:created_at;autoCreateTime"`
	UpdatedAt      time.Time  `json:"updated_at" gorm:"column:updated_at;autoUpdateTime"`
}

func (BillingReservation) TableName() string {
	if platformdb.UsingPostgreSQL {
		return "billing.reservations"
	}
	return "billing_reservations"
}

func (r *BillingReservation) BeforeCreate(_ *gorm.DB) error {
	if strings.TrimSpace(r.ReservationID) == "" {
		r.ReservationID = platformruntime.GetUUID()
	}
	if strings.TrimSpace(r.Status) == "" {
		r.Status = BillingReservationStatusOpen
	}
	return nil
}

type BillingSettlement struct {
	SettlementID    string    `json:"settlement_id" gorm:"column:settlement_id;primaryKey;size:64"`
	ReservationID   string    `json:"reservation_id" gorm:"column:reservation_id;size:64;uniqueIndex:uq_billing_settlements_reservation;index"`
	UsageEvidenceID string    `json:"usage_evidence_id" gorm:"column:usage_evidence_id;size:64"`
	ActualAmount    int64     `json:"actual_amount" gorm:"column:actual_amount"`
	DeltaAmount     int64     `json:"delta_amount" gorm:"column:delta_amount"`
	Status          string    `json:"status" gorm:"column:status;size:32"`
	IdempotencyKey  string    `json:"idempotency_key" gorm:"column:idempotency_key;size:255;uniqueIndex:uq_billing_settlements_idempotency"`
	SettledAt       time.Time `json:"settled_at" gorm:"column:settled_at;autoCreateTime"`
	CreatedAt       time.Time `json:"created_at" gorm:"column:created_at;autoCreateTime"`
}

func (BillingSettlement) TableName() string {
	if platformdb.UsingPostgreSQL {
		return "billing.settlements"
	}
	return "billing_settlements"
}

func (s *BillingSettlement) BeforeCreate(_ *gorm.DB) error {
	if strings.TrimSpace(s.SettlementID) == "" {
		s.SettlementID = platformruntime.GetUUID()
	}
	if strings.TrimSpace(s.Status) == "" {
		s.Status = BillingSettlementStatusPending
	}
	return nil
}
