package database

import (
	"aura/logging"
	"os"
	"strings"
)

// forkDatabaseClient keeps deployment-only connection details out of application configuration.
func forkDatabaseClient() (DB, logging.LogErrorInfo) {
	switch strings.TrimSpace(os.Getenv("AURA_DB_DRIVER")) {
	case "", "sqlite":
		return &SQliteDB{}, logging.LogErrorInfo{}
	case "postgres":
		dsn := strings.TrimSpace(os.Getenv("AURA_POSTGRES_DSN"))
		if dsn == "" {
			return nil, logging.LogErrorInfo{Message: "AURA_POSTGRES_DSN is required for PostgreSQL"}
		}
		return &PostgresDB{dsn: dsn}, logging.LogErrorInfo{}
	default:
		return nil, logging.LogErrorInfo{Message: "AURA_DB_DRIVER must be sqlite or postgres"}
	}
}

// IsPostgres identifies operations that must not use SQLite migrations or file maintenance.
func IsPostgres() bool {
	_, ok := Client.(*PostgresDB)
	return ok
}
