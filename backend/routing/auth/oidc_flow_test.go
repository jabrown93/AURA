package routes_auth

import (
	"aura/config"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/lestrrat-go/jwx/v3/jwk"
	"github.com/lestrrat-go/jwx/v3/jwt"
)

// fakeProvider is a minimal OpenID provider: discovery, a JWKS, and a token endpoint that
// hands back an ID token the test controls.
type fakeProvider struct {
	server     *httptest.Server
	signingKey jwk.Key
	clientID   string

	// idTokenClaims is applied to every ID token issued; tests mutate it to exercise
	// verification failures.
	idTokenClaims map[string]any

	lastTokenForm url.Values
}

func newFakeProvider(t *testing.T, clientID string) *fakeProvider {
	t.Helper()

	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate a signing key: %v", err)
	}
	signingKey, err := jwk.Import(rsaKey)
	if err != nil {
		t.Fatalf("failed to import the signing key: %v", err)
	}
	if err := signingKey.Set(jwk.KeyIDKey, "test-key"); err != nil {
		t.Fatalf("failed to set the key id: %v", err)
	}

	provider := &fakeProvider{signingKey: signingKey, clientID: clientID, idTokenClaims: map[string]any{}}

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, map[string]any{
			"issuer":                                provider.issuer(),
			"authorization_endpoint":                provider.issuer() + "/authorize",
			"token_endpoint":                        provider.issuer() + "/token",
			"jwks_uri":                              provider.issuer() + "/jwks",
			"end_session_endpoint":                  provider.issuer() + "/end-session",
			"id_token_signing_alg_values_supported": []string{"RS256"},
		})
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, _ *http.Request) {
		public, err := jwk.PublicKeyOf(provider.signingKey)
		if err != nil {
			t.Errorf("failed to derive the public key: %v", err)
			return
		}
		set := jwk.NewSet()
		if err := set.AddKey(public); err != nil {
			t.Errorf("failed to build the key set: %v", err)
			return
		}
		writeJSON(w, set)
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Errorf("failed to parse the token request: %v", err)
			return
		}
		provider.lastTokenForm = r.PostForm

		writeJSON(w, map[string]any{
			"access_token": "access-token",
			"token_type":   "bearer",
			"expires_in":   3600,
			"id_token":     provider.signIDToken(t),
		})
	})

	provider.server = httptest.NewServer(mux)
	t.Cleanup(provider.server.Close)
	return provider
}

func (p *fakeProvider) issuer() string {
	if p.server == nil {
		// Discovery is fetched after the server is up; only the handlers read this.
		return ""
	}
	return p.server.URL
}

func (p *fakeProvider) signIDToken(t *testing.T) string {
	t.Helper()

	token := jwt.New()
	claims := map[string]any{
		"iss": p.issuer(),
		"aud": p.clientID,
		"exp": time.Now().Add(5 * time.Minute).Unix(),
		"iat": time.Now().Unix(),
	}
	for k, v := range p.idTokenClaims {
		claims[k] = v
	}
	for k, v := range claims {
		if err := token.Set(k, v); err != nil {
			t.Fatalf("failed to set claim %s: %v", k, err)
		}
	}

	signed, err := jwt.Sign(token, jwt.WithKey(jwa.RS256(), p.signingKey))
	if err != nil {
		t.Fatalf("failed to sign the ID token: %v", err)
	}
	return string(signed)
}

func writeJSON(w http.ResponseWriter, payload any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(payload)
}

// useOIDCConfig points the app at the fake provider and resets the cached client so the
// next call rediscovers.
func useOIDCConfig(t *testing.T, cfg config.Config_OIDC) {
	t.Helper()

	previous := config.Current.Auth
	config.Current.Auth = config.Config_Auth{Enabled: true, OIDC: cfg}

	oidcMu.Lock()
	oidcCached = nil
	oidcMu.Unlock()

	t.Cleanup(func() {
		config.Current.Auth = previous
		oidcMu.Lock()
		oidcCached = nil
		oidcMu.Unlock()
	})
}

// signIn drives a full authorization code flow and returns the callback's response.
func signIn(t *testing.T, provider *fakeProvider) *httptest.ResponseRecorder {
	t.Helper()

	loginRec := httptest.NewRecorder()
	StartOIDCLogin(loginRec, httptest.NewRequest(http.MethodGet, "http://aura.example.com/api/auth/oidc/login", nil))

	if loginRec.Code != http.StatusFound {
		t.Fatalf("login status = %d, want %d", loginRec.Code, http.StatusFound)
	}
	authURL, err := url.Parse(loginRec.Header().Get("Location"))
	if err != nil {
		t.Fatalf("failed to parse the authorize URL: %v", err)
	}
	query := authURL.Query()

	if query.Get("code_challenge") == "" || query.Get("code_challenge_method") != "S256" {
		t.Fatal("authorize request must use PKCE with S256")
	}

	// The provider echoes the nonce it was given, as a real one does.
	if _, set := provider.idTokenClaims["nonce"]; !set {
		provider.idTokenClaims["nonce"] = query.Get("nonce")
	}

	callbackRec := httptest.NewRecorder()
	callback := httptest.NewRequest(http.MethodGet,
		"http://aura.example.com/api/auth/oidc/callback?code=test-code&state="+url.QueryEscape(query.Get("state")), nil)
	for _, cookie := range loginRec.Result().Cookies() {
		callback.AddCookie(cookie)
	}
	HandleOIDCCallback(callbackRec, callback)

	return callbackRec
}

