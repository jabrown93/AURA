package routes_config

import (
	"aura/config"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetAppConfigStatusExposesMediuxValidityAndSanitizesConfig(t *testing.T) {
	current := config.Current
	mediuxValid := config.MediuxValid
	t.Cleanup(func() {
		config.Current = current
		config.MediuxValid = mediuxValid
	})

	config.Current.Mediux.ApiToken = "secret-mediux-token"
	config.MediuxValid = false

	recorder := httptest.NewRecorder()
	GetAppConfigStatus(recorder, httptest.NewRequest(http.MethodGet, "/api/config", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", recorder.Code, http.StatusOK)
	}
	if strings.Contains(recorder.Body.String(), "secret-mediux-token") {
		t.Fatal("status response exposed MediUX token")
	}

	var response struct {
		Data struct {
			Status struct {
				MediuxValid *bool `json:"mediux_valid"`
			} `json:"status"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Data.Status.MediuxValid == nil {
		t.Fatal("mediux_valid missing from status response")
	}
	if *response.Data.Status.MediuxValid {
		t.Fatal("mediux_valid = true, want false")
	}
}
