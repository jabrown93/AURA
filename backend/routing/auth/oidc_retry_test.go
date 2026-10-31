package routes_auth

import (
	"aura/config"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestOIDCDiscoveryRetriesAfterStartupFailure(t *testing.T) {
	// A fake identity provider that can be taken down, and counts discovery requests
	var down atomic.Bool
	var discoveries atomic.Int32
	mux := http.NewServeMux()
	idp := httptest.NewServer(mux)
	t.Cleanup(idp.Close)
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		discoveries.Add(1)
		if down.Load() {
			http.Error(w, "down", http.StatusBadGateway)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer":                 idp.URL,
			"authorization_endpoint": idp.URL + "/authorize",
			"token_endpoint":         idp.URL + "/token",
			"jwks_uri":               idp.URL + "/jwks",
		})
	})

	previous := config.Current
	t.Cleanup(func() {
		config.Current = previous
		_ = InitOIDC(context.Background())
	})
	config.Current.Auth.Enabled = true
	config.Current.Auth.PasswordEnabled = false
	config.Current.Auth.OIDC = config.Config_Auth_OIDC{
		Enabled:      true,
		IssuerURL:    idp.URL,
		ClientID:     "aura",
		ClientSecret: "secret",
		RedirectURLs: []string{"http://aura.test/api/auth/oidc/callback"},
	}

	authMethods := func() authMethodsResponse {
		t.Helper()
		w := httptest.NewRecorder()
		GetAuthMethods(w, httptest.NewRequest(http.MethodGet, "/api/config/auth-methods", nil))
		var body struct {
			Data authMethodsResponse `json:"data"`
		}
		if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
			t.Fatalf("auth-methods: %v", err)
		}
		return body.Data
	}
	loginLocation := func() string {
		t.Helper()
		w := httptest.NewRecorder()
		OIDCLoginRedirect(w, httptest.NewRequest(http.MethodGet, "/api/auth/oidc/login", nil))
		return w.Header().Get("Location")
	}

	// The IdP is unreachable while aura boots
	down.Store(true)
	if err := InitOIDC(context.Background()); err.Message == "" {
		t.Fatal("InitOIDC succeeded against a down IdP")
	}
	if methods := authMethods(); methods.OIDCEnabled {
		t.Fatal("OIDC reported available right after failed discovery")
	}
	if got := loginLocation(); got != "/login?error=oidc_unavailable" {
		t.Fatalf("login Location = %q", got)
	}
	// Neither request may retry so soon after the startup attempt
	if n := discoveries.Load(); n != 1 {
		t.Fatalf("discovery requests = %d, want 1 (throttled)", n)
	}

	// Once the throttle has passed, a login page visit retries - and a still-down IdP fails again
	oidcLastAttempt = time.Time{}
	if methods := authMethods(); methods.OIDCEnabled {
		t.Fatal("OIDC reported available while IdP still down")
	}
	if n := discoveries.Load(); n != 2 {
		t.Fatalf("discovery requests = %d, want 2", n)
	}

	// The IdP comes back: the next login page visit retries and SSO works, without a restart
	down.Store(false)
	oidcLastAttempt = time.Time{}
	if methods := authMethods(); !methods.OIDCEnabled {
		t.Fatal("OIDC not available after the IdP came back")
	}
	if got := loginLocation(); !strings.HasPrefix(got, idp.URL+"/authorize?") {
		t.Fatalf("login Location = %q, want IdP authorize URL", got)
	}
	// Once initialized, no further discovery happens
	if n := discoveries.Load(); n != 3 {
		t.Fatalf("discovery requests = %d, want 3", n)
	}
}
