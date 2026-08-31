package migration

import (
	"aura/database"
	"strings"
	"testing"
)

func TestCheckSchemaSupported(t *testing.T) {
	tests := []struct {
		name           string
		currentVersion int
		latestVersion  int
		wantErr        bool
	}{
		{"fresh database", 0, 6, false},
		{"needs migration", 4, 6, false},
		{"up to date", 6, 6, false},
		{"one version ahead", 7, 6, true},
		{"several versions ahead", 12, 6, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := checkSchemaSupported(tt.currentVersion, tt.latestVersion)
			if gotErr := got.Message != ""; gotErr != tt.wantErr {
				t.Fatalf("checkSchemaSupported(%d, %d) error = %v, want error = %v (message %q)",
					tt.currentVersion, tt.latestVersion, gotErr, tt.wantErr, got.Message)
			}
			if !tt.wantErr {
				return
			}
			if got.Detail["database_version"] != tt.currentVersion {
				t.Errorf("detail database_version = %v, want %d", got.Detail["database_version"], tt.currentVersion)
			}
			// The operator's only recovery path is the backup written before the
			// migration that took the DB past this binary, so name it.
			if !strings.Contains(got.Help, "_backup_v6_to_v7_") {
				t.Errorf("help does not name the recovery backup file: %q", got.Help)
			}
		})
	}
}

// The guard is only correct if every schema version the binary can produce is
// accepted; a LATEST_DB_VERSION bump without a matching migration case would
// otherwise start refusing its own databases.
func TestCheckSchemaSupportedAcceptsLatestVersion(t *testing.T) {
	if err := checkSchemaSupported(database.LATEST_DB_VERSION, database.LATEST_DB_VERSION); err.Message != "" {
		t.Fatalf("current binary rejects its own schema version %d: %s", database.LATEST_DB_VERSION, err.Message)
	}
}
