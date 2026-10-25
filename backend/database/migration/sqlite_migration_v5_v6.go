package migration

import (
	"aura/database"
	"aura/logging"
	"context"
)

// migrate_5_to_6 adds an "edition" column to MediaItems, SavedItems, and
// IgnoredItems, and folds it into each table's uniqueness constraint so that
// multiple editions of the same TMDB item (e.g. Theatrical vs Director's Cut)
// no longer collide with each other. Existing rows get edition = ”.
func migrate_5_to_6(ctx context.Context) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Migrating Database from v5 to v6", logging.LevelInfo)
	defer logAction.Complete()
	logging.LOGGER.Info().Timestamp().Int("From Version", 5).Int("To Version", 6).Msg("Starting database migration")

	Err = logging.LogErrorInfo{}

	backupErr := database.Backup(ctx, 5, 6)
	if backupErr.Message != "" {
		return backupErr
	}

	conn, _, getDBConnErr := database.GetDBConnection(ctx)
	if getDBConnErr.Message != "" {
		return getDBConnErr
	}

	editionColumnExists, checkColumnErr := checkColumnExists(ctx, "MediaItems", "edition")
	if checkColumnErr.Message != "" {
		return checkColumnErr
	}

	if !editionColumnExists {
		tx, err := conn.BeginTx(ctx, nil)
		if err != nil {
			logAction.SetError("Failed to begin transaction for adding edition column", "", map[string]any{"error": err.Error()})
			return *logAction.Error
		}

		// --- MediaItems ---
		if _, err = tx.ExecContext(ctx, `ALTER TABLE MediaItems RENAME TO MediaItems_old;`); err != nil {
			tx.Rollback()
			logAction.SetError("Failed to rename MediaItems table", "", map[string]any{"error": err.Error()})
			return *logAction.Error
		}
		if _, err = tx.ExecContext(ctx, `
			CREATE TABLE MediaItems (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				tmdb_id TEXT NOT NULL,
				library_title TEXT NOT NULL,
				edition TEXT NOT NULL DEFAULT '',
				rating_key TEXT NOT NULL,
				type TEXT NOT NULL CHECK (type IN ('movie','show')),
				title TEXT NOT NULL,
				year INTEGER NOT NULL,
				on_server INTEGER NOT NULL DEFAULT 0 CHECK (on_server IN (0,1)),
				UNIQUE (tmdb_id, library_title, edition)
			);
		`); err != nil {
			tx.Rollback()
			logAction.SetError("Failed to create new MediaItems table with edition column", "", map[string]any{"error": err.Error()})
			return *logAction.Error
		}
		if _, err = tx.ExecContext(ctx, `
			INSERT INTO MediaItems (id, tmdb_id, library_title, edition, rating_key, type, title, year, on_server)
			SELECT id, tmdb_id, library_title, '', rating_key, type, title, year, on_server
			FROM MediaItems_old;
		`); err != nil {
			tx.Rollback()
			logAction.SetError("Failed to copy data into new MediaItems table", "", map[string]any{"error": err.Error()})
			return *logAction.Error
		}
		if _, err = tx.ExecContext(ctx, `DROP TABLE MediaItems_old;`); err != nil {
			tx.Rollback()
			logAction.SetError("Failed to drop old MediaItems table", "", map[string]any{"error": err.Error()})
			return *logAction.Error
		}

		// --- SavedItems ---
		if _, err = tx.ExecContext(ctx, `ALTER TABLE SavedItems RENAME TO SavedItems_old;`); err != nil {
			tx.Rollback()
			logAction.SetError("Failed to rename SavedItems table", "", map[string]any{"error": err.Error()})
			return *logAction.Error
		}
		if _, err = tx.ExecContext(ctx, `
			CREATE TABLE SavedItems (
				tmdb_id TEXT NOT NULL,
				library_title TEXT NOT NULL,
				edition TEXT NOT NULL DEFAULT '',
				poster_set_id INTEGER NOT NULL,

				poster_selected INTEGER NOT NULL DEFAULT 0 CHECK (poster_selected IN (0,1)),
				backdrop_selected INTEGER NOT NULL DEFAULT 0 CHECK (backdrop_selected IN (0,1)),
				season_poster_selected INTEGER NOT NULL DEFAULT 0 CHECK (season_poster_selected IN (0,1)),
				special_season_poster_selected INTEGER NOT NULL DEFAULT 0 CHECK (special_season_poster_selected IN (0,1)),
				titlecard_selected INTEGER NOT NULL DEFAULT 0 CHECK (titlecard_selected IN (0,1)),

				autodownload INTEGER NOT NULL DEFAULT 0 CHECK (autodownload IN (0,1)),
				auto_add_new_collection_items INTEGER NOT NULL DEFAULT 0 CHECK (auto_add_new_collection_items IN (0,1)),
				last_downloaded DATETIME NOT NULL,

				PRIMARY KEY (tmdb_id, library_title, edition, poster_set_id),

				FOREIGN KEY (poster_set_id) REFERENCES PosterSets(id)
					ON DELETE CASCADE
					ON UPDATE CASCADE,

				FOREIGN KEY (tmdb_id, library_title, edition) REFERENCES MediaItems(tmdb_id, library_title, edition)
					ON DELETE CASCADE
					ON UPDATE CASCADE
			) WITHOUT ROWID;
		`); err != nil {
			tx.Rollback()
			logAction.SetError("Failed to create new SavedItems table with edition column", "", map[string]any{"error": err.Error()})
			return *logAction.Error
		}
		if _, err = tx.ExecContext(ctx, `
			INSERT INTO SavedItems (
				tmdb_id, library_title, edition, poster_set_id,
				poster_selected, backdrop_selected, season_poster_selected, special_season_poster_selected, titlecard_selected,
				autodownload, auto_add_new_collection_items, last_downloaded
			)
			SELECT
				tmdb_id, library_title, '', poster_set_id,
				poster_selected, backdrop_selected, season_poster_selected, special_season_poster_selected, titlecard_selected,
				autodownload, auto_add_new_collection_items, last_downloaded
			FROM SavedItems_old;
		`); err != nil {
			tx.Rollback()
			logAction.SetError("Failed to copy data into new SavedItems table", "", map[string]any{"error": err.Error()})
			return *logAction.Error
		}
		if _, err = tx.ExecContext(ctx, `DROP TABLE SavedItems_old;`); err != nil {
			tx.Rollback()
			logAction.SetError("Failed to drop old SavedItems table", "", map[string]any{"error": err.Error()})
			return *logAction.Error
		}

		// --- IgnoredItems ---
		if _, err = tx.ExecContext(ctx, `ALTER TABLE IgnoredItems RENAME TO IgnoredItems_old;`); err != nil {
			tx.Rollback()
			logAction.SetError("Failed to rename IgnoredItems table", "", map[string]any{"error": err.Error()})
			return *logAction.Error
		}
		if _, err = tx.ExecContext(ctx, `
			CREATE TABLE IgnoredItems (
				tmdb_id TEXT NOT NULL,
				library_title TEXT NOT NULL,
				edition TEXT NOT NULL DEFAULT '',
				mode TEXT NOT NULL CHECK (mode IN ('always','until-set-available','until-new-set-available')),
				current_sets TEXT NOT NULL DEFAULT '[]',
				PRIMARY KEY (tmdb_id, library_title, edition)
			) WITHOUT ROWID;
		`); err != nil {
			tx.Rollback()
			logAction.SetError("Failed to create new IgnoredItems table with edition column", "", map[string]any{"error": err.Error()})
			return *logAction.Error
		}
		if _, err = tx.ExecContext(ctx, `
			INSERT INTO IgnoredItems (tmdb_id, library_title, edition, mode, current_sets)
			SELECT tmdb_id, library_title, '', mode, current_sets
			FROM IgnoredItems_old;
		`); err != nil {
			tx.Rollback()
			logAction.SetError("Failed to copy data into new IgnoredItems table", "", map[string]any{"error": err.Error()})
			return *logAction.Error
		}
		if _, err = tx.ExecContext(ctx, `DROP TABLE IgnoredItems_old;`); err != nil {
			tx.Rollback()
			logAction.SetError("Failed to drop old IgnoredItems table", "", map[string]any{"error": err.Error()})
			return *logAction.Error
		}

		if err = tx.Commit(); err != nil {
			logAction.SetError("Failed to commit transaction for adding edition column", "", map[string]any{"error": err.Error()})
			return *logAction.Error
		}
	}

	logging.LOGGER.Info().Timestamp().Msg("Database migration v5.0 to v6.0 completed successfully")
	return Err
}
