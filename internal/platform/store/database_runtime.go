package store

import (
	"fmt"
	"github.com/glebarez/sqlite"
	auditdomain "github.com/sh2001sh/CodeGo-Api/internal/audit/domain"
	auditschema "github.com/sh2001sh/CodeGo-Api/internal/audit/schema"
	billingschema "github.com/sh2001sh/CodeGo-Api/internal/billing/schema"
	commerceschema "github.com/sh2001sh/CodeGo-Api/internal/commerce/schema"
	gatewayschema "github.com/sh2001sh/CodeGo-Api/internal/gateway/schema"
	identitydomain "github.com/sh2001sh/CodeGo-Api/internal/identity/domain"
	identityschema "github.com/sh2001sh/CodeGo-Api/internal/identity/schema"
	platformconfig "github.com/sh2001sh/CodeGo-Api/internal/platform/config"
	platformdb "github.com/sh2001sh/CodeGo-Api/internal/platform/db"
	platformobservability "github.com/sh2001sh/CodeGo-Api/internal/platform/observability"
	platformschema "github.com/sh2001sh/CodeGo-Api/internal/platform/schema"
	workflowschema "github.com/sh2001sh/CodeGo-Api/internal/workflow/schema"

	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"os"
	"strings"
	"time"
)

// InitPrimaryDB initializes the primary application database.
func InitPrimaryDB() error {
	db, err := openDatabase("SQL_DSN", false)
	if err != nil {
		platformobservability.FatalLog(err.Error())
		return err
	}
	if platformconfig.DebugEnabled {
		db = db.Debug()
	}
	platformdb.DB = db
	if platformdb.UsingMySQL {
		if err := checkMySQLChineseSupport(platformdb.DB); err != nil {
			return err
		}
	}

	sqlDB, err := platformdb.DB.DB()
	if err != nil {
		return err
	}
	sqlDB.SetMaxIdleConns(platformconfig.GetEnvOrDefaultInt("SQL_MAX_IDLE_CONNS", 100))
	sqlDB.SetMaxOpenConns(platformconfig.GetEnvOrDefaultInt("SQL_MAX_OPEN_CONNS", 1000))
	sqlDB.SetConnMaxLifetime(time.Second * time.Duration(platformconfig.GetEnvOrDefaultInt("SQL_MAX_LIFETIME", 60)))

	if !platformconfig.IsMasterNode {
		return nil
	}
	if !platformconfig.GetEnvOrDefaultBool("AUTO_DB_MIGRATION", false) {
		platformobservability.SysLog("database migration deferred; run db-migrate before starting services")
		return nil
	}

	platformobservability.SysLog("database migration started")
	return migratePrimaryDB()
}

// InitLogDB initializes the optional log database.
func InitLogDB() error {
	if os.Getenv("LOG_SQL_DSN") == "" {
		platformdb.LogDB = platformdb.DB
		return nil
	}

	db, err := openDatabase("LOG_SQL_DSN", true)
	if err != nil {
		platformobservability.FatalLog(err.Error())
		return err
	}
	if platformconfig.DebugEnabled {
		db = db.Debug()
	}
	platformdb.LogDB = db
	if platformdb.LogSQLType == platformdb.DatabaseTypeMySQL {
		if err := checkMySQLChineseSupport(platformdb.LogDB); err != nil {
			return err
		}
	}

	sqlDB, err := platformdb.LogDB.DB()
	if err != nil {
		return err
	}
	sqlDB.SetMaxIdleConns(platformconfig.GetEnvOrDefaultInt("SQL_MAX_IDLE_CONNS", 100))
	sqlDB.SetMaxOpenConns(platformconfig.GetEnvOrDefaultInt("SQL_MAX_OPEN_CONNS", 1000))
	sqlDB.SetConnMaxLifetime(time.Second * time.Duration(platformconfig.GetEnvOrDefaultInt("SQL_MAX_LIFETIME", 60)))

	if !platformconfig.IsMasterNode {
		return nil
	}
	if !platformconfig.GetEnvOrDefaultBool("AUTO_DB_MIGRATION", false) {
		platformobservability.SysLog("log database migration deferred; run db-migrate before starting services")
		return nil
	}

	platformobservability.SysLog("database migration started")
	return migrateLogDB()
}

