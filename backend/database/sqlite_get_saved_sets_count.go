package database

import (
	"aura/logging"
	"context"
)

func (s *SQliteDB) GetCountSavedSets(ctx context.Context) (count int, logErr logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Getting Count of Saved Sets", logging.LevelInfo)
	defer logAction.Complete()

	count = 0

	// Make the query to get the count of saved sets
	// Unique tmdb_id and library_title combinations
	query := `
        SELECT COUNT(*) FROM (
            SELECT tmdb_id, library_title, edition
            FROM SavedItems
            GROUP BY tmdb_id, library_title, edition
        ) AS unique_sets;
    `
	row := s.conn.QueryRowContext(ctx, query)
	if err := row.Scan(&count); err != nil {
		logAction.SetError("Failed to scan count of saved sets", "", map[string]any{"error": err.Error(), "query": query})
		return count, *logAction.Error
	}

	return count, logErr
}
