package main

import (
	"aura/config"
	"aura/mediux"
	"net/http"
	"net/http/httptest"
	"testing"
)

func preservePreFlightState(t *testing.T) {
	t.Helper()
	current := config.Current
	loaded := config.Loaded
	valid := config.Valid
	mediaServerValid := config.MediaServerValid
	mediaServerReachable := config.MediaServerReachable
	mediaServerName := config.MediaServerName
	mediuxValid := config.MediuxValid
	mediuxReachable := config.MediuxReachable
	appLoadingStep := config.AppLoadingStep
	mediuxAPIURL := mediux.MediuxApiURL
	t.Cleanup(func() {
		config.Current = current
		config.Loaded = loaded
		config.Valid = valid
		config.MediaServerValid = mediaServerValid
		config.MediaServerReachable = mediaServerReachable
		config.MediaServerName = mediaServerName
		config.MediuxValid = mediuxValid
		config.MediuxReachable = mediuxReachable
		config.AppLoadingStep = appLoadingStep
		mediux.MediuxApiURL = mediuxAPIURL
	})
}

// TestRunPreFlightSeparatesMediuxOutageFromRejectedToken pins the distinction the
// status response depends on: MediUX refusing the token is a config problem that
// must fail preflight and send the user to onboarding, while MediUX not
// answering is an outage the app starts through in a degraded state.
func TestRunPreFlightSeparatesMediuxOutageFromRejectedToken(t *testing.T) {
	tests := []struct {
		name                string
		mediuxStatus        int
		wantSuccess         bool
		wantMediuxValid     bool
		wantMediuxReachable bool
		wantNeedsSetup      bool
	}{
		{
			name:                "both healthy",
			mediuxStatus:        http.StatusOK,
			wantSuccess:         true,
			wantMediuxValid:     true,
			wantMediuxReachable: true,
			wantNeedsSetup:      false,
		},
		{
			name:                "mediux unreachable starts degraded",
			mediuxStatus:        http.StatusServiceUnavailable,
			wantSuccess:         true,
			wantMediuxValid:     false,
			wantMediuxReachable: false,
			wantNeedsSetup:      false,
		},
		{
			name:                "mediux rejects token",
			mediuxStatus:        http.StatusUnauthorized,
			wantSuccess:         false,
			wantMediuxValid:     false,
			wantMediuxReachable: true,
			wantNeedsSetup:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			preservePreFlightState(t)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/users/me" {
					if tt.mediuxStatus != http.StatusOK {
						http.Error(w, "MediUX rejected the request", tt.mediuxStatus)
						return
					}
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write([]byte(`{}`))
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"MediaContainer":{"friendlyName":"Plex","version":"1.0.0"}}`))
			}))
			defer server.Close()

			mediux.MediuxApiURL = server.URL
			config.Current.MediaServer = config.Config_MediaServer{Type: "Plex", URL: server.URL, ApiToken: "plex-token"}
			config.Current.Mediux = config.Config_Mediux{ApiToken: "mediux-token"}
			config.Loaded = true
			config.Valid = true
			config.MediaServerValid = false
			config.MediaServerReachable = false
			config.MediuxValid = false
			config.MediuxReachable = false

			gotSuccess := runPreFlight()

			if gotSuccess != tt.wantSuccess {
				t.Errorf("runPreFlight() = %v, want %v", gotSuccess, tt.wantSuccess)
			}
			if !config.MediaServerValid {
				t.Error("config.MediaServerValid = false, want true")
			}
			if !config.MediaServerReachable {
				t.Error("config.MediaServerReachable = false, want true")
			}
			if config.MediuxValid != tt.wantMediuxValid {
				t.Errorf("config.MediuxValid = %v, want %v", config.MediuxValid, tt.wantMediuxValid)
			}
			if config.MediuxReachable != tt.wantMediuxReachable {
				t.Errorf("config.MediuxReachable = %v, want %v", config.MediuxReachable, tt.wantMediuxReachable)
			}

			// Mirrors main.go: a failed preflight marks the config invalid and leaves
			// the app on onboarding-only routes.
			if !gotSuccess {
				config.Valid = false
			}
			if config.NeedsSetup() != tt.wantNeedsSetup {
				t.Errorf("config.NeedsSetup() = %v, want %v", config.NeedsSetup(), tt.wantNeedsSetup)
			}
		})
	}
}

func TestRunPreFlightRejectsInvalidMediaServer(t *testing.T) {
	preservePreFlightState(t)
	config.Current.MediaServer = config.Config_MediaServer{Type: "unsupported"}
	config.MediaServerValid = true
	config.MediaServerReachable = true

	if runPreFlight() {
		t.Fatal("runPreFlight() = true, want false when media server is invalid")
	}
	if config.MediaServerValid {
		t.Error("config.MediaServerValid = true, want false")
	}
	if config.MediaServerReachable {
		t.Error("config.MediaServerReachable = true, want false")
	}
}