// CloseDatabases closes the primary and log database handles.
func CloseDatabases() error {
	if platformdb.LogDB != nil && platformdb.LogDB != platformdb.DB {
		if err := closeDatabaseHandle(platformdb.LogDB); err != nil {
			return err
		}
	}
	return closeDatabaseHandle(platformdb.DB)
}

func openDatabase(envName string, isLog bool) (*gorm.DB, error) {
	dsn := os.Getenv(envName)
	if dsn != "" {
		if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
			platformobservability.SysLog("using PostgreSQL as database")
			if !isLog {
				platformdb.UsingPostgreSQL = true
				platformdb.UsingMySQL = false
				platformdb.UsingSQLite = false
			} else {
				platformdb.LogSQLType = platformdb.DatabaseTypePostgreSQL
			}
			return gorm.Open(postgres.New(postgres.Config{
				DSN:                  dsn,
				PreferSimpleProtocol: true,
			}), &gorm.Config{PrepareStmt: true})
		}

		if strings.HasPrefix(dsn, "local") {
			platformobservability.SysLog("SQL_DSN not set, using SQLite as database")
			if !isLog {
				platformdb.UsingSQLite = true
				platformdb.UsingMySQL = false
				platformdb.UsingPostgreSQL = false
			} else {
				platformdb.LogSQLType = platformdb.DatabaseTypeSQLite
			}
			return gorm.Open(sqlite.Open(platformdb.SQLitePath), &gorm.Config{PrepareStmt: true})
		}

		platformobservability.SysLog("using MySQL as database")
		if !strings.Contains(dsn, "parseTime") {
			if strings.Contains(dsn, "?") {
				dsn += "&parseTime=true"
			} else {
				dsn += "?parseTime=true"
			}
		}
		if !isLog {
			platformdb.UsingMySQL = true
			platformdb.UsingSQLite = false
			platformdb.UsingPostgreSQL = false
		} else {
			platformdb.LogSQLType = platformdb.DatabaseTypeMySQL
		}
		return gorm.Open(mysql.Open(dsn), &gorm.Config{PrepareStmt: true})
	}

	platformobservability.SysLog("SQL_DSN not set, using SQLite as database")
	if !isLog {
		platformdb.UsingSQLite = true
		platformdb.UsingMySQL = false
		platformdb.UsingPostgreSQL = false
	} else {
		platformdb.LogSQLType = platformdb.DatabaseTypeSQLite
	}
	return gorm.Open(sqlite.Open(platformdb.SQLitePath), &gorm.Config{PrepareStmt: true})
}

func migratePrimaryDB() error {
	migrateSubscriptionPlanPriceAmount()
	if err := migrateTokenModelLimitsToText(); err != nil {
		return err
	}
	if err := ensureCodeGoSchemas(); err != nil {
		return err
	}

	err := platformdb.DB.AutoMigrate(
		&gatewayschema.Channel{},
		&identityschema.Token{},
		&identityschema.User{},
		&identitydomain.PasskeyCredential{},
		&platformschema.Option{},
		&commerceschema.Redemption{},
		&gatewayschema.Ability{},
		&auditschema.Log{},
		&commerceschema.TopUp{},
		&auditdomain.QuotaData{},
		&workflowschema.Task{},
		&billingschema.BillingAccount{},
		&billingschema.BillingBalanceSnapshot{},
		&billingschema.BillingLedgerEntry{},
		&billingschema.BillingReservation{},
		&billingschema.BillingSettlement{},
		&billingschema.BillingOutboxEvent{},
		&workflowschema.WorkflowTaskWorkflow{},
		&workflowschema.WorkflowTaskSnapshot{},
		&workflowschema.WorkflowTaskTerminalResult{},
		&gatewayschema.Model{},
		&gatewayschema.Vendor{},
		&gatewayschema.PrefillGroup{},
		&platformschema.Setup{},
		&identitydomain.TwoFA{},
		&identitydomain.TwoFABackupCode{},
		&commerceschema.SubscriptionOrder{},
		&commerceschema.UserSubscription{},
		&commerceschema.SubscriptionPreConsumeRecord{},
		&identitydomain.CustomOAuthProvider{},
		&identitydomain.UserOAuthBinding{},
		&identitydomain.ImageWorkspaceItem{},
	)
	if err != nil {
		return err
	}

	if platformdb.UsingSQLite {
		if err := ensureSubscriptionPlanTableSQLite(); err != nil {
			return err
		}
		if err := ensureSubscriptionOrderTableSQLite(); err != nil {
			return err
		}
		return nil
	}

	return platformdb.DB.AutoMigrate(&commerceschema.SubscriptionPlan{})
}

