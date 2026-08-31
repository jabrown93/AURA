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
	mediaServerValid := config.MediaServerValid
	mediaServerName := config.MediaServerName
	mediuxValid := config.MediuxValid
	appLoadingStep := config.AppLoadingStep
	mediuxAPIURL := mediux.MediuxApiURL
	t.Cleanup(func() {
		config.Current = current
		config.MediaServerValid = mediaServerValid
		config.MediaServerName = mediaServerName
		config.MediuxValid = mediuxValid
		config.AppLoadingStep = appLoadingStep
		mediux.MediuxApiURL = mediuxAPIURL
	})
}

func TestRunPreFlightAllowsHealthyMediaServerWhenMediUXValidationFails(t *testing.T) {
	preservePreFlightState(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/users/me" {
			http.Error(w, "MediUX unavailable", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"MediaContainer":{"friendlyName":"Plex","version":"1.0.0"}}`))
	}))
	defer server.Close()

	mediux.MediuxApiURL = server.URL
	config.Current.MediaServer = config.Config_MediaServer{Type: "Plex", URL: server.URL, ApiToken: "plex-token"}
	config.Current.Mediux = config.Config_Mediux{ApiToken: "mediux-token"}
	config.MediaServerValid = false
	config.MediuxValid = true

	if !runPreFlight() {
		t.Fatal("runPreFlight() = false, want true when only MediUX validation fails")
	}
	if !config.MediaServerValid {
		t.Error("config.MediaServerValid = false, want true")
	}
	if config.MediuxValid {
		t.Error("config.MediuxValid = true, want false")
	}
}

func TestRunPreFlightMarksValidMediUXToken(t *testing.T) {
	preservePreFlightState(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"MediaContainer":{"friendlyName":"Plex","version":"1.0.0"}}`))
	}))
	defer server.Close()

	mediux.MediuxApiURL = server.URL
	config.Current.MediaServer = config.Config_MediaServer{Type: "Plex", URL: server.URL, ApiToken: "plex-token"}
	config.Current.Mediux = config.Config_Mediux{ApiToken: "mediux-token"}
	config.MediaServerValid = false
	config.MediuxValid = false

	if !runPreFlight() {
		t.Fatal("runPreFlight() = false, want true when both services validate")
	}
	if !config.MediuxValid {
		t.Error("config.MediuxValid = false, want true")
	}
}

func TestRunPreFlightRejectsInvalidMediaServer(t *testing.T) {
	preservePreFlightState(t)
	config.Current.MediaServer = config.Config_MediaServer{Type: "unsupported"}
	config.MediaServerValid = true

	if runPreFlight() {
		t.Fatal("runPreFlight() = true, want false when media server is invalid")
	}
	if config.MediaServerValid {
		t.Error("config.MediaServerValid = true, want false")
	}
}
