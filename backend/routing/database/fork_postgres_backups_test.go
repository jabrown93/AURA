package routes_db

import (
	"aura/database"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestPostgresBackupEndpoints never lists SQLite files or reports an unsupported backup as successful.
func TestPostgresBackupEndpoints(t *testing.T) {
	old := database.Client
	t.Cleanup(func() { database.Client = old })
	database.Client = &database.PostgresDB{}
	w := httptest.NewRecorder()
	GetDatabaseBackups(w, httptest.NewRequest(http.MethodGet, "/api/db/backups", nil))
	var response struct {
		Data struct {
			Supported bool                      `json:"supported"`
			Folder    string                    `json:"folder"`
			Backups   []database.DatabaseBackup `json:"backups"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil || w.Code != http.StatusOK || response.Data.Supported || response.Data.Folder != "" || len(response.Data.Backups) != 0 {
		t.Fatalf("unsupported backup listing: %d %s %v", w.Code, w.Body.String(), err)
	}
	w = httptest.NewRecorder()
	CreateDatabaseBackup(w, httptest.NewRequest(http.MethodPost, "/api/db/backups", nil))
	if w.Code != http.StatusNotImplemented {
		t.Fatalf("unsupported backup creation: %d %s", w.Code, w.Body.String())
	}
}