// BootstrapPrimarySchema creates the legacy application schema required by a
// brand-new database before v2 migrations and historical backfill can run.
// Existing production databases should use the versioned db-migrate path only.
func BootstrapPrimarySchema() error {
	if platformdb.DB == nil {
		return fmt.Errorf("primary database is not initialized")
	}
	return migratePrimaryDB()
}

func migrateLogDB() error {
	return platformdb.LogDB.AutoMigrate(&auditschema.Log{})
}

func ensureCodeGoSchemas() error {
	if !platformdb.UsingPostgreSQL {
		return nil
	}
	schemas := []string{"billing", "gateway", "workflow", "readmodel", "audit", "platform"}
	for _, schema := range schemas {
		if err := platformdb.DB.Exec("CREATE SCHEMA IF NOT EXISTS " + schema).Error; err != nil {
			return err
		}
	}
	return nil
}

type sqliteColumnDef struct {
	Name string
	DDL  string
}

func ensureSubscriptionPlanTableSQLite() error {
	if !platformdb.UsingSQLite {
		return nil
	}
	return ensureSubscriptionPlanTableSQLiteTx(platformdb.DB)
}

func ensureSubscriptionPlanTableSQLiteTx(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("database is not initialized")
	}
	tableName := "subscription_plans"
	if !db.Migrator().HasTable(tableName) {
		createSQL := "CREATE TABLE `" + tableName + "` (\n" +
			"`id` integer,\n" +
			"`title` varchar(128) NOT NULL,\n" +
			"`subtitle` varchar(255) DEFAULT '',\n" +
			"`price_amount` decimal(10,6) NOT NULL,\n" +
			"`currency` varchar(8) NOT NULL DEFAULT 'USD',\n" +
			"`duration_unit` varchar(16) NOT NULL DEFAULT 'month',\n" +
			"`duration_value` integer NOT NULL DEFAULT 1,\n" +
			"`custom_seconds` bigint NOT NULL DEFAULT 0,\n" +
			"`enabled` numeric DEFAULT 1,\n" +
			"`internal_only` numeric DEFAULT 0,\n" +
			"`sort_order` integer DEFAULT 0,\n" +
			"`stripe_price_id` varchar(128) DEFAULT '',\n" +
			"`creem_product_id` varchar(128) DEFAULT '',\n" +
			"`max_purchase_per_user` integer DEFAULT 0,\n" +
			"`plan_type` varchar(20) DEFAULT 'monthly',\n" +
			"`fuel_enabled` numeric DEFAULT 0,\n" +
			"`fuel_unit_price` decimal(10,4) DEFAULT 0,\n" +
			"`fuel_min_quota` bigint DEFAULT 0,\n" +
			"`fuel_quota_step` bigint DEFAULT 0,\n" +
			"`upgrade_group` varchar(64) DEFAULT '',\n" +
			"`total_amount` bigint NOT NULL DEFAULT 0,\n" +
			"`period_amount` bigint NOT NULL DEFAULT 0,\n" +
			"`model_limits` text,\n" +
			"`quota_reset_period` varchar(16) DEFAULT 'never',\n" +
			"`quota_reset_custom_seconds` bigint DEFAULT 0,\n" +
			"`created_at` bigint,\n" +
			"`updated_at` bigint,\n" +
			"PRIMARY KEY (`id`)\n" +
			")"
		return db.Exec(createSQL).Error
	}

	var cols []struct {
		Name string `gorm:"column:name"`
	}
	if err := db.Raw("PRAGMA table_info(`" + tableName + "`)").Scan(&cols).Error; err != nil {
		return err
	}
	existing := make(map[string]struct{}, len(cols))
	for _, c := range cols {
		existing[c.Name] = struct{}{}
	}
	required := []sqliteColumnDef{
		{Name: "title", DDL: "`title` varchar(128) NOT NULL"},
		{Name: "subtitle", DDL: "`subtitle` varchar(255) DEFAULT ''"},
		{Name: "price_amount", DDL: "`price_amount` decimal(10,6) NOT NULL"},
		{Name: "currency", DDL: "`currency` varchar(8) NOT NULL DEFAULT 'USD'"},
		{Name: "duration_unit", DDL: "`duration_unit` varchar(16) NOT NULL DEFAULT 'month'"},
		{Name: "duration_value", DDL: "`duration_value` integer NOT NULL DEFAULT 1"},
		{Name: "custom_seconds", DDL: "`custom_seconds` bigint NOT NULL DEFAULT 0"},
		{Name: "enabled", DDL: "`enabled` numeric DEFAULT 1"},
		{Name: "internal_only", DDL: "`internal_only` numeric DEFAULT 0"},
		{Name: "sort_order", DDL: "`sort_order` integer DEFAULT 0"},
		{Name: "stripe_price_id", DDL: "`stripe_price_id` varchar(128) DEFAULT ''"},
		{Name: "creem_product_id", DDL: "`creem_product_id` varchar(128) DEFAULT ''"},
		{Name: "max_purchase_per_user", DDL: "`max_purchase_per_user` integer DEFAULT 0"},
		{Name: "plan_type", DDL: "`plan_type` varchar(20) DEFAULT 'monthly'"},
		{Name: "fuel_enabled", DDL: "`fuel_enabled` numeric DEFAULT 0"},
		{Name: "fuel_unit_price", DDL: "`fuel_unit_price` decimal(10,4) DEFAULT 0"},
		{Name: "fuel_min_quota", DDL: "`fuel_min_quota` bigint DEFAULT 0"},
		{Name: "fuel_quota_step", DDL: "`fuel_quota_step` bigint DEFAULT 0"},
		{Name: "upgrade_group", DDL: "`upgrade_group` varchar(64) DEFAULT ''"},
		{Name: "total_amount", DDL: "`total_amount` bigint NOT NULL DEFAULT 0"},
		{Name: "period_amount", DDL: "`period_amount` bigint NOT NULL DEFAULT 0"},
		{Name: "model_limits", DDL: "`model_limits` text"},
		{Name: "quota_reset_period", DDL: "`quota_reset_period` varchar(16) DEFAULT 'never'"},
		{Name: "quota_reset_custom_seconds", DDL: "`quota_reset_custom_seconds` bigint DEFAULT 0"},
		{Name: "created_at", DDL: "`created_at` bigint"},
		{Name: "updated_at", DDL: "`updated_at` bigint"},
	}
	for _, col := range required {
		if _, ok := existing[col.Name]; ok {
			continue
		}
		if err := db.Exec("ALTER TABLE `" + tableName + "` ADD COLUMN " + col.DDL).Error; err != nil {
			return err
		}
	}
	return nil
}

