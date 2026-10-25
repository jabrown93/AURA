package migration

import (
	"aura/database"
	"aura/logging"
	"context"
	"fmt"
)

// Fork: the fork's own schema changes live outside upstream's numbered migrations, so a
// future upstream migration number can never collide with one of ours.
//
// The fork once shipped a v5->v6 migration that only added SavedItems.force_preload_missing,
// while upstream's v5->v6 adds the edition column. A database at version 6 without
// MediaItems.edition was migrated by the fork: it is rewound to 5 so upstream's migrations run,
// and its force_preload_missing values are stashed and restored afterwards.

const forkPreloadStash = "ForkPreloadMissingStash"

// checkSchemaSupported rejects an on-disk schema newer than this binary understands.
// Migrations are one-way, so a downgraded binary booting against a newer schema writes rows the
// schema rejects. Refusing to start keeps the per-migration backup usable.
func checkSchemaSupported(currentVersion, latestVersion int) logging.LogErrorInfo {
	if currentVersion <= latestVersion {
		return logging.LogErrorInfo{}
	}
	return logging.LogErrorInfo{
		Message: fmt.Sprintf("Database schema version %d is newer than this build supports (%d)", currentVersion, latestVersion),
		Help: fmt.Sprintf(
			"This build was started against a database written by a newer version of aura. Redeploy the newer image, or restore the backup taken before that upgrade (*_backup_v%d_to_v%d_*.db in the config directory) before running this version.",
			latestVersion, latestVersion+1,
		),
		Detail: map[string]any{
			"database_version":  currentVersion,
			"supported_version": latestVersion,
		},
	}
}

// rewindForkV6 returns the version upstream's migrations should start from.
func rewindForkV6(ctx context.Context, currentVersion int) (int, logging.LogErrorInfo) {
	if currentVersion != 6 {
		return currentVersion, logging.LogErrorInfo{}
	}
	hasEdition, Err := checkColumnExists(ctx, "MediaItems", "edition")
	if Err.Message != "" || hasEdition {
		return currentVersion, Err
	}
	hasPreload, Err := checkColumnExists(ctx, "SavedItems", "force_preload_missing")
	if Err.Message != "" {
		return currentVersion, Err
	}
	conn, _, Err := database.GetDBConnection(ctx)
	if Err.Message != "" {
		return currentVersion, Err
	}
	if hasPreload {
		// Upstream's v5->v6 rebuilds SavedItems without this column, so keep the flagged rows aside.
		if _, err := conn.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS `+forkPreloadStash+` AS
			SELECT tmdb_id, library_title, poster_set_id FROM SavedItems WHERE force_preload_missing = 1;`); err != nil {
			return currentVersion, logging.LogErrorInfo{Message: "Failed to stash force_preload_missing values", Detail: map[string]any{"error": err.Error()}}
		}
	}
	logging.LOGGER.Warn().Timestamp().Msg("Database was migrated to v6 by an older fork build, re-running upstream migrations from v5")
	return 5, database.UpdateVersionTable(ctx, 5)
}

// applyForkSchema adds the fork's columns if missing and restores stashed values. Idempotent.
func applyForkSchema(ctx context.Context) logging.LogErrorInfo {
	conn, _, Err := database.GetDBConnection(ctx)
	if Err.Message != "" {
		return Err
	}
	hasPreload, Err := checkColumnExists(ctx, "SavedItems", "force_preload_missing")
	if Err.Message != "" {
		return Err
	}
	if !hasPreload {
		if _, err := conn.ExecContext(ctx, `ALTER TABLE SavedItems ADD COLUMN force_preload_missing INTEGER NOT NULL DEFAULT 0 CHECK (force_preload_missing IN (0,1));`); err != nil {
			return logging.LogErrorInfo{Message: "Failed to add SavedItems.force_preload_missing", Detail: map[string]any{"error": err.Error()}}
		}
	}

	var stashed int
	if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?;`, forkPreloadStash).Scan(&stashed); err != nil {
		return logging.LogErrorInfo{Message: "Failed to look for stashed force_preload_missing values", Detail: map[string]any{"error": err.Error()}}
	}
	if stashed == 0 {
		return logging.LogErrorInfo{}
	}
	if _, err := conn.ExecContext(ctx, `UPDATE SavedItems SET force_preload_missing = 1
		WHERE EXISTS (SELECT 1 FROM `+forkPreloadStash+` s
			WHERE s.tmdb_id = SavedItems.tmdb_id AND s.library_title = SavedItems.library_title AND s.poster_set_id = SavedItems.poster_set_id);`); err != nil {
		return logging.LogErrorInfo{Message: "Failed to restore force_preload_missing values", Detail: map[string]any{"error": err.Error()}}
	}
	if _, err := conn.ExecContext(ctx, `DROP TABLE `+forkPreloadStash+`;`); err != nil {
		return logging.LogErrorInfo{Message: "Failed to drop the force_preload_missing stash", Detail: map[string]any{"error": err.Error()}}
	}
	return logging.LogErrorInfo{}
}
