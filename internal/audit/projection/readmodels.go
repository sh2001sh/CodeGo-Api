package projection

import (
	"context"
	"fmt"
	"sync"
	"time"

	auditschema "github.com/sh2001sh/CodeGo-Api/internal/audit/schema"
	billingschema "github.com/sh2001sh/CodeGo-Api/internal/billing/schema"
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
	platformobservability "github.com/sh2001sh/CodeGo-Api/internal/platform/observability"
	"gorm.io/gorm"
)

type UserUsageDaily struct {
	Day              string `gorm:"column:day;primaryKey;size:10"`
	UserID           int    `gorm:"column:user_id;primaryKey;index"`
	RequestCount     int64  `gorm:"column:request_count"`
	PromptTokens     int64  `gorm:"column:prompt_tokens"`
	CompletionTokens int64  `gorm:"column:completion_tokens"`
	Quota            int64  `gorm:"column:quota"`
	UpdatedAt        time.Time
}

func (UserUsageDaily) TableName() string {
	if platformdb.UsingPostgreSQL {
		return "readmodel.user_usage_daily"
	}
	return "readmodel_user_usage_daily"
}

type ChannelUsageDaily struct {
	Day              string `gorm:"column:day;primaryKey;size:10"`
	ChannelID        int    `gorm:"column:channel_id;primaryKey;index"`
	RequestCount     int64  `gorm:"column:request_count"`
	PromptTokens     int64  `gorm:"column:prompt_tokens"`
	CompletionTokens int64  `gorm:"column:completion_tokens"`
	Quota            int64  `gorm:"column:quota"`
	UpdatedAt        time.Time
}

func (ChannelUsageDaily) TableName() string {
	if platformdb.UsingPostgreSQL {
		return "readmodel.channel_usage_daily"
	}
	return "readmodel_channel_usage_daily"
}

type BillingAccountView struct {
	AccountID        string `gorm:"column:account_id;primaryKey;size:64"`
	AccountType      string `gorm:"column:account_type;size:32;index"`
	OwnerType        string `gorm:"column:owner_type;size:32;index"`
	OwnerID          int64  `gorm:"column:owner_id;index"`
	QuotaUnit        string `gorm:"column:quota_unit;size:32"`
	AvailableBalance int64  `gorm:"column:available_balance"`
	ReservedBalance  int64  `gorm:"column:reserved_balance"`
	ConsumedTotal    int64  `gorm:"column:consumed_total"`
	RefundedTotal    int64  `gorm:"column:refunded_total"`
	GrantedTotal     int64  `gorm:"column:granted_total"`
	UpdatedAt        time.Time
}

type readModelSchemaMigration struct {
	ID        string `gorm:"primaryKey;size:128"`
	AppliedAt time.Time
}

func (readModelSchemaMigration) TableName() string {
	if platformdb.UsingPostgreSQL {
		return "platform.schema_migrations"
	}
	return "platform_schema_migrations"
}

func (BillingAccountView) TableName() string {
	if platformdb.UsingPostgreSQL {
		return "readmodel.billing_account_views"
	}
	return "readmodel_billing_account_views"
}

var readModelWorkerOnce sync.Once

func StartReadModelWorker(ctx context.Context) {
	readModelWorkerOnce.Do(func() {
		go func() {
			for {
				if err := RebuildReadModels(ctx); err != nil {
					platformobservability.SysError("read model rebuild failed: " + err.Error())
				}
				select {
				case <-ctx.Done():
					return
				case <-time.After(time.Hour):
				}
			}
		}()
	})
}

func RebuildReadModels(ctx context.Context) error {
	if platformdb.DB == nil {
		return fmt.Errorf("primary database is not initialized")
	}
	if err := EnsureReadModelSchema(); err != nil {
		return err
	}
	if err := rebuildBillingAccountViews(ctx); err != nil {
		return err
	}
	if platformdb.LogDB == nil {
		return nil
	}
	return rebuildUsageDaily(ctx)
}

// EnsureReadModelSchema creates the dedicated reporting tables.
func EnsureReadModelSchema() error {
	if platformdb.DB == nil {
		return fmt.Errorf("primary database is not initialized")
	}
	return platformdb.DB.AutoMigrate(&UserUsageDaily{}, &ChannelUsageDaily{}, &BillingAccountView{})
}