func ensureSubscriptionOrderTableSQLite() error {
	if !platformdb.UsingSQLite {
		return nil
	}
	return ensureSubscriptionOrderTableSQLiteTx(platformdb.DB)
}

func ensureSubscriptionOrderTableSQLiteTx(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("database is not initialized")
	}
	tableName := "subscription_orders"
	if !db.Migrator().HasTable(tableName) {
		return nil
	}

	var cols []struct {
		Name string `gorm:"column:name"`
	}
	if err := db.Raw("PRAGMA table_info(`" + tableName + "`)").Scan(&cols).Error; err != nil {
		return err
	}
	existing := make(map[string]struct{}, len(cols))
	for _, c := range cols {
		existing[c.Name] = struct{}{}
	}
	required := []sqliteColumnDef{
		{Name: "purchase_type", DDL: "`purchase_type` varchar(32) DEFAULT 'normal'"},
		{Name: "fulfillment_status", DDL: "`fulfillment_status` varchar(32) DEFAULT 'completed'"},
		{Name: "target_subscription_id", DDL: "`target_subscription_id` int DEFAULT 0"},
		{Name: "booster_quota", DDL: "`booster_quota` bigint DEFAULT 0"},
		{Name: "booster_rate", DDL: "`booster_rate` decimal(10,6) DEFAULT 0"},
		{Name: "booster_expires_at", DDL: "`booster_expires_at` bigint DEFAULT 0"},
		{Name: "fuel_quota", DDL: "`fuel_quota` bigint DEFAULT 0"},
		{Name: "fuel_unit_price", DDL: "`fuel_unit_price` decimal(10,4) DEFAULT 0"},
		{Name: "fuel_expires_at", DDL: "`fuel_expires_at` bigint DEFAULT 0"},
	}
	for _, col := range required {
		if _, ok := existing[col.Name]; ok {
			continue
		}
		if err := db.Exec("ALTER TABLE `" + tableName + "` ADD COLUMN " + col.DDL).Error; err != nil {
			return err
		}
	}
	return nil
}

