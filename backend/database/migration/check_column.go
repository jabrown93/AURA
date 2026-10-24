package migration

import (
	"aura/database"
	"aura/logging"
	"context"
)

func checkColumnExists(ctx context.Context, tableName string, columnName string) (exists bool, Err logging.LogErrorInfo) {
	Err = logging.LogErrorInfo{}

	// Get DB connection
	conn, _, getDBConnErr := database.GetDBConnection(ctx)
	if getDBConnErr.Message != "" {
		return false, getDBConnErr
	}

	// Check if the column already exists to avoid duplicate column error
	checkColumnQuery := `PRAGMA table_info(` + tableName + `);`
	rows, err := conn.QueryContext(ctx, checkColumnQuery)
	if err != nil {
		Err = logging.LogErrorInfo{
			Message: "Failed to query " + tableName + " table info",
			Detail:  map[string]any{"error": err.Error()},
		}
		return false, Err
	}
	defer rows.Close()

	exists = false
	for rows.Next() {
		var cid int
		var name string
		var ctype string
		var notnull int
		var dfltValue interface{}
		var pk int
		err = rows.Scan(&cid, &name, &ctype, &notnull, &dfltValue, &pk)
		if err != nil {
			Err = logging.LogErrorInfo{
				Message: "Failed to scan " + tableName + " table info",
				Detail:  map[string]any{"error": err.Error()},
			}
			return false, Err
		}
		if name == columnName {
			exists = true
			break
		}
	}
	if err = rows.Err(); err != nil {
		Err = logging.LogErrorInfo{
			Message: "Failed to read " + tableName + " table info",
			Detail:  map[string]any{"error": err.Error()},
		}
		return false, Err
	}

	return exists, Err
}
