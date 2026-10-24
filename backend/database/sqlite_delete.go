package database

import (
	"aura/logging"
	"context"
	"database/sql"
	"fmt"
)

// unlinkPosterSetFromMediaItemTx:
// - deletes SavedItems link for (tmdb_id, library_title, poster_set_id)
// - deletes ImageFiles rows for (poster_set_id, item_tmdb_id)
// - if PosterSet becomes orphaned (no SavedItems references), deletes:
//   - ALL ImageFiles for that poster_set_id
//   - PosterSets row
//
// Returns: (linksDeleted, itemImagesDeleted, orphanSetDeleted, orphanImagesDeleted)
func unlinkPosterSetFromMediaItemTx(
	ctx context.Context,
	tx *sql.Tx,
	tmdbID, libraryTitle, edition, setID string,
) (int64, int64, bool, int64, logging.LogErrorInfo) {
	// Lookup PosterSets PK by set_id
	var posterSetPK int64
	err := tx.QueryRowContext(ctx, `
        SELECT id
        FROM PosterSets
        WHERE set_id = ?
        LIMIT 1;
    `, setID).Scan(&posterSetPK)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, 0, false, 0, logging.LogErrorInfo{}
		}
		return 0, 0, false, 0, logging.LogErrorInfo{
			Message: "Failed to find PosterSet by set_id",
			Detail:  map[string]any{"error": err.Error(), "set_id": setID},
		}
	}

	// 1) Unlink this media item
	res, err := tx.ExecContext(ctx, `
        DELETE FROM SavedItems
        WHERE tmdb_id = ?
          AND library_title = ?
          AND edition = ?
          AND poster_set_id = ?;
    `, tmdbID, libraryTitle, edition, posterSetPK)
	if err != nil {
		return 0, 0, false, 0, logging.LogErrorInfo{
			Message: "Failed to delete SavedItems link for media item",
			Detail:  map[string]any{"error": err.Error(), "tmdb_id": tmdbID, "library_title": libraryTitle, "edition": edition, "set_id": setID},
		}
	}
	linksDeleted, _ := res.RowsAffected()

	// 2) Always delete item-scoped images for this set + item (safe even if set is shared)
	res, err = tx.ExecContext(ctx, `
        DELETE FROM ImageFiles
        WHERE poster_set_id = ?
          AND item_tmdb_id = ?;
    `, posterSetPK, tmdbID)
	if err != nil {
		return linksDeleted, 0, false, 0, logging.LogErrorInfo{
			Message: "Failed to delete ImageFiles for unlinked media item",
			Detail: map[string]any{
				"error":         err.Error(),
				"poster_set_id": posterSetPK,
				"tmdb_id":       tmdbID,
				"set_id":        setID,
			},
		}
	}
	itemImagesDeleted, _ := res.RowsAffected()

	// 3) If nobody references this set anymore, delete the set and *all* its images too
	var remaining int
	if err := tx.QueryRowContext(ctx, `
        SELECT COUNT(*)
        FROM SavedItems
        WHERE poster_set_id = ?;
    `, posterSetPK).Scan(&remaining); err != nil {
		return linksDeleted, itemImagesDeleted, false, 0, logging.LogErrorInfo{
			Message: "Failed to check remaining references",
			Detail:  map[string]any{"error": err.Error(), "poster_set_id": posterSetPK, "set_id": setID},
		}
	}

	var orphanImagesDeleted int64
	var orphanSetDeleted bool

	if remaining == 0 {
		// Delete ALL images for the set (across any items)
		res, err := tx.ExecContext(ctx, `DELETE FROM ImageFiles WHERE poster_set_id = ?;`, posterSetPK)
		if err != nil {
			return linksDeleted, itemImagesDeleted, false, 0, logging.LogErrorInfo{
				Message: "Failed to delete ImageFiles for orphaned poster set",
				Detail:  map[string]any{"error": err.Error(), "poster_set_id": posterSetPK, "set_id": setID},
			}
		}
		orphanImagesDeleted, _ = res.RowsAffected()

		// Delete the set itself
		if _, err := tx.ExecContext(ctx, `DELETE FROM PosterSets WHERE id = ?;`, posterSetPK); err != nil {
			return linksDeleted, itemImagesDeleted, false, orphanImagesDeleted, logging.LogErrorInfo{
				Message: "Failed to delete orphaned PosterSet",
				Detail:  map[string]any{"error": err.Error(), "poster_set_id": posterSetPK, "set_id": setID},
			}
		}
		orphanSetDeleted = true
	}

	return linksDeleted, itemImagesDeleted, orphanSetDeleted, orphanImagesDeleted, logging.LogErrorInfo{}
}

func (s *SQliteDB) DeletePosterSetForMediaItem(ctx context.Context, tmdbID, libraryTitle, edition, setID string) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(
		ctx,
		fmt.Sprintf("Unlinking PosterSet (set_id=%s) from media item (%s | %s | %s)", setID, tmdbID, libraryTitle, edition),
		logging.LevelInfo,
	)
	defer logAction.Complete()

	Err = logging.LogErrorInfo{}
	if s.conn == nil {
		logAction.SetError("Database connection is nil", "", map[string]any{"set_id": setID})
		return *logAction.Error
	}

	tx, err := s.conn.BeginTx(ctx, nil)
	if err != nil {
		logAction.SetError("Failed to start transaction", "", map[string]any{"error": err.Error(), "set_id": setID})
		return *logAction.Error
	}
	defer func() { _ = tx.Rollback() }()

	linksDeleted, itemImagesDeleted, orphanSetDeleted, orphanImagesDeleted, errInfo :=
		unlinkPosterSetFromMediaItemTx(ctx, tx, tmdbID, libraryTitle, edition, setID)
	if errInfo.Message != "" {
		logAction.SetError(errInfo.Message, "", errInfo.Detail)
		return *logAction.Error
	}

	logAction.AppendResult("action", "delete_set_for_media_item")
	logAction.AppendResult("saveditems_deleted", linksDeleted)
	logAction.AppendResult("item_images_deleted", itemImagesDeleted)
	logAction.AppendResult("orphan_set_deleted", orphanSetDeleted)
	logAction.AppendResult("orphan_images_deleted", orphanImagesDeleted)

	if err := tx.Commit(); err != nil {
		logAction.SetError("Failed to commit transaction", "", map[string]any{"error": err.Error()})
		return *logAction.Error
	}

	return Err
}