func ensureUserSubscriptionTableSQLiteTx(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("database is not initialized")
	}
	const tableName = "user_subscriptions"
	var cols []struct {
		Name string `gorm:"column:name"`
	}
	if err := db.Raw("PRAGMA table_info(`" + tableName + "`)").Scan(&cols).Error; err != nil {
		return err
	}
	existing := make(map[string]struct{}, len(cols))
	for _, col := range cols {
		existing[col.Name] = struct{}{}
	}
	required := []sqliteColumnDef{
		{Name: "period_amount", DDL: "`period_amount` bigint NOT NULL DEFAULT 0"},
		{Name: "period_used", DDL: "`period_used` bigint NOT NULL DEFAULT 0"},
		{Name: "model_limits", DDL: "`model_limits` text"},
		{Name: "model_usage", DDL: "`model_usage` text"},
		{Name: "last_reset_time", DDL: "`last_reset_time` bigint DEFAULT 0"},
		{Name: "next_reset_time", DDL: "`next_reset_time` bigint DEFAULT 0"},
		{Name: "upgrade_group", DDL: "`upgrade_group` varchar(64) DEFAULT ''"},
		{Name: "prev_user_group", DDL: "`prev_user_group` varchar(64) DEFAULT ''"},
	}
	for _, col := range required {
		if _, ok := existing[col.Name]; ok {
			continue
		}
		if err := db.Exec("ALTER TABLE `" + tableName + "` ADD COLUMN " + col.DDL).Error; err != nil {
			return err
		}
	}
	return nil
}

func migrateTokenModelLimitsToText() error {
	if platformdb.UsingSQLite {
		return nil
	}

	tableName := "tokens"
	columnName := "model_limits"

	if !platformdb.DB.Migrator().HasTable(tableName) {
		return nil
	}
	if !platformdb.DB.Migrator().HasColumn(&identityschema.Token{}, columnName) {
		return nil
	}

	var alterSQL string
	if platformdb.UsingPostgreSQL {
		var dataType string
		if err := platformdb.DB.Raw(`SELECT data_type FROM information_schema.columns
			WHERE table_schema = current_schema() AND table_name = ? AND column_name = ?`,
			tableName, columnName).Scan(&dataType).Error; err == nil && dataType == "text" {
			return nil
		}
		alterSQL = fmt.Sprintf(`ALTER TABLE %s ALTER COLUMN %s TYPE text`, tableName, columnName)
	} else if platformdb.UsingMySQL {
		var columnType string
		if err := platformdb.DB.Raw(`SELECT COLUMN_TYPE FROM information_schema.columns
				WHERE table_schema = DATABASE() AND table_name = ? AND column_name = ?`,
			tableName, columnName).Scan(&columnType).Error; err == nil && strings.ToLower(columnType) == "text" {
			return nil
		}
		alterSQL = fmt.Sprintf("ALTER TABLE %s MODIFY COLUMN %s text", tableName, columnName)
	} else {
		return nil
	}

	if alterSQL != "" {
		if err := platformdb.DB.Exec(alterSQL).Error; err != nil {
			return fmt.Errorf("failed to migrate %s.%s to text: %w", tableName, columnName, err)
		}
		platformobservability.SysLog(fmt.Sprintf("Successfully migrated %s.%s to text", tableName, columnName))
	}
	return nil
}