// ApplyReadModelMigrations creates reporting tables and records the schema version.
func ApplyReadModelMigrations(ctx context.Context) error {
	if platformdb.DB == nil {
		return fmt.Errorf("primary database is not initialized")
	}
	db := platformdb.DB.WithContext(ctx)
	if err := db.AutoMigrate(&readModelSchemaMigration{}); err != nil {
		return err
	}
	const migrationID = "20260710_read_models"
	var applied readModelSchemaMigration
	err := db.Where("id = ?", migrationID).First(&applied).Error
	if err == nil {
		return nil
	}
	if err != gorm.ErrRecordNotFound {
		return err
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.AutoMigrate(&UserUsageDaily{}, &ChannelUsageDaily{}, &BillingAccountView{}); err != nil {
			return err
		}
		return tx.Create(&readModelSchemaMigration{ID: migrationID}).Error
	})
}

func rebuildBillingAccountViews(ctx context.Context) error {
	var accounts []billingschema.BillingAccount
	if err := platformdb.DB.WithContext(ctx).Find(&accounts).Error; err != nil {
		return err
	}
	views := make([]BillingAccountView, 0, len(accounts))
	for _, account := range accounts {
		var snapshot billingschema.BillingBalanceSnapshot
		if err := platformdb.DB.WithContext(ctx).Where("account_id = ?", account.AccountID).First(&snapshot).Error; err != nil && err != gorm.ErrRecordNotFound {
			return err
		}
		views = append(views, BillingAccountView{AccountID: account.AccountID, AccountType: account.AccountType, OwnerType: account.OwnerType, OwnerID: account.OwnerID, QuotaUnit: account.QuotaUnit, AvailableBalance: snapshot.AvailableBalance, ReservedBalance: snapshot.ReservedBalance, ConsumedTotal: snapshot.ConsumedTotal, RefundedTotal: snapshot.RefundedTotal, GrantedTotal: snapshot.GrantedTotal})
	}
	return platformdb.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("1 = 1").Delete(&BillingAccountView{}).Error; err != nil {
			return err
		}
		if len(views) == 0 {
			return nil
		}
		return tx.CreateInBatches(views, 500).Error
	})
}

func rebuildUsageDaily(ctx context.Context) error {
	day := "date(created_at, 'unixepoch')"
	if platformdb.UsingPostgreSQL {
		day = "TO_CHAR(TO_TIMESTAMP(created_at), 'YYYY-MM-DD')"
	}
	userRows := []UserUsageDaily{}
	channelRows := []ChannelUsageDaily{}
	if err := platformdb.LogDB.WithContext(ctx).Model(&auditschema.Log{}).Where("type = ?", auditschema.LogTypeConsume).
		Select(day + " AS day, user_id, COUNT(*) AS request_count, COALESCE(SUM(prompt_tokens), 0) AS prompt_tokens, COALESCE(SUM(completion_tokens), 0) AS completion_tokens, COALESCE(SUM(quota), 0) AS quota").
		Group(day + ", user_id").Scan(&userRows).Error; err != nil {
		return err
	}
	if err := platformdb.LogDB.WithContext(ctx).Model(&auditschema.Log{}).Where("type = ?", auditschema.LogTypeConsume).
		Select(day + " AS day, channel_id, COUNT(*) AS request_count, COALESCE(SUM(prompt_tokens), 0) AS prompt_tokens, COALESCE(SUM(completion_tokens), 0) AS completion_tokens, COALESCE(SUM(quota), 0) AS quota").
		Group(day + ", channel_id").Scan(&channelRows).Error; err != nil {
		return err
	}
	return platformdb.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("1 = 1").Delete(&UserUsageDaily{}).Error; err != nil {
			return err
		}
		if err := tx.Where("1 = 1").Delete(&ChannelUsageDaily{}).Error; err != nil {
			return err
		}
		if len(userRows) > 0 {
			if err := tx.CreateInBatches(userRows, 500).Error; err != nil {
				return err
			}
		}
		if len(channelRows) > 0 {
			if err := tx.CreateInBatches(channelRows, 500).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