func sessionCookieFrom(rec *httptest.ResponseRecorder) *http.Cookie {
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == SessionCookieName && cookie.Value != "" {
			return cookie
		}
	}
	return nil
}

func TestOIDCSignInIssuesASession(t *testing.T) {
	withTestTokenAuth(t)

	provider := newFakeProvider(t, "aura")
	provider.idTokenClaims["sub"] = "user-subject"
	provider.idTokenClaims["email"] = "user@example.com"
	provider.idTokenClaims["groups"] = []string{"media"}

	useOIDCConfig(t, config.Config_OIDC{
		Enabled:       true,
		IssuerURL:     provider.issuer(),
		ClientID:      "aura",
		ClientSecret:  "client-secret",
		RedirectURL:   "http://aura.example.com/api/auth/oidc/callback",
		AllowedGroups: []string{"media"},
	})

	rec := signIn(t, provider)

	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "/" {
		t.Fatalf("callback returned %d to %q, want a redirect to /", rec.Code, rec.Header().Get("Location"))
	}

	cookie := sessionCookieFrom(rec)
	if cookie == nil {
		t.Fatal("no session cookie was set")
	}

	session := resolveSession(callbackRequest(cookie))
	if !session.Authenticated || session.Subject != "user-subject" {
		t.Fatalf("session = %+v, want an authenticated session for the ID token subject", session)
	}

	// PKCE only protects the exchange if the verifier actually reaches the provider.
	if provider.lastTokenForm.Get("code_verifier") == "" {
		t.Error("token request did not include the PKCE code verifier")
	}
}

func TestOIDCSignInRejections(t *testing.T) {
	withTestTokenAuth(t)

	tests := []struct {
		name      string
		claims    map[string]any
		allowed   []string
		wantError string
	}{
		{
			name:      "user outside the allowlist",
			claims:    map[string]any{"sub": "user-subject", "groups": []string{"guests"}},
			allowed:   []string{"admins"},
			wantError: oidcErrorForbidden,
		},
		{
			name:      "replayed token for another sign-in",
			claims:    map[string]any{"sub": "user-subject", "nonce": "some-other-nonce"},
			wantError: oidcErrorVerify,
		},
		{
			name:      "token without a subject",
			claims:    map[string]any{"email": "user@example.com"},
			wantError: oidcErrorVerify,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := newFakeProvider(t, "aura")
			provider.idTokenClaims = tt.claims

			useOIDCConfig(t, config.Config_OIDC{
				Enabled:       true,
				IssuerURL:     provider.issuer(),
				ClientID:      "aura",
				ClientSecret:  "client-secret",
				RedirectURL:   "http://aura.example.com/api/auth/oidc/callback",
				AllowedGroups: tt.allowed,
			})

			rec := signIn(t, provider)

			if got := rec.Header().Get("Location"); got != "/login?error="+tt.wantError {
				t.Fatalf("redirected to %q, want the %s error", got, tt.wantError)
			}
			if sessionCookieFrom(rec) != nil {
				t.Fatal("a rejected sign-in must not set a session cookie")
			}
		})
	}
}

func TestOIDCLoginRedirectsWhenDisabled(t *testing.T) {
	withTestTokenAuth(t)

	previous := config.Current.Auth
	config.Current.Auth = config.Config_Auth{Enabled: true}
	t.Cleanup(func() { config.Current.Auth = previous })

	rec := httptest.NewRecorder()
	StartOIDCLogin(rec, httptest.NewRequest(http.MethodGet, "http://aura.example.com/api/auth/oidc/login", nil))

	if got := rec.Header().Get("Location"); got != "/login?error="+oidcErrorDisabled {
		t.Fatalf("redirected to %q, want the disabled error", got)
	}
}

func TestEndSessionURL(t *testing.T) {
	withTestTokenAuth(t)

	provider := newFakeProvider(t, "aura")
	useOIDCConfig(t, config.Config_OIDC{
		Enabled:           true,
		IssuerURL:         provider.issuer(),
		ClientID:          "aura",
		ClientSecret:      "client-secret",
		RedirectURL:       "https://aura.example.com/api/auth/oidc/callback",
		RPInitiatedLogout: true,
	})

	got, err := url.Parse(endSessionURL(t.Context()))
	if err != nil {
		t.Fatalf("failed to parse the end session URL: %v", err)
	}

	if got.Path != "/end-session" {
		t.Fatalf("path = %q, want the provider's end_session_endpoint", got.Path)
	}
	if got.Query().Get("client_id") != "aura" {
		t.Error("end session URL must identify the client")
	}
	if got.Query().Get("post_logout_redirect_uri") != "https://aura.example.com/login" {
		t.Errorf("post_logout_redirect_uri = %q, want the login page on the callback's origin", got.Query().Get("post_logout_redirect_uri"))
	}
}

func TestEndSessionURLEmptyWhenSingleLogoutIsOff(t *testing.T) {
	withTestTokenAuth(t)

	provider := newFakeProvider(t, "aura")
	useOIDCConfig(t, config.Config_OIDC{
		Enabled:      true,
		IssuerURL:    provider.issuer(),
		ClientID:     "aura",
		ClientSecret: "client-secret",
		RedirectURL:  "https://aura.example.com/api/auth/oidc/callback",
	})

	if got := endSessionURL(t.Context()); got != "" {
		t.Fatalf("endSessionURL() = %q, want empty when RP-initiated logout is off", got)
	}
}
