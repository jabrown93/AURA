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

func TestOIDCDiscoveryRefreshesStaleSnapshot(t *testing.T) {
	// A fake identity provider whose authorization endpoint can move, and that can be taken down
	var down atomic.Bool
	var authorizePath atomic.Value
	authorizePath.Store("/authorize")
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
			"authorization_endpoint": idp.URL + authorizePath.Load().(string),
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

	loginLocation := func() string {
		t.Helper()
		w := httptest.NewRecorder()
		OIDCLoginRedirect(w, httptest.NewRequest(http.MethodGet, "/api/auth/oidc/login", nil))
		return w.Header().Get("Location")
	}
	// Dates the published snapshot so the next login page visit treats it as stale
	expireSnapshot := func() {
		oidcLastAttempt = time.Time{}
		oidcLastSuccess.Store(time.Now().Add(-oidcRefreshInterval).UnixNano())
	}

	if err := InitOIDC(context.Background()); err.Message != "" {
		t.Fatalf("InitOIDC: %s", err.Message)
	}
	first := oidcCurrent.Load()
	if !strings.HasPrefix(loginLocation(), idp.URL+"/authorize?") {
		t.Fatalf("login Location = %q", loginLocation())
	}

	// A fresh snapshot is reused: no discovery per login
	if n := discoveries.Load(); n != 1 {
		t.Fatalf("discovery requests = %d, want 1", n)
	}
	if oidcCurrent.Load() != first {
		t.Fatal("fresh snapshot was replaced")
	}

	// The IdP moves its authorization endpoint. Once the snapshot is stale, the next login
	// re-fetches discovery and sends the browser to the new endpoint, without a restart.
	authorizePath.Store("/oauth2/authorize")
	expireSnapshot()
	if got := loginLocation(); !strings.HasPrefix(got, idp.URL+"/oauth2/authorize?") {
		t.Fatalf("login Location after IdP change = %q", got)
	}
	if n := discoveries.Load(); n != 2 {
		t.Fatalf("discovery requests = %d, want 2", n)
	}
	second := oidcCurrent.Load()
	if second == first {
		t.Fatal("stale snapshot was not replaced")
	}

	// A refresh against a down IdP keeps serving the last good snapshot instead of unpublishing it
	down.Store(true)
	expireSnapshot()
	if got := loginLocation(); !strings.HasPrefix(got, idp.URL+"/oauth2/authorize?") {
		t.Fatalf("login Location while IdP down = %q", got)
	}
	if n := discoveries.Load(); n != 3 {
		t.Fatalf("discovery requests = %d, want 3", n)
	}
	if oidcCurrent.Load() != second {
		t.Fatal("failed refresh replaced the published snapshot")
	}
	// ...and does not hammer the down IdP on every subsequent login
	if loginLocation(); discoveries.Load() != 3 {
		t.Fatalf("discovery requests = %d, want 3 (throttled)", discoveries.Load())
	}
}
