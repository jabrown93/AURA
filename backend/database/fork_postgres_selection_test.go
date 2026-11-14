package database

import (
	"strings"
	"testing"
)

// TestDatabaseSelection keeps SQLite the default and rejects partial PostgreSQL configuration.
func TestDatabaseSelection(t *testing.T) {
	for _, test := range []struct {
		name, driver, dsn string
		postgres, invalid bool
	}{
		{name: "default"},
		{name: "sqlite", driver: "sqlite", dsn: "ignored"},
		{name: "postgres", driver: "postgres", dsn: "postgres://private-user:private-password@localhost/aura", postgres: true},
		{name: "missing DSN", driver: "postgres", invalid: true},
		{name: "invalid driver", driver: "unknown", invalid: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("AURA_DB_DRIVER", test.driver)
			t.Setenv("AURA_POSTGRES_DSN", test.dsn)
			db, info := NewDatabaseClient()
			if (info.Message != "") != test.invalid || strings.Contains(info.Message, "private-") {
				t.Fatalf("selection: %T %+v", db, info)
			}
			if test.invalid {
				if db != nil {
					t.Fatalf("invalid configuration returned %T", db)
				}
				return
			}
			_, postgres := db.(*PostgresDB)
			if postgres != test.postgres {
				t.Fatalf("selected %T", db)
			}
			if !postgres {
				if _, sqlite := db.(*SQliteDB); !sqlite {
					t.Fatalf("selected %T", db)
				}
			}
		})
	}
}
