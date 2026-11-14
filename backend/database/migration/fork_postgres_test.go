package migration

import (
	"aura/database"
	"testing"
)

// TestPostgresSkipsSQLiteMigrations verifies no SQLite query is issued even without a live PG connection.
func TestPostgresSkipsSQLiteMigrations(t *testing.T) {
	old := database.Client
	t.Cleanup(func() { database.Client = old })
	database.Client = &database.PostgresDB{}
	count, info := RunMigrations()
	if count != 0 || info.Message != "" {
		t.Fatalf("PostgreSQL entered SQLite migrations: %d %+v", count, info)
	}
}
