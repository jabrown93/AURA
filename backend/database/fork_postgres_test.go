package database

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

// newPostgresTestDB isolates each test in its own schema, never deleting existing tables.
func newPostgresTestDB(t *testing.T) *PostgresDB {
	t.Helper()
	dsn := os.Getenv("AURA_TEST_POSTGRES_DSN")
	if dsn == "" {
		if os.Getenv("CI") == "true" {
			t.Fatal("AURA_TEST_POSTGRES_DSN is required in CI")
		}
		t.Skip("AURA_TEST_POSTGRES_DSN is not set")
	}
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatal("invalid PostgreSQL test DSN")
	}
	control := stdlib.OpenDB(*cfg)
	t.Cleanup(func() { control.Close() })
	var id [8]byte
	if _, err := rand.Read(id[:]); err != nil {
		t.Fatal(err)
	}
	schema := "aura_test_" + hex.EncodeToString(id[:])
	if _, err := control.ExecContext(context.Background(), "CREATE SCHEMA "+schema); err != nil {
		t.Fatal("could not create PostgreSQL test schema")
	}
	t.Cleanup(func() {
		if _, err := control.ExecContext(context.Background(), "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Error("could not remove PostgreSQL test schema")
		}
	})
	cfg.RuntimeParams["search_path"] = schema
	cfg.RuntimeParams["timezone"] = "UTC"
	db := &PostgresDB{conn: stdlib.OpenDB(*cfg)}
	t.Cleanup(func() { db.Close(context.Background()) })
	return db
}
