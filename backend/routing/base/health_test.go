package routes_base

import (
	"aura/config"
	"aura/database"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthCheckStatus(t *testing.T) {
	previousLoaded, previousValid, previousFullyLoaded, previousStep := config.Loaded, config.Valid, config.AppFullyLoaded, config.AppLoadingStep
	previousClient := database.Client
	t.Cleanup(func() {
		config.Loaded, config.Valid, config.AppFullyLoaded, config.AppLoadingStep = previousLoaded, previousValid, previousFullyLoaded, previousStep
		database.Client = previousClient
	})
	// The database isn't open yet in any of these
	database.Client = nil

	tests := []struct {
		name                 string
		loaded, valid, ready bool
		wantStatus, wantStep string
	}{
		{name: "starting", loaded: false, valid: false, ready: false, wantStatus: "starting", wantStep: "Loading"},
		{name: "setup", loaded: true, valid: false, ready: false, wantStatus: "setup"},
		{name: "running", loaded: true, valid: true, ready: true, wantStatus: "ok"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config.Loaded, config.Valid, config.AppFullyLoaded, config.AppLoadingStep = tt.loaded, tt.valid, tt.ready, "Loading"

			rec := httptest.NewRecorder()
			HealthCheck(rec, httptest.NewRequest(http.MethodGet, "/api/health", nil))
			if rec.Code != http.StatusOK {
				t.Fatalf("status code = %d, want 200", rec.Code)
			}
			var body struct {
				Data healthCheckResponse `json:"data"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body.Data.Status != tt.wantStatus || body.Data.LoadingStep != tt.wantStep || body.Data.Database != "not_open" {
				t.Errorf("got %+v, want status %q, loading step %q and database not_open", body.Data, tt.wantStatus, tt.wantStep)
			}
		})
	}
}
