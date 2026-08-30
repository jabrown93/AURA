package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// The image exemption exists so <img> tags can load without an Authorization header.
// It must not also exempt the destructive DELETE on the same prefix.
func TestAuthenticatorImageExemptionIsGetOnly(t *testing.T) {
	tests := []struct {
		name        string
		method      string
		path        string
		wantAllowed bool
	}{
		{"image GET is exempt", http.MethodGet, "/api/images/media/item", true},
		{"image DELETE is not exempt", http.MethodDelete, "/api/images/temp", false},
		{"login is exempt", http.MethodPost, "/api/login", true},
		{"sonarr webhook is exempt", http.MethodPost, "/api/sonarr/webhook", true},
		{"search is not exempt", http.MethodGet, "/api/search", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withAuthEnabled(t, true)

			reached := false
			handler := Authenticator(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				reached = true
			}))

			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest(tt.method, tt.path, nil))

			if reached != tt.wantAllowed {
				t.Errorf("%s %s: reached handler = %v, want %v", tt.method, tt.path, reached, tt.wantAllowed)
			}
			if !tt.wantAllowed && recorder.Code != http.StatusUnauthorized {
				t.Errorf("%s %s: status = %d, want %d", tt.method, tt.path, recorder.Code, http.StatusUnauthorized)
			}
		})
	}
}
