package database

import (
	"aura/logging"
	"context"
)

// CreatorSetCount is a creator and how many of their sets are saved
type CreatorSetCount struct {
	Username string `json:"username"`
	Sets     int    `json:"sets"`
}

// GetSavedSetCreators returns the creators of the saved sets, the most saved sets first
func (s *SQliteDB) GetSavedSetCreators(ctx context.Context) ([]CreatorSetCount, logging.LogErrorInfo) {
	creators := []CreatorSetCount{}
	rows, err := s.conn.QueryContext(ctx, `
		SELECT ps.user, COUNT(DISTINCT ps.id) AS sets
		FROM PosterSets ps
		JOIN SavedItems si ON si.poster_set_id = ps.id
		WHERE ps.user != ''
		GROUP BY ps.user
		ORDER BY sets DESC, ps.user COLLATE NOCASE;
	`)
	if err != nil {
		return creators, dbError(ctx, "Getting the creators of the saved sets", "Failed to get the creators of the saved sets", err)
	}
	defer rows.Close()
	for rows.Next() {
		var creator CreatorSetCount
		if err := rows.Scan(&creator.Username, &creator.Sets); err != nil {
			return creators, dbError(ctx, "Getting the creators of the saved sets", "Failed to read a creator of the saved sets", err)
		}
		creators = append(creators, creator)
	}
	if err := rows.Err(); err != nil {
		return creators, dbError(ctx, "Getting the creators of the saved sets", "Failed to read the creators of the saved sets", err)
	}
	return creators, logging.LogErrorInfo{}
}
