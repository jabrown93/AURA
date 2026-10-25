package database

import (
	"aura/logging"
	"context"
)

func (s *SQliteDB) GetAllUniqueUsers(ctx context.Context) (users []string, logErr logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Getting All Unique Users from Saved Sets", logging.LevelInfo)
	defer logAction.Complete()

	users = []string{}

	// Make the query to get unique users
	query := `
	SELECT DISTINCT user
	FROM PosterSets;
	`
	rows, err := s.conn.QueryContext(ctx, query)
	if err != nil {
		logAction.SetError("Failed to query unique users", "", map[string]any{"error": err.Error(), "query": query})
		return users, *logAction.Error
	}
	defer rows.Close()

	// Iterate through the rows and collect unique users
	for rows.Next() {
		var user string
		if err := rows.Scan(&user); err != nil {
			logAction.SetError("Failed to scan unique user row", "", map[string]any{"error": err.Error()})
			return users, *logAction.Error
		}
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		logAction.SetError("Failed while iterating over unique user rows", "", map[string]any{"error": err.Error()})
		return users, *logAction.Error
	}

	return users, logErr
}