func migrateSubscriptionPlanPriceAmount() {
	if platformdb.UsingSQLite {
		return
	}

	tableName := "subscription_plans"
	columnName := "price_amount"

	if !platformdb.DB.Migrator().HasTable(tableName) {
		return
	}
	if !platformdb.DB.Migrator().HasColumn(&commerceschema.SubscriptionPlan{}, columnName) {
		return
	}

	var alterSQL string
	if platformdb.UsingPostgreSQL {
		var dataType string
		if err := platformdb.DB.Raw(`SELECT data_type FROM information_schema.columns
			WHERE table_schema = current_schema() AND table_name = ? AND column_name = ?`,
			tableName, columnName).Scan(&dataType).Error; err == nil && dataType == "numeric" {
			return
		}
		alterSQL = fmt.Sprintf(`ALTER TABLE %s ALTER COLUMN %s TYPE decimal(10,6) USING %s::decimal(10,6)`,
			tableName, columnName, columnName)
	} else if platformdb.UsingMySQL {
		var columnType string
		if err := platformdb.DB.Raw(`SELECT COLUMN_TYPE FROM information_schema.columns
				WHERE table_schema = DATABASE() AND table_name = ? AND column_name = ?`,
			tableName, columnName).Scan(&columnType).Error; err == nil && strings.HasPrefix(strings.ToLower(columnType), "decimal") {
			return
		}
		alterSQL = fmt.Sprintf("ALTER TABLE %s MODIFY COLUMN %s decimal(10,6) NOT NULL DEFAULT 0", tableName, columnName)
	} else {
		return
	}

	if alterSQL != "" {
		if err := platformdb.DB.Exec(alterSQL).Error; err != nil {
			platformobservability.SysLog(fmt.Sprintf("Warning: failed to migrate %s.%s to decimal: %v", tableName, columnName, err))
		} else {
			platformobservability.SysLog(fmt.Sprintf("Successfully migrated %s.%s to decimal(10,6)", tableName, columnName))
		}
	}
}

func closeDatabaseHandle(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

func checkMySQLChineseSupport(db *gorm.DB) error {
	var schemaCharset string
	var schemaCollation string
	err := db.Raw("SELECT DEFAULT_CHARACTER_SET_NAME, DEFAULT_COLLATION_NAME FROM information_schema.SCHEMATA WHERE SCHEMA_NAME = DATABASE()").Row().Scan(&schemaCharset, &schemaCollation)
	if err != nil {
		return fmt.Errorf("读取当前库默认字符集/排序规则失败 / Failed to read schema default charset/collation: %v", err)
	}

	toLower := func(s string) string { return strings.ToLower(s) }
	allowedCharsets := map[string]string{
		"utf8mb4": "utf8mb4_",
		"utf8":    "utf8_",
		"gbk":     "gbk_",
		"big5":    "big5_",
		"gb18030": "gb18030_",
	}
	isChineseCapable := func(cs string, cl string) bool {
		csLower := toLower(cs)
		clLower := toLower(cl)
		if prefix, ok := allowedCharsets[csLower]; ok {
			if clLower == "" {
				return true
			}
			return strings.HasPrefix(clLower, prefix)
		}
		for _, prefix := range allowedCharsets {
			if strings.HasPrefix(clLower, prefix) {
				return true
			}
		}
		return false
	}

	if !isChineseCapable(schemaCharset, schemaCollation) {
		return fmt.Errorf("当前库默认字符集/排序规则不支持中文：schema(%s/%s)。请将库设置为 utf8mb4/utf8/gbk/big5/gb18030 / Schema default charset/collation is not Chinese-capable: schema(%s/%s). Please set to utf8mb4/utf8/gbk/big5/gb18030",
			schemaCharset, schemaCollation, schemaCharset, schemaCollation)
	}

	type tableInfo struct {
		Name      string
		Collation *string
	}
	var tables []tableInfo
	if err := db.Raw("SELECT TABLE_NAME, TABLE_COLLATION FROM information_schema.TABLES WHERE TABLE_SCHEMA = DATABASE() AND TABLE_TYPE = 'BASE TABLE'").Scan(&tables).Error; err != nil {
		return fmt.Errorf("读取表排序规则失败 / Failed to read table collations: %v", err)
	}

	var badTables []string
	for _, t := range tables {
		if t.Collation == nil || *t.Collation == "" {
			continue
		}
		lower := strings.ToLower(*t.Collation)
		ok := false
		for _, prefix := range allowedCharsets {
			if strings.HasPrefix(lower, prefix) {
				ok = true
				break
			}
		}
		if !ok {
			badTables = append(badTables, fmt.Sprintf("%s(%s)", t.Name, *t.Collation))
		}
	}

	if len(badTables) > 0 {
		maxShow := 20
		shown := badTables
		if len(shown) > maxShow {
			shown = shown[:maxShow]
		}
		return fmt.Errorf(
			"存在不支持中文的表，请修复其排序规则/字符集。示例（最多展示 %d 项）：%v / Found tables not Chinese-capable. Please fix their collation/charset. Examples (showing up to %d): %v",
			maxShow, shown, maxShow, shown,
		)
	}
	return nil
}
