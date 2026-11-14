package database

import (
	"aura/logging"
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib"
)

const postgresSchemaVersion = 1
const postgresUpstreamVersion = 18
const postgresConfigVersion = 5

//go:embed fork_postgres_schema.sql
var postgresSchemaSQL string

// PostgresDB keeps fork-owned SQL separate from upstream's SQLite implementation.
// A deployment runs one AURA instance and manages PostgreSQL backups externally.
type PostgresDB struct {
	conn *sql.DB
	dsn  string
}

// postgresError excludes driver text: connection errors can include credentials.
func postgresError(action string, err error) logging.LogErrorInfo {
	if err == nil {
		return logging.LogErrorInfo{}
	}
	info := logging.LogErrorInfo{Message: action}
	var serverError *pgconn.PgError
	if errors.As(err, &serverError) {
		info.Detail = map[string]any{"sqlstate": serverError.Code}
	}
	return info
}

// GetDBConnection opens the configured target without logging or persisting its DSN.
func (p *PostgresDB) GetDBConnection(ctx context.Context) (*sql.DB, bool, logging.LogErrorInfo) {
	if p.conn != nil {
		return p.conn, false, logging.LogErrorInfo{}
	}
	if p.dsn == "" {
		return nil, false, logging.LogErrorInfo{Message: "AURA_POSTGRES_DSN is required for PostgreSQL"}
	}
	conn, err := sql.Open("pgx", p.dsn)
	if err != nil {
		return nil, false, postgresError("Invalid PostgreSQL connection configuration", err)
	}
	// Bound startup even when an operator omits connect_timeout from the DSN.
	pingCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if err = conn.PingContext(pingCtx); err != nil {
		conn.Close()
		return nil, false, postgresError("Failed to connect to PostgreSQL", err)
	}
	conn.SetMaxOpenConns(10)
	conn.SetMaxIdleConns(5)
	p.conn = conn
	return conn, false, logging.LogErrorInfo{}
}

// Init migrates PostgreSQL before callers load database-backed configuration.
func (p *PostgresDB) Init(ctx context.Context) (bool, logging.LogErrorInfo) {
	if _, _, info := p.GetDBConnection(ctx); info.Message != "" {
		return false, info
	}
	tx, err := p.conn.BeginTx(ctx, nil)
	if err != nil {
		return false, postgresError("Failed to begin PostgreSQL schema migration", err)
	}
	defer tx.Rollback()
	// Transaction-scoped lock serializes schema changes and releases on every exit path.
	if _, err = tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(1096110657)"); err != nil {
		return false, postgresError("Failed to lock PostgreSQL schema", err)
	}
	var exists bool
	if err = tx.QueryRowContext(ctx, "SELECT to_regclass('aura_schema_migrations') IS NOT NULL").Scan(&exists); err != nil {
		return false, postgresError("Failed to read PostgreSQL schema state", err)
	}
	if !exists {
		var tableCount int
		if err = tx.QueryRowContext(ctx, "SELECT count(*) FROM pg_tables WHERE schemaname = current_schema()").Scan(&tableCount); err != nil {
			return false, postgresError("Failed to inspect PostgreSQL schema", err)
		}
		if tableCount != 0 {
			return false, logging.LogErrorInfo{Message: "PostgreSQL requires an empty schema for first initialization"}
		}
		if _, err = tx.ExecContext(ctx, postgresSchemaSQL); err != nil {
			return false, postgresError("Failed to create PostgreSQL schema", err)
		}
		if _, err = tx.ExecContext(ctx, "INSERT INTO ConfigSchemaVersion VALUES ($1)", postgresConfigVersion); err != nil {
			return false, postgresError("Failed to record PostgreSQL config version", err)
		}
		if _, err = tx.ExecContext(ctx, `CREATE TABLE aura_schema_migrations (version BIGINT PRIMARY KEY, upstream_version BIGINT NOT NULL);
INSERT INTO aura_schema_migrations VALUES (1, 18);`); err != nil {
			return false, postgresError("Failed to record PostgreSQL schema version", err)
		}
	}
	var version, upstreamVersion, versions int
	if err = tx.QueryRowContext(ctx, "SELECT max(version), max(upstream_version), count(*) FROM aura_schema_migrations").Scan(&version, &upstreamVersion, &versions); err != nil {
		return false, postgresError("Failed to read PostgreSQL schema version", err)
	}
	if version != postgresSchemaVersion || versions != 1 || upstreamVersion != postgresUpstreamVersion || postgresUpstreamVersion != LATEST_DB_VERSION || postgresConfigVersion != LATEST_CONFIG_SCHEMA_VERSION {
		return false, logging.LogErrorInfo{Message: "Unsupported PostgreSQL schema or upstream storage version"}
	}
	var configVersion int
	if err = tx.QueryRowContext(ctx, "SELECT version FROM ConfigSchemaVersion").Scan(&configVersion); err != nil {
		return false, postgresError("Failed to read PostgreSQL config version", err)
	}
	if configVersion != postgresConfigVersion {
		return false, logging.LogErrorInfo{Message: "Unsupported PostgreSQL configuration schema"}
	}
	if err = tx.Commit(); err != nil {
		return false, postgresError("Failed to commit PostgreSQL schema migration", err)
	}
	return !exists, logging.LogErrorInfo{}
}

