package db

import "gorm.io/gorm"

const (
	DatabaseTypeMySQL      = "mysql"
	DatabaseTypeSQLite     = "sqlite"
	DatabaseTypePostgreSQL = "postgres"
)

var UsingSQLite = false
var UsingPostgreSQL = false
var LogSQLType = DatabaseTypeSQLite
var UsingMySQL = false
var UsingClickHouse = false

var SQLitePath = "codego-api.db?_busy_timeout=30000"

// DB is the primary application database handle. It belongs to the platform
// runtime rather than to any business module.
var DB *gorm.DB

// LogDB is the optional dedicated database handle for audit logs.
var LogDB *gorm.DB
