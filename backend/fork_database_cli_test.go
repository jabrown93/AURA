package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

// TestDatabaseImportCommand refuses ambiguous or unsafe invocations before server startup.
func TestDatabaseImportCommand(t *testing.T) {
	for _, test := range []struct {
		name    string
		args    []string
		driver  string
		handled bool
	}{
		{"server", nil, "", false},
		{"unknown", []string{"db", "typo"}, "postgres", true},
		{"missing-confirmation", []string{"db", "import-sqlite", "--source", "source.db"}, "postgres", true},
		{"missing-dsn", []string{"db", "import-sqlite", "--source", "source.db", "--confirm-app-stopped"}, "postgres", true},
		{"wrong-driver", []string{"db", "import-sqlite", "--source", "source.db", "--confirm-app-stopped"}, "sqlite", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("AURA_DB_DRIVER", test.driver)
			t.Setenv("AURA_POSTGRES_DSN", "")
			var out, stderr bytes.Buffer
			handled, code := forkDatabaseCommand(context.Background(), test.args, &out, &stderr)
			if handled != test.handled || (handled && code == 0) {
				t.Fatalf("handled=%v code=%d stderr=%s", handled, code, stderr.String())
			}
			if strings.Contains(out.String(), "source.db") {
				t.Fatal("unexpected source content output")
			}
		})
	}
}