// DeleteAllPosterSetsForMediaItem unlinks *all* sets for a given media item.
// It also deletes item-scoped images, and deletes any sets that become orphaned (with all their images).
func (s *SQliteDB) DeleteAllPosterSetsForMediaItem(ctx context.Context, tmdbID, libraryTitle, edition string) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(
		ctx,
		fmt.Sprintf("Unlinking ALL PosterSets from media item (%s | %s | %s)", tmdbID, libraryTitle, edition),
		logging.LevelInfo,
	)
	defer logAction.Complete()

	Err = logging.LogErrorInfo{}
	if s.conn == nil {
		logAction.SetError("Database connection is nil", "", map[string]any{"tmdb_id": tmdbID, "library_title": libraryTitle})
		return *logAction.Error
	}

	tx, err := s.conn.BeginTx(ctx, nil)
	if err != nil {
		logAction.SetError("Failed to start transaction", "", map[string]any{"error": err.Error()})
		return *logAction.Error
	}
	defer func() { _ = tx.Rollback() }()

	// Pull all set_ids linked to this media item
	rows, err := tx.QueryContext(ctx, `
        SELECT ps.set_id
        FROM SavedItems si
        JOIN PosterSets ps ON ps.id = si.poster_set_id
        WHERE si.tmdb_id = ?
          AND si.library_title = ?
          AND si.edition = ?;
    `, tmdbID, libraryTitle, edition)
	if err != nil {
		logAction.SetError("Failed to list poster sets for media item", "", map[string]any{"error": err.Error()})
		return *logAction.Error
	}
	defer func() { _ = rows.Close() }()

	var setIDs []string
	for rows.Next() {
		var setID string
		if err := rows.Scan(&setID); err != nil {
			logAction.SetError("Failed to scan set_id", "", map[string]any{"error": err.Error()})
			return *logAction.Error
		}
		setIDs = append(setIDs, setID)
	}
	if err := rows.Err(); err != nil {
		logAction.SetError("Rows iteration failed", "", map[string]any{"error": err.Error()})
		return *logAction.Error
	}

	var totalLinksDeleted int64
	var totalItemImagesDeleted int64
	var totalOrphanSetsDeleted int64
	var totalOrphanImagesDeleted int64

	for _, setID := range setIDs {
		linksDeleted, itemImagesDeleted, orphanSetDeleted, orphanImagesDeleted, errInfo :=
			unlinkPosterSetFromMediaItemTx(ctx, tx, tmdbID, libraryTitle, edition, setID)
		if errInfo.Message != "" {
			logAction.SetError(errInfo.Message, "", errInfo.Detail)
			return *logAction.Error
		}

		totalLinksDeleted += linksDeleted
		totalItemImagesDeleted += itemImagesDeleted
		if orphanSetDeleted {
			totalOrphanSetsDeleted++
		}
		totalOrphanImagesDeleted += orphanImagesDeleted
	}

	logAction.AppendResult("action", "delete_all_sets_for_media_item")
	logAction.AppendResult("sets_found", len(setIDs))
	logAction.AppendResult("saveditems_deleted", totalLinksDeleted)
	logAction.AppendResult("item_images_deleted", totalItemImagesDeleted)
	logAction.AppendResult("orphan_sets_deleted", totalOrphanSetsDeleted)
	logAction.AppendResult("orphan_images_deleted", totalOrphanImagesDeleted)

	if err := tx.Commit(); err != nil {
		logAction.SetError("Failed to commit transaction", "", map[string]any{"error": err.Error()})
		return *logAction.Error
	}

	return Err
}

func (s *SQliteDB) DeleteMediaItemAndIgnoredStatus(ctx context.Context, tmdbID, libraryTitle, edition string) logging.LogErrorInfo {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Deleting MediaItem and Ignored status", logging.LevelInfo)
	defer logAction.Complete()

	tx, err := s.conn.BeginTx(ctx, nil)
	if err != nil {
		logAction.SetError("Failed to start transaction", "", map[string]any{"error": err.Error()})
		return *logAction.Error
	}
	defer func() { _ = tx.Rollback() }()

	// Delete from IgnoredItems
	_, err = tx.ExecContext(ctx, `
        DELETE FROM IgnoredItems WHERE tmdb_id = ? AND library_title = ? AND edition = ?
    `, tmdbID, libraryTitle, edition)
	if err != nil {
		logAction.SetError("Failed to delete from IgnoredItems", "", map[string]any{"error": err.Error()})
		return *logAction.Error
	}

	// Delete from MediaItems (cascades to related tables)
	_, err = tx.ExecContext(ctx, `
        DELETE FROM MediaItems WHERE tmdb_id = ? AND library_title = ? AND edition = ?
    `, tmdbID, libraryTitle, edition)
	if err != nil {
		logAction.SetError("Failed to delete from MediaItems", "", map[string]any{"error": err.Error()})
		return *logAction.Error
	}

	if err := tx.Commit(); err != nil {
		logAction.SetError("Failed to commit transaction", "", map[string]any{"error": err.Error()})
		return *logAction.Error
	}

	return logging.LogErrorInfo{}
}
