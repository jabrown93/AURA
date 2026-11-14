package database

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestPostgresConnectionErrorsHideCredentials(t *testing.T) {
	ctx := context.Background()
	for _, dsn := range []string{
		"postgres://user:do-not-leak@127.0.0.1:1/aura?connect_timeout=1",
		"password=do-not-leak invalid",
	} {
		db := &PostgresDB{dsn: dsn}
		_, _, info := db.GetDBConnection(ctx)
		if info.Message == "" {
			t.Fatal("invalid connection unexpectedly succeeded")
		}
		if strings.Contains(fmt.Sprint(info), "do-not-leak") {
			t.Fatal("connection error exposed credentials")
		}
	}
}

// TestPostgresSchema covers first initialization, persistence and incompatible-version refusal.
func TestPostgresSchema(t *testing.T) {
	ctx := context.Background()
	db := newPostgresTestDB(t)
	newDB, info := db.Init(ctx)
	if info.Message != "" {
		t.Fatal(info.Message)
	}
	t.Cleanup(func() { db.Close(ctx) })
	if !newDB {
		t.Fatal("test requires an empty PostgreSQL schema")
	}
	version, info := db.GetCurrentVersion(ctx)
	if info.Message != "" || version != LATEST_DB_VERSION {
		t.Fatalf("upstream schema version = %d, error = %s", version, info.Message)
	}
	secret, info := db.GetAuthTokenSecret(ctx)
	if info.Message != "" || secret == "" {
		t.Fatalf("auth secret missing: %s", info.Message)
	}
	again, info := db.GetAuthTokenSecret(ctx)
	if info.Message != "" || again != secret {
		t.Fatal("auth secret changed between reads")
	}
	if info = db.SaveConfigValues(ctx, map[string]string{"one": "first", "two": "second"}, true); info.Message != "" {
		t.Fatal(info.Message)
	}
	if info = db.SaveConfigValues(ctx, map[string]string{"one": "updated"}, true); info.Message != "" {
		t.Fatal(info.Message)
	}
	values, info := db.GetAllConfigValues(ctx)
	if info.Message != "" || len(values) != 1 || values["one"] != "updated" {
		t.Fatalf("config replacement failed: %s", info.Message)
	}
	if newDB, info = db.Init(ctx); info.Message != "" || newDB {
		t.Fatalf("schema reopening: new = %v, error = %s", newDB, info.Message)
	}
	if _, err := db.conn.ExecContext(ctx, "UPDATE aura_schema_migrations SET version = 999"); err != nil {
		t.Fatal(err)
	}
	if _, info = db.Init(ctx); info.Message == "" {
		t.Fatal("future PostgreSQL schema accepted")
	}
}
