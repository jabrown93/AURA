package database

import (
	"aura/logging"
	"context"
	"database/sql"
	"errors"
)

// CreateAuthTable verifies the table created by PostgreSQL's transactional schema migration.
func (p *PostgresDB) CreateAuthTable(ctx context.Context) logging.LogErrorInfo {
	_, err := p.conn.ExecContext(ctx, "SELECT token_secret FROM AUTH LIMIT 0")
	return postgresError("Failed to verify PostgreSQL auth table", err)
}

// GetAuthTokenSecret atomically preserves one secret even when requests initialize auth concurrently.
func (p *PostgresDB) GetAuthTokenSecret(ctx context.Context) (string, logging.LogErrorInfo) {
	var secret string
	err := p.conn.QueryRowContext(ctx, "SELECT token_secret FROM AUTH WHERE singleton = 1").Scan(&secret)
	if err == nil {
		return secret, logging.LogErrorInfo{}
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", postgresError("Failed to read PostgreSQL auth secret", err)
	}
	secret, err = generateTokenAuthSecret()
	if err != nil {
		return "", postgresError("Failed to generate auth secret", err)
	}
	if _, err = p.conn.ExecContext(ctx, "INSERT INTO AUTH (token_secret) VALUES ($1) ON CONFLICT (singleton) DO NOTHING", secret); err != nil {
		return "", postgresError("Failed to save PostgreSQL auth secret", err)
	}
	err = p.conn.QueryRowContext(ctx, "SELECT token_secret FROM AUTH WHERE singleton = 1").Scan(&secret)
	return secret, postgresError("Failed to read PostgreSQL auth secret", err)
}

// GetAllConfigValues preserves the upstream field-per-row configuration format.
func (p *PostgresDB) GetAllConfigValues(ctx context.Context) (map[string]string, logging.LogErrorInfo) {
	values := map[string]string{}
	rows, err := p.conn.QueryContext(ctx, "SELECT key, value FROM Config")
	if err != nil {
		return nil, postgresError("Failed to read PostgreSQL configuration", err)
	}
	defer rows.Close()
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, postgresError("Failed to scan PostgreSQL configuration", err)
		}
		values[key] = value
	}
	return values, postgresError("Failed to read PostgreSQL configuration", rows.Err())
}

// SaveConfigValues changes values atomically and leaves unchanged rows' timestamps untouched.
func (p *PostgresDB) SaveConfigValues(ctx context.Context, values map[string]string, replace bool) logging.LogErrorInfo {
	tx, err := p.conn.BeginTx(ctx, nil)
	if err != nil {
		return postgresError("Failed to begin configuration save", err)
	}
	defer tx.Rollback()
	if replace {
		if _, err := tx.ExecContext(ctx, "DELETE FROM Config"); err != nil {
			return postgresError("Failed to replace PostgreSQL configuration", err)
		}
	}
	for key, value := range values {
		if _, err := tx.ExecContext(ctx, `INSERT INTO Config (key, value) VALUES ($1, $2)
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = CURRENT_TIMESTAMP
WHERE Config.value IS DISTINCT FROM EXCLUDED.value`, key, value); err != nil {
			return postgresError("Failed to save PostgreSQL configuration", err)
		}
	}
	return postgresError("Failed to commit configuration save", tx.Commit())
}
