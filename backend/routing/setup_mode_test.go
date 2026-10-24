package routing

import (
	"aura/config"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func serve(t *testing.T, handler http.Handler, method, path, setupCode, body string) (int, string) {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if setupCode != "" {
		req.Header.Set("X-Setup-Code", setupCode)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	return recorder.Code, recorder.Body.String()
}

func setTestConfig(t *testing.T, cfg config.Config, valid bool) {
	t.Helper()
	previous, previousLoaded, previousValid := config.Current, config.Loaded, config.Valid
	t.Cleanup(func() { config.Current, config.Loaded, config.Valid = previous, previousLoaded, previousValid })
	config.Current, config.Loaded, config.Valid = cfg, true, valid
	t.Setenv(config.DisableAuthEnv, "")
}

func TestSetupModeNeedsSetupCode(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Mediux.ApiToken = "mediux-secret-token"
	setTestConfig(t, cfg, false)
	router := NewRouter()

	// Without the setup code, the status is returned without the config
	code, body := serve(t, router, http.MethodGet, "/api/config", "", "")
	if code != http.StatusOK || !strings.Contains(body, `"setup_code_required":true`) || strings.Contains(body, "cret-token") {
		t.Errorf("GET /api/config without the setup code = %d %s", code, body)
	}

	// Setup routes are refused without the setup code
	for _, path := range []string{"/api/config", "/api/validate/mediaserver", "/api/onboarding/finalize"} {
		if code, _ := serve(t, router, http.MethodPost, path, "", "{}"); code != http.StatusForbidden {
			t.Errorf("POST %s without the setup code = %d, want 403", path, code)
		}
	}

	// Checking the setup code
	if code, body := serve(t, router, http.MethodPost, "/api/setup/verify", "", `{"code":"AAAA-AAAA"}`); code == http.StatusOK && strings.Contains(body, `"valid":true`) {
		t.Errorf("a wrong setup code was accepted: %s", body)
	}
	code, body = serve(t, router, http.MethodPost, "/api/setup/verify", "", `{"code":"`+config.SetupCode()+`"}`)
	var verify struct {
		Data struct {
			Valid bool `json:"valid"`
		} `json:"data"`
	}
	_ = json.Unmarshal([]byte(body), &verify)
	if code != http.StatusOK || !verify.Data.Valid {
		t.Errorf("POST /api/setup/verify with the setup code = %d %s", code, body)
	}

	// With the setup code, the masked config is returned
	code, body = serve(t, router, http.MethodGet, "/api/config", config.SetupCode(), "")
	if code != http.StatusOK || strings.Contains(body, `"setup_code_required":true`) || !strings.Contains(body, "***oken") {
		t.Errorf("GET /api/config with the setup code = %d %s", code, body)
	}
}

func TestSetupModeNeedsLoginWhenAuthEnabled(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Auth.Enabled = true
	cfg.Auth.Password = "$argon2id$hash"
	setTestConfig(t, cfg, false)
	router := NewRouter()

	// The setup code is not enough when a login is configured
	if code, _ := serve(t, router, http.MethodPost, "/api/config", config.SetupCode(), "{}"); code != http.StatusUnauthorized {
		t.Errorf("POST /api/config with auth enabled = %d, want 401", code)
	}
	if code, _ := serve(t, router, http.MethodGet, "/api/config", config.SetupCode(), ""); code != http.StatusUnauthorized {
		t.Errorf("GET /api/config with auth enabled = %d, want 401", code)
	}
	// The login methods stay public, so the login page works in setup mode
	if code, _ := serve(t, router, http.MethodGet, "/api/config/auth-methods", "", ""); code != http.StatusOK {
		t.Errorf("GET /api/config/auth-methods = %d, want 200", code)
	}
}

func TestSearchNeedsLogin(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Auth.Enabled = true
	cfg.Auth.Password = "$argon2id$hash"
	setTestConfig(t, cfg, true)
	router := NewRouter()

	if code, _ := serve(t, router, http.MethodGet, "/api/search?query=test", "", ""); code != http.StatusUnauthorized {
		t.Errorf("GET /api/search without a login = %d, want 401", code)
	}
}

func TestSetupModeConfigHasErrors(t *testing.T) {
	// A fresh install isn't set up yet, which is not an error
	setTestConfig(t, config.DefaultConfig(), false)
	router := NewRouter()
	_, body := serve(t, router, http.MethodGet, "/api/config", config.SetupCode(), "")
	if !strings.Contains(body, `"config_has_errors":false`) {
		t.Errorf("fresh install: want config_has_errors false, got %s", body)
	}

	// Once required settings are entered, an invalid config has errors
	cfg := config.DefaultConfig()
	cfg.MediaServer.URL = "http://plex:32400"
	setTestConfig(t, cfg, false)
	router = NewRouter()
	_, body = serve(t, router, http.MethodGet, "/api/config", config.SetupCode(), "")
	if !strings.Contains(body, `"config_has_errors":true`) {
		t.Errorf("entered but invalid config: want config_has_errors true, got %s", body)
	}
}
