package routes_config

import (
	"aura/config"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestExportAppConfig(t *testing.T) {
	setupConfigSectionTest(t)

	recorder := httptest.NewRecorder()
	ExportAppConfig(recorder, httptest.NewRequest(http.MethodGet, "/api/config/export", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status %d: %s", recorder.Code, recorder.Body.String())
	}
	if got := recorder.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", got)
	}
	if !strings.Contains(recorder.Header().Get("Content-Disposition"), "aura-config-") {
		t.Errorf("Content-Disposition = %q", recorder.Header().Get("Content-Disposition"))
	}

	// Secrets are included unmasked, but not the User ID
	var exported config.Config
	if err := yaml.Unmarshal(recorder.Body.Bytes(), &exported); err != nil {
		t.Fatalf("export is not valid YAML: %v", err)
	}
	if exported.Auth.OIDC.ClientSecret != "client-secret" || exported.Auth.APIKeyHash != "$argon2id$hash" {
		t.Error("export should include the real secrets")
	}
	if exported.MediaServer.UserID != "" {
		t.Error("export should not include the User ID")
	}
	if config.Current.MediaServer.UserID != "admin-user-id" {
		t.Error("exporting should not change the current config")
	}
}

func TestExportAppConfigRedacted(t *testing.T) {
	setupConfigSectionTest(t)

	recorder := httptest.NewRecorder()
	ExportAppConfig(recorder, httptest.NewRequest(http.MethodGet, "/api/config/export?redacted=true", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status %d: %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Header().Get("Content-Disposition"), "aura-config-redacted-") {
		t.Errorf("Content-Disposition = %q", recorder.Header().Get("Content-Disposition"))
	}
	body := recorder.Body.String()
	if !strings.HasPrefix(body, config.RedactedConfigMarker) {
		t.Errorf("a redacted export should start with the marker, got %q", body[:min(len(body), 80)])
	}
	for _, secret := range []string{"client-secret", "$argon2id$", "discord.example.com", "id.example.com"} {
		if strings.Contains(body, secret) {
			t.Errorf("redacted export still has %q", secret)
		}
	}
	if config.Current.Auth.OIDC.ClientSecret != "client-secret" {
		t.Error("exporting should not change the current config")
	}
}

func TestImportAppConfigRejectsInvalidFiles(t *testing.T) {
	tests := []struct{ name, yamlText, wantError string }{
		{"empty", "  ", "The config file is empty"},
		{"not yaml", "Auth: [unclosed", "The config file could not be read"},
		{"redacted", config.RedactedConfigMarker + ", exported today\nAuth:\n  Enabled: false\n", "This is a redacted config for sharing"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := setupConfigSectionTest(t)
			body, _ := json.Marshal(importConfigRequest{YAML: test.yamlText})

			recorder := httptest.NewRecorder()
			ImportAppConfig(recorder, httptest.NewRequest(http.MethodPost, "/api/config/import", strings.NewReader(string(body))))

			if recorder.Code != http.StatusInternalServerError || !strings.Contains(recorder.Body.String(), test.wantError) {
				t.Errorf("status %d, body %s, want error %q", recorder.Code, recorder.Body.String(), test.wantError)
			}
			if store.saves != 0 {
				t.Error("an invalid file should not be saved")
			}
		})
	}
}
