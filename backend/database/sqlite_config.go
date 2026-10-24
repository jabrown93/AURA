package database

import (
	"aura/logging"
	"context"
	"database/sql"
	"fmt"
)

// The config is stored with one row per config field.
// The config tables are created on startup instead of in the database migrations, as the config is loaded first.
// They have their own schema version and migrations.
func (s *SQliteDB) CreateConfigTables(ctx context.Context) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Creating Config Tables", logging.LevelDebug)
	defer logAction.Complete()

	query := `
CREATE TABLE IF NOT EXISTS Config (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS ConfigSchemaVersion (
    version INTEGER NOT NULL
);
	`
	_, err := s.conn.ExecContext(ctx, query)
	if err != nil {
		logAction.SetError("Failed to create Config tables", err.Error(), map[string]any{
			"error": err.Error(),
			"query": query,
		})
		return *logAction.Error
	}

	Err = s.migrateConfigSchema(ctx)
	if Err.Message != "" {
		return Err
	}

	return logging.LogErrorInfo{}
}

// migrateConfigSchema runs any config migrations newer than the stored config schema version
func (s *SQliteDB) migrateConfigSchema(ctx context.Context) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Migrating Config Schema", logging.LevelDebug)
	defer logAction.Complete()

	tx, err := s.conn.BeginTx(ctx, nil)
	if err != nil {
		logAction.SetError("Failed to start transaction", err.Error(), map[string]any{"error": err.Error()})
		return *logAction.Error
	}
	defer func() { _ = tx.Rollback() }()

	var currentVersion int
	err = tx.QueryRowContext(ctx, `SELECT version FROM ConfigSchemaVersion LIMIT 1;`).Scan(&currentVersion)
	switch {
	case err == sql.ErrNoRows:
		// No config stored yet, so start at the latest version
		if _, err := tx.ExecContext(ctx, `INSERT INTO ConfigSchemaVersion (version) VALUES (?);`, LATEST_CONFIG_SCHEMA_VERSION); err != nil {
			logAction.SetError("Failed to set config schema version", err.Error(), map[string]any{"error": err.Error()})
			return *logAction.Error
		}
	case err != nil:
		logAction.SetError("Failed to get config schema version", err.Error(), map[string]any{"error": err.Error()})
		return *logAction.Error
	case currentVersion > LATEST_CONFIG_SCHEMA_VERSION:
		// Saved by a newer version of aura. Unknown values are ignored
		logging.LOGGER.Warn().Timestamp().Int("config_schema_version", currentVersion).Int("latest_config_schema_version", LATEST_CONFIG_SCHEMA_VERSION).
			Msg("The config was saved by a newer version of aura")
	case currentVersion < LATEST_CONFIG_SCHEMA_VERSION:
		for version := currentVersion; version < LATEST_CONFIG_SCHEMA_VERSION; version++ {
			// configMigrations[0] migrates from version 1 to 2
			migration := configMigrations[version-1]
			if err := migration.Migrate(ctx, tx); err != nil {
				logAction.SetError(fmt.Sprintf("Failed to migrate config schema from v%d to v%d", version, version+1), err.Error(), map[string]any{
					"error":       err.Error(),
					"description": migration.Description,
				})
				return *logAction.Error
			}
			logging.LOGGER.Info().Timestamp().Msgf("Migrated config schema from v%d to v%d: %s", version, version+1, migration.Description)
		}
		if _, err := tx.ExecContext(ctx, `UPDATE ConfigSchemaVersion SET version = ?;`, LATEST_CONFIG_SCHEMA_VERSION); err != nil {
			logAction.SetError("Failed to update config schema version", err.Error(), map[string]any{"error": err.Error()})
			return *logAction.Error
		}
	}

	if err := tx.Commit(); err != nil {
		logAction.SetError("Failed to commit transaction", err.Error(), map[string]any{"error": err.Error()})
		return *logAction.Error
	}

	return logging.LogErrorInfo{}
}

func (s *SQliteDB) GetAllConfigValues(ctx context.Context) (values map[string]string, Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Getting Config Values", logging.LevelTrace)
	defer logAction.Complete()

	values = map[string]string{}

	rows, err := s.conn.QueryContext(ctx, `SELECT key, value FROM Config;`)
	if err != nil {
		logAction.SetError("Failed to query Config", err.Error(), map[string]any{"error": err.Error()})
		return values, *logAction.Error
	}
	defer rows.Close()

	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			logAction.SetError("Failed to scan Config row", err.Error(), map[string]any{"error": err.Error()})
			return values, *logAction.Error
		}
		values[key] = value
	}
	if err := rows.Err(); err != nil {
		logAction.SetError("Failed while iterating over Config rows", err.Error(), map[string]any{"error": err.Error()})
		return values, *logAction.Error
	}

	return values, logging.LogErrorInfo{}
}

func (s *SQliteDB) SaveConfigValues(ctx context.Context, values map[string]string, replace bool) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Saving Config Values", logging.LevelTrace)
	defer logAction.Complete()

	tx, err := s.conn.BeginTx(ctx, nil)
	if err != nil {
		logAction.SetError("Failed to start transaction", err.Error(), map[string]any{"error": err.Error()})
		return *logAction.Error
	}
	defer func() { _ = tx.Rollback() }()

	// Get the stored values, so only changed values are written
	storedValues := map[string]string{}
	if replace {
		if _, err := tx.ExecContext(ctx, `DELETE FROM Config;`); err != nil {
			logAction.SetError("Failed to clear Config", err.Error(), map[string]any{"error": err.Error()})
			return *logAction.Error
		}
	} else {
		rows, err := tx.QueryContext(ctx, `SELECT key, value FROM Config;`)
		if err != nil {
			logAction.SetError("Failed to query Config", err.Error(), map[string]any{"error": err.Error()})
			return *logAction.Error
		}
		for rows.Next() {
			var key, value string
			if err := rows.Scan(&key, &value); err != nil {
				rows.Close()
				logAction.SetError("Failed to scan Config row", err.Error(), map[string]any{"error": err.Error()})
				return *logAction.Error
			}
			storedValues[key] = value
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			logAction.SetError("Failed while iterating over Config rows", err.Error(), map[string]any{"error": err.Error()})
			return *logAction.Error
		}
	}

	changedKeys := []string{}
	for key, value := range values {
		if storedValue, found := storedValues[key]; found && storedValue == value {
			continue
		}
		_, err := tx.ExecContext(ctx, `
INSERT INTO Config (key, value, updated_at) VALUES (?, ?, CURRENT_TIMESTAMP)
ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at;`, key, value)
		if err != nil {
			logAction.SetError("Failed to save Config value", err.Error(), map[string]any{"error": err.Error(), "key": key})
			return *logAction.Error
		}
		changedKeys = append(changedKeys, key)
	}

	if err := tx.Commit(); err != nil {
		logAction.SetError("Failed to commit transaction", err.Error(), map[string]any{"error": err.Error()})
		return *logAction.Error
	}

	logAction.AppendResult("changed_values", len(changedKeys))
	return logging.LogErrorInfo{}
}