// Close releases the connection pool; PostgreSQL needs no SQLite WAL checkpoint.
func (p *PostgresDB) Close(context.Context) logging.LogErrorInfo {
	if p.conn == nil {
		return logging.LogErrorInfo{}
	}
	err := p.conn.Close()
	p.conn = nil
	return postgresError("Failed to close PostgreSQL", err)
}

// Check verifies that PostgreSQL remains reachable.
func (p *PostgresDB) Check(ctx context.Context) error {
	if p.conn == nil {
		return errors.New("the database is not open")
	}
	if err := p.conn.PingContext(ctx); err != nil {
		return errors.New("PostgreSQL is not reachable")
	}
	return nil
}

// CreateTables verifies the independently versioned PostgreSQL schema.
func (p *PostgresDB) CreateTables(ctx context.Context) logging.LogErrorInfo {
	_, info := p.Init(ctx)
	return info
}

// CreateVersionTable uses the PostgreSQL migration ledger instead of SQLite migrations.
func (p *PostgresDB) CreateVersionTable(ctx context.Context) logging.LogErrorInfo {
	return p.CreateTables(ctx)
}

// GetCurrentVersion reports the upstream storage contract implemented by this schema.
func (p *PostgresDB) GetCurrentVersion(ctx context.Context) (int, logging.LogErrorInfo) {
	var version int
	err := p.conn.QueryRowContext(ctx, "SELECT version FROM VERSION").Scan(&version)
	return version, postgresError("Failed to read PostgreSQL storage version", err)
}

// UpdateVersionTable cannot silently claim support for an unported upstream schema.
func (p *PostgresDB) UpdateVersionTable(ctx context.Context, version int) logging.LogErrorInfo {
	if version != postgresUpstreamVersion {
		return logging.LogErrorInfo{Message: "Unsupported PostgreSQL upstream storage version"}
	}
	_, err := p.conn.ExecContext(ctx, "UPDATE VERSION SET version = $1", version)
	return postgresError("Failed to update PostgreSQL storage version", err)
}

// Vacuum is not an AURA maintenance operation on an operator-managed PostgreSQL server.
func (p *PostgresDB) Vacuum(context.Context) logging.LogErrorInfo {
	return logging.LogErrorInfo{Message: "PostgreSQL maintenance is operator-managed"}
}

// CreateBackup never presents an SQLite file operation as a PostgreSQL backup.
func (p *PostgresDB) CreateBackup(context.Context, int) (DatabaseBackup, logging.LogErrorInfo) {
	return DatabaseBackup{}, logging.LogErrorInfo{Message: "PostgreSQL backups are operator-managed; use pg_dump or server backups"}
}

// Backup is only used by SQLite migrations, which must not run against PostgreSQL.
func (p *PostgresDB) Backup(context.Context, int, int) logging.LogErrorInfo {
	return logging.LogErrorInfo{Message: "SQLite migration backups are unavailable for PostgreSQL"}
}
