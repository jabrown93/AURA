package middleware

import (
	"aura/config"
	routes_auth "aura/routing/auth"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCSRFProtect(t *testing.T) {
	tests := []struct {
		name        string
		authEnabled bool
		method      string
		host        string
		headers     map[string]string
		withCookie  bool
		wantAllowed bool
	}{
		{
			name:        "auth disabled lets everything through",
			method:      http.MethodPost,
			host:        "aura.example.com",
			withCookie:  true,
			wantAllowed: true,
		},
		{
			name:        "safe method is never blocked",
			authEnabled: true,
			method:      http.MethodGet,
			host:        "aura.example.com",
			withCookie:  true,
			wantAllowed: true,
		},
		{
			name:        "bearer client without session cookie is exempt",
			authEnabled: true,
			method:      http.MethodPost,
			host:        "aura.example.com",
			wantAllowed: true,
		},
		{
			name:        "same origin cookie request is allowed",
			authEnabled: true,
			method:      http.MethodPost,
			host:        "aura.example.com",
			headers:     map[string]string{"Origin": "https://aura.example.com"},
			withCookie:  true,
			wantAllowed: true,
		},
		{
			name:        "origin matching forwarded host is allowed",
			authEnabled: true,
			method:      http.MethodPost,
			host:        "aura.internal:8888",
			headers:     map[string]string{"Origin": "https://aura.example.com", "X-Forwarded-Host": "aura.example.com"},
			withCookie:  true,
			wantAllowed: true,
		},
		{
			name:        "port mismatch is tolerated",
			authEnabled: true,
			method:      http.MethodPost,
			host:        "aura.example.com:8888",
			headers:     map[string]string{"Origin": "https://aura.example.com"},
			withCookie:  true,
			wantAllowed: true,
		},
		{
			name:        "referer is used when origin is absent",
			authEnabled: true,
			method:      http.MethodPost,
			host:        "aura.example.com",
			headers:     map[string]string{"Referer": "https://aura.example.com/settings"},
			withCookie:  true,
			wantAllowed: true,
		},
		{
			name:        "cross origin cookie request is rejected",
			authEnabled: true,
			method:      http.MethodPost,
			host:        "aura.example.com",
			headers:     map[string]string{"Origin": "https://evil.example.net"},
			withCookie:  true,
			wantAllowed: false,
		},
		{
			name:        "opaque origin is rejected",
			authEnabled: true,
			method:      http.MethodPost,
			host:        "aura.example.com",
			headers:     map[string]string{"Origin": "null"},
			withCookie:  true,
			wantAllowed: false,
		},
		{
			name:        "cookie request without origin or referer is rejected",
			authEnabled: true,
			method:      http.MethodPost,
			host:        "aura.example.com",
			withCookie:  true,
			wantAllowed: false,
		},
		{
			name:        "subdomain of allowed host is rejected",
			authEnabled: true,
			method:      http.MethodPost,
			host:        "aura.example.com",
			headers:     map[string]string{"Origin": "https://evil.aura.example.com"},
			withCookie:  true,
			wantAllowed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			previous := config.Current.Auth.Enabled
			config.Current.Auth.Enabled = tt.authEnabled
			t.Cleanup(func() { config.Current.Auth.Enabled = previous })

			called := false
			handler := CSRFProtect(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				called = true
			}))

			req := httptest.NewRequest(tt.method, "http://"+tt.host+"/api/db", nil)
			req.Host = tt.host
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}
			if tt.withCookie {
				req.AddCookie(&http.Cookie{Name: routes_auth.SessionCookieName, Value: "token"})
			}

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if called != tt.wantAllowed {
				t.Fatalf("handler called = %v, want %v", called, tt.wantAllowed)
			}
			if !tt.wantAllowed && rec.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
			}
		})
	}
}
