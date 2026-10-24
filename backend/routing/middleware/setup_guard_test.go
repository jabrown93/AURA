package middleware

import (
	"aura/config"
	"aura/utils/httpx"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSetupGuard(t *testing.T) {
	previous := config.Current
	t.Cleanup(func() { config.Current = previous })
	config.Current.Auth.Enabled = false
	t.Setenv(config.DisableAuthEnv, "")

	var codeVerified bool
	handler := SetupGuard(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		codeVerified = httpx.SetupCodeVerified(r.Context())
		w.WriteHeader(http.StatusOK)
	}))
	request := func(method, path, code string) int {
		req := httptest.NewRequest(method, path, nil)
		if code != "" {
			req.Header.Set(SetupCodeHeader, code)
		}
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, req)
		return recorder.Code
	}

	// Without a login, setup routes need the setup code
	if got := request(http.MethodPost, "/api/config", ""); got != http.StatusForbidden {
		t.Errorf("POST /api/config without the setup code = %d, want 403", got)
	}
	if got := request(http.MethodPost, "/api/config", "WRONG-CODE"); got != http.StatusForbidden {
		t.Errorf("POST /api/config with a wrong setup code = %d, want 403", got)
	}
	if got := request(http.MethodPost, "/api/config", config.SetupCode()); got != http.StatusOK || !codeVerified {
		t.Errorf("POST /api/config with the setup code = %d (verified %v), want 200", got, codeVerified)
	}

	// The status and the code check are always allowed
	if got := request(http.MethodGet, "/api/config", ""); got != http.StatusOK || codeVerified {
		t.Errorf("GET /api/config without the setup code = %d (verified %v), want 200 without the config", got, codeVerified)
	}
	if got := request(http.MethodPost, "/api/setup/verify", ""); got != http.StatusOK {
		t.Errorf("POST /api/setup/verify = %d, want 200", got)
	}

	// With auth enabled, setup routes need a login. The setup code is not enough
	config.Current.Auth.Enabled = true
	if got := request(http.MethodPost, "/api/config", config.SetupCode()); got != http.StatusUnauthorized {
		t.Errorf("POST /api/config with auth enabled = %d, want 401", got)
	}
	if got := request(http.MethodPost, "/api/login", ""); got != http.StatusOK {
		t.Errorf("POST /api/login with auth enabled = %d, want 200 (public)", got)
	}
}
