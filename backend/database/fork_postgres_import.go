package database

import (
	"aura/models"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

var postgresImportTables = []string{"AUTH", "Config", "MediaItems", "Movies", "Series", "Seasons", "Episodes", "PosterSets", "ImageFiles", "SavedItems", "IgnoredItems", "DownloadQueueJobs", "DownloadHistory", "SiloItemIDs", "Collections", "CollectionItems", "DownloadEvents", "StatisticsSnapshots"}

// SQLiteImportReport contains counts only; configuration and auth values must never reach CLI output.
type SQLiteImportReport struct {
	Tables         map[string]int64
	ResetQueueJobs int64
}

// ImportSQLite copies an upgraded offline snapshot atomically into an initialized, empty PostgreSQL database.
func (p *PostgresDB) ImportSQLite(ctx context.Context, source *sql.DB) (report SQLiteImportReport, err error) {
	report.Tables = make(map[string]int64)
	if err = validateSQLiteImportSchema(ctx, source); err != nil {
		return report, err
	}
	tx, err := p.conn.BeginTx(ctx, nil)
	if err != nil {
		return report, errors.New("cannot start PostgreSQL import")
	}
	defer tx.Rollback()
	for _, lock := range []int64{1096110657, 1096110658, 1096110659} {
		if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock($1)`, lock); err != nil {
			return report, errors.New("cannot lock PostgreSQL import")
		}
	}
	names := make([]string, len(postgresImportTables))
	for i, table := range postgresImportTables {
		names[i] = pgx.Identifier{strings.ToLower(table)}.Sanitize()
	}
	if _, err = tx.ExecContext(ctx, "LOCK TABLE "+strings.Join(names, ",")+" IN ACCESS EXCLUSIVE MODE"); err != nil {
		return report, errors.New("cannot lock import tables")
	}
	for i, table := range postgresImportTables {
		var count int64
		if err = tx.QueryRowContext(ctx, "SELECT count(*) FROM "+names[i]).Scan(&count); err != nil {
			return report, fmt.Errorf("cannot inspect target table %s", table)
		}
		if count != 0 {
			return report, fmt.Errorf("target table %s is not empty; import refused", table)
		}
	}
	for _, table := range postgresImportTables {
		count, reset, copyErr := copySQLiteImportTable(ctx, source, tx, table)
		if copyErr != nil {
			return report, copyErr
		}
		report.Tables[table] = count
		report.ResetQueueJobs += reset
	}
	if _, err = tx.ExecContext(ctx, `SET CONSTRAINTS ALL IMMEDIATE`); err != nil {
		return report, errors.New("imported relationships are invalid")
	}
	saved, info := postgresGetAllSavedSets(ctx, tx, models.DBFilter{ItemsPerPage: -1})
	if info.Message != "" {
		return report, errors.New("imported saved items failed application read validation")
	}
	var sourceSaved int64
	if err = source.QueryRowContext(ctx, `SELECT count(*) FROM MediaItems m WHERE EXISTS (SELECT 1 FROM SavedItems s WHERE s.tmdb_id=m.tmdb_id AND s.library_title=m.library_title AND s.edition=m.edition)`).Scan(&sourceSaved); err != nil || saved.Total != int(sourceSaved) {
		return report, errors.New("saved item counts differ after import")
	}
	// ALTER ... RESTART is transactional; setval would leak sequence changes after a rollback.
	for _, table := range []string{"MediaItems", "Movies", "Series", "Seasons", "Episodes", "PosterSets", "ImageFiles", "DownloadQueueJobs", "DownloadHistory", "DownloadEvents"} {
		name := pgx.Identifier{strings.ToLower(table)}.Sanitize()
		var maxID int64
		if err = tx.QueryRowContext(ctx, "SELECT COALESCE(MAX(id),0) FROM "+name).Scan(&maxID); err != nil {
			return report, fmt.Errorf("cannot read %s sequence", table)
		}
		if maxID == math.MaxInt64 {
			return report, fmt.Errorf("%s ID sequence is exhausted", table)
		}
		nextID := max(int64(1), maxID+1)
		if _, err = tx.ExecContext(ctx, "ALTER TABLE "+name+" ALTER COLUMN id RESTART WITH "+strconv.FormatInt(nextID, 10)); err != nil {
			return report, fmt.Errorf("cannot reset %s sequence", table)
		}
	}
	if err = tx.Commit(); err != nil {
		return report, errors.New("cannot commit PostgreSQL import")
	}
	return report, nil
}

// validateSQLiteImportSchema refuses unknown state rather than silently dropping an upstream addition.
func validateSQLiteImportSchema(ctx context.Context, source *sql.DB) error {
	var version, configVersion int
	if err := source.QueryRowContext(ctx, `SELECT version FROM VERSION`).Scan(&version); err != nil || version != postgresUpstreamVersion {
		return errors.New("SQLite snapshot must be upgraded to the supported schema before import")
	}
	if err := source.QueryRowContext(ctx, `SELECT version FROM ConfigSchemaVersion`).Scan(&configVersion); err != nil || configVersion != postgresConfigVersion {
		return errors.New("unsupported SQLite configuration schema")
	}
	rows, err := source.QueryContext(ctx, `SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'`)
	if err != nil {
		return errors.New("cannot inspect SQLite schema")
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		if err = rows.Scan(&name); err != nil {
			return errors.New("cannot read SQLite schema")
		}
		if !slices.Contains(postgresImportTables, name) && name != "VERSION" && name != "ConfigSchemaVersion" {
			return fmt.Errorf("unsupported SQLite table %q; review import compatibility", name)
		}
	}
	if err = rows.Err(); err != nil {
		return errors.New("cannot read SQLite schema")
	}
	check, err := source.QueryContext(ctx, `PRAGMA foreign_key_check`)
	if err != nil {
		return errors.New("cannot validate SQLite relationships")
	}
	defer check.Close()
	if check.Next() {
		return errors.New("SQLite snapshot contains invalid relationships")
	}
	if check.Err() != nil {
		return errors.New("cannot validate SQLite relationships")
	}
	return nil
}

// copySQLiteImportTable compares both column sets before using identifiers from either catalog.
func copySQLiteImportTable(ctx context.Context, source *sql.DB, tx *sql.Tx, table string) (count, reset int64, err error) {
	name := pgx.Identifier{strings.ToLower(table)}.Sanitize()
	rows, err := source.QueryContext(ctx, "SELECT * FROM "+name+" LIMIT 0")
	if err != nil {
		return 0, 0, fmt.Errorf("cannot inspect SQLite table %s", table)
	}
	columns, err := rows.Columns()
	rows.Close()
	if err != nil {
		return 0, 0, fmt.Errorf("cannot read SQLite columns for %s", table)
	}
	targetTypes := map[string]string{}
	targetRows, err := tx.QueryContext(ctx, `SELECT column_name,data_type FROM information_schema.columns WHERE table_schema=current_schema() AND table_name=$1`, strings.ToLower(table))
	if err != nil {
		return 0, 0, fmt.Errorf("cannot inspect PostgreSQL table %s", table)
	}
	for targetRows.Next() {
		var column, kind string
		if err = targetRows.Scan(&column, &kind); err != nil {
			targetRows.Close()
			return 0, 0, fmt.Errorf("cannot read PostgreSQL columns for %s", table)
		}
		targetTypes[column] = kind
	}
	err = targetRows.Err()
	targetRows.Close()
	if err != nil {
		return 0, 0, fmt.Errorf("cannot read PostgreSQL columns for %s", table)
	}
	if table == "AUTH" {
		delete(targetTypes, "singleton")
	}
	if len(columns) != len(targetTypes) {
		return 0, 0, fmt.Errorf("column mismatch for %s; review import compatibility", table)
	}
	quoted, binds := make([]string, len(columns)), make([]string, len(columns))
	for i, column := range columns {
		if _, ok := targetTypes[column]; !ok {
			return 0, 0, fmt.Errorf("unsupported column in %s; review import compatibility", table)
		}
		quoted[i] = pgx.Identifier{column}.Sanitize()
		binds[i] = "$" + strconv.Itoa(i+1)
	}
	rows, err = source.QueryContext(ctx, "SELECT "+strings.Join(quoted, ",")+" FROM "+name)
	if err != nil {
		return 0, 0, fmt.Errorf("cannot read SQLite table %s", table)
	}
	defer rows.Close()
	statement, err := tx.PrepareContext(ctx, "INSERT INTO "+name+" ("+strings.Join(quoted, ",")+") VALUES ("+strings.Join(binds, ",")+")")
	if err != nil {
		return 0, 0, fmt.Errorf("cannot prepare import for %s", table)
	}
	defer statement.Close()
	for rows.Next() {
		values := make([]any, len(columns))
		dest := make([]any, len(columns))
		for i := range values {
			dest[i] = &values[i]
		}
		if err = rows.Scan(dest...); err != nil {
			return count, reset, fmt.Errorf("cannot scan SQLite table %s", table)
		}
		interrupted := table == "DownloadQueueJobs" && slices.ContainsFunc(columns, func(column string) bool {
			return column == "status" && importText(values[slices.Index(columns, column)]) == "processing"
		})
		if interrupted {
			reset++
		}
		for i, column := range columns {
			if interrupted {
				switch column {
				case "status":
					values[i] = "pending"
				case "worker_id":
					values[i] = ""
				case "started_at", "finished_at":
					values[i] = nil
				}
			}
			values[i], err = sqliteImportValue(table, column, targetTypes[column], values[i])
			if err != nil {
				return count, reset, err
			}
		}
		if _, err = statement.ExecContext(ctx, values...); err != nil {
			return count, reset, fmt.Errorf("cannot import %s row %d; check source constraints", table, count+1)
		}
		count++
	}
	if rows.Err() != nil {
		return count, reset, fmt.Errorf("cannot finish reading SQLite table %s", table)
	}
	var copied int64
	if err = tx.QueryRowContext(ctx, "SELECT count(*) FROM "+name).Scan(&copied); err != nil || copied != count {
		return count, reset, fmt.Errorf("row count mismatch for %s", table)
	}
	return count, reset, nil
}

// importText handles SQLite driver values without logging their potentially secret contents.
func importText(value any) string {
	if bytes, ok := value.([]byte); ok {
		return string(bytes)
	}
	text, _ := value.(string)
	return text
}

// sqliteImportValue converts only storage representations that differ between the two engines.
func sqliteImportValue(table, column, kind string, value any) (any, error) {
	if value == nil {
		return nil, nil
	}
	if table == "IgnoredItems" && column == "current_sets" {
		sets := []string{}
		if text := importText(value); text != "" {
			sets = strings.Split(text, ",")
		}
		data, _ := json.Marshal(sets)
		return string(data), nil
	}
	switch kind {
	case "jsonb":
		text := importText(value)
		if !json.Valid([]byte(text)) {
			return nil, fmt.Errorf("invalid JSON in %s.%s", table, column)
		}
		return text, nil
	case "timestamp with time zone":
		if date, ok := value.(time.Time); ok {
			return date, nil
		}
		for _, layout := range []string{time.RFC3339Nano, "2006-01-02 15:04:05.999999999Z07:00", "2006-01-02 15:04:05.999999999-07:00", "2006-01-02 15:04:05.999999999", "2006-01-02"} {
			if date, err := time.ParseInLocation(layout, importText(value), time.UTC); err == nil {
				return date, nil
			}
		}
		return nil, fmt.Errorf("invalid timestamp in %s.%s", table, column)
	}
	return value, nil
}
