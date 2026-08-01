package routes_auth

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestIsSecure(t *testing.T) {
	tests := []struct {
		name      string
		tls       bool
		forwarded string
		want      bool
	}{
		{name: "plain http", want: false},
		{name: "direct tls", tls: true, want: true},
		{name: "proxied https", forwarded: "https", want: true},
		{name: "proxied http", forwarded: "http", want: false},
		{name: "chained proxies keep the client scheme first", forwarded: "https, http", want: true},
		{name: "case insensitive", forwarded: "HTTPS", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "http://aura.example.com/api/config", nil)
			if tt.tls {
				req.TLS = &tls.ConnectionState{}
			}
			if tt.forwarded != "" {
				req.Header.Set("X-Forwarded-Proto", tt.forwarded)
			}

			if got := RequestIsSecure(req); got != tt.want {
				t.Fatalf("RequestIsSecure() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSessionCookieRoundTrip(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "http://aura.example.com/api/login", nil)

	SetSessionCookie(rec, req, "signed.jwt.value")

	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected exactly one cookie, got %d", len(cookies))
	}
	cookie := cookies[0]

	if cookie.Name != SessionCookieName || cookie.Value != "signed.jwt.value" {
		t.Fatalf("unexpected cookie %s=%s", cookie.Name, cookie.Value)
	}
	if !cookie.HttpOnly {
		t.Error("session cookie must be HttpOnly so scripts cannot read the token")
	}
	if cookie.SameSite != http.SameSiteLaxMode {
		t.Errorf("SameSite = %v, want Lax", cookie.SameSite)
	}
	if cookie.Secure {
		t.Error("Secure must not be set for a plain HTTP request, browsers would drop the cookie")
	}

	next := httptest.NewRequest(http.MethodGet, "http://aura.example.com/api/config", nil)
	next.AddCookie(cookie)
	if got := TokenFromSessionCookie(next); got != "signed.jwt.value" {
		t.Fatalf("TokenFromSessionCookie() = %q, want the stored token", got)
	}
}

func TestClearSessionCookie(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "https://aura.example.com/api/logout", nil)
	req.TLS = &tls.ConnectionState{}

	ClearSessionCookie(rec, req)

	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected exactly one cookie, got %d", len(cookies))
	}
	if cookies[0].MaxAge >= 0 {
		t.Fatalf("MaxAge = %d, want a negative value to expire the cookie", cookies[0].MaxAge)
	}
	if !cookies[0].Secure {
		t.Error("Secure must be set when the request arrived over TLS")
	}
}

func TestTokenFromSessionCookieMissing(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://aura.example.com/api/config", nil)
	if got := TokenFromSessionCookie(req); got != "" {
		t.Fatalf("TokenFromSessionCookie() = %q, want empty string", got)
	}
}
