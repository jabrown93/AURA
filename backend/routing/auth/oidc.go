package routes_auth

import (
	"aura/config"
	"aura/logging"
	"aura/utils/httpx"
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

const oidcStateCookieName = "aura_oidc_state"

// The Redirect URL the login started with, so the callback completes it with the same one
const oidcRedirectCookieName = "aura_oidc_redirect"

// The PKCE code verifier, so a stolen login code can't be used without it
const oidcVerifierCookieName = "aura_oidc_verifier"
const oidcStateTTLSeconds = 5 * 60

// A lazy retry runs inside a login page request, so it gets less time than the startup attempt
const oidcRetryTimeout = 5 * time.Second

// Minimum time between lazy retries, so a down IdP doesn't turn every login page load into a discovery call
const oidcRetryInterval = 30 * time.Second

// How long a successful discovery is trusted. After this a login page visit re-fetches the
// document in the background of the request, so an IdP change (new signing key algorithm, moved
// endpoints, provider re-pointed at this client) is picked up without a restart. The old
// snapshot keeps serving until the new one is published.
const oidcRefreshInterval = 10 * time.Minute

// oidcClient is what a successful discovery produced. Handlers take one snapshot, so a concurrent
// re-init (config save, lazy retry) can't swap part of it out under them.
type oidcClient struct {
	oauth2Cfg  *oauth2.Config
	idVerifier *oidc.IDTokenVerifier
}

var (
	// nil until discovery succeeds
	oidcCurrent atomic.Pointer[oidcClient]

	// Serializes discovery attempts, so concurrent login page loads don't stampede the IdP
	oidcInitMu      sync.Mutex
	oidcLastAttempt time.Time    // guarded by oidcInitMu
	oidcLastSuccess atomic.Int64 // unix nanos; read without the lock on the login page fast path
)

// InitOIDC performs OIDC discovery against the configured issuer and, on success, makes OIDC
// login available. If discovery fails (unreachable/misconfigured IdP), OIDC is left unavailable
// until a login page visit retries it (see ensureOIDC) - the rest of the app must keep working,
// since a broken IdP shouldn't take down the whole server. The error is returned so callers can
// show why OIDC is unavailable.
func InitOIDC(ctx context.Context) logging.LogErrorInfo {
	oidcInitMu.Lock()
	defer oidcInitMu.Unlock()
	return initOIDC(ctx)
}

// initOIDC is InitOIDC for a caller already holding oidcInitMu. The outcome replaces the current
// snapshot either way: this runs at startup and after a config save, when whatever was published
// before may describe a different IdP.
func initOIDC(ctx context.Context) logging.LogErrorInfo {
	client, errInfo := discoverOIDC(ctx)
	oidcCurrent.Store(client)
	return errInfo
}

// discoverOIDC validates the OIDC config and runs discovery, returning the client to publish
// (nil when OIDC is disabled or discovery failed). It never touches oidcCurrent, so a caller can
// decide whether a failure should unpublish the existing snapshot. Caller holds oidcInitMu.
func discoverOIDC(ctx context.Context) (*oidcClient, logging.LogErrorInfo) {
	oidcLastAttempt = time.Now()

	cfg := config.Current.Auth.OIDC
	if !cfg.Enabled {
		return nil, logging.LogErrorInfo{}
	}

	// Each Redirect URL must point to aura's OIDC callback, otherwise the login can never complete
	if len(cfg.RedirectURLs) == 0 {
		return nil, logging.LogErrorInfo{
			Message: "OIDC needs at least one Redirect URL",
			Help:    "Add your aura URL followed by /api/auth/oidc/callback (e.g. https://aura.example.com/api/auth/oidc/callback), and register the same URL with your identity provider",
		}
	}
	for _, redirect := range cfg.RedirectURLs {
		redirectURL, err := url.Parse(redirect)
		if err != nil || !strings.HasSuffix(strings.TrimRight(redirectURL.Path, "/"), config.OIDCCallbackPath) {
			logging.LOGGER.Error().Timestamp().Str("redirect_url", redirect).Msg("OIDC Redirect URL does not point to aura's OIDC callback - OIDC login will be unavailable until this is fixed")
			return nil, logging.LogErrorInfo{
				Message: "OIDC Redirect URL must point to aura's OIDC callback",
				Help:    "Set it to your aura URL followed by /api/auth/oidc/callback (e.g. https://aura.example.com/api/auth/oidc/callback), and register the same URL with your identity provider",
				Detail:  map[string]any{"redirect_url": redirect},
			}
		}
	}

	provider, err := oidc.NewProvider(ctx, cfg.IssuerURL)
	if err != nil {
		logging.LOGGER.Error().Timestamp().Str("issuer_url", cfg.IssuerURL).Err(err).Msg("OIDC discovery failed - OIDC login will be unavailable until the identity provider can be reached, discovery is retried on the next login page visit")
		return nil, logging.LogErrorInfo{
			Message: "OIDC discovery failed",
			Help:    "Ensure the Issuer URL is the base URL of your identity provider (e.g. https://idp.example.com) and that aura can reach it",
			Detail:  map[string]any{"issuer_url": cfg.IssuerURL, "error": err.Error()},
		}
	}

	oidcLastSuccess.Store(time.Now().UnixNano())
	logging.LOGGER.Info().Timestamp().Str("issuer_url", cfg.IssuerURL).Msg("OIDC initialized successfully")
	return &oidcClient{
		oauth2Cfg: &oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			RedirectURL:  cfg.RedirectURLs[0],
			Endpoint:     provider.Endpoint(),
			Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
		},
		idVerifier: provider.Verifier(&oidc.Config{ClientID: cfg.ClientID}),
	}, logging.LogErrorInfo{}
}

// ensureOIDC keeps the published snapshot usable from a login page request. When OIDC is enabled
// but not initialized - typically because the identity provider was unreachable while aura
// booted - it retries discovery, so an SSO-only deployment isn't locked out until a restart. When
// the snapshot is older than oidcRefreshInterval it re-fetches the document so IdP-side changes
// are picked up; the current snapshot stays published throughout and is only replaced by a
// successful discovery, so in-flight logins never see a half-initialized or missing client.
// Attempts are serialized and throttled, so a down IdP is hit at most once per oidcRetryInterval
// however often the login page is loaded.
func ensureOIDC() {
	if !config.AuthEnabled() || !config.Current.Auth.OIDC.Enabled {
		return
	}
	if oidcCurrent.Load() != nil && time.Since(time.Unix(0, oidcLastSuccess.Load())) < oidcRefreshInterval {
		return
	}
	oidcInitMu.Lock()
	defer oidcInitMu.Unlock()
	// The request that held the lock may have just retried or refreshed
	if time.Since(oidcLastAttempt) < oidcRetryInterval {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), oidcRetryTimeout)
	defer cancel()
	if client, _ := discoverOIDC(ctx); client != nil {
		oidcCurrent.Store(client)
	}
}

// loadOIDC returns the initialized OIDC client, or nil when OIDC login is unavailable
func loadOIDC() *oidcClient {
	if !config.AuthEnabled() || !config.Current.Auth.OIDC.Enabled {
		return nil
	}
	return oidcCurrent.Load()
}

// OIDCAvailable reports whether OIDC login is configured, enabled, and successfully initialized.
func OIDCAvailable() bool {
	return loadOIDC() != nil
}

// OIDCLoginRedirect godoc
// @Summary      Start OIDC Login
// @Description  Redirects to the configured OIDC identity provider to start a browser login. Not usable programmatically - this is a browser redirect flow.
// @Tags         Auth
// @Success      302
// @Router       /api/auth/oidc/login [get]
func OIDCLoginRedirect(w http.ResponseWriter, r *http.Request) {
	ensureOIDC()
	client := loadOIDC()
	if client == nil {
		http.Redirect(w, r, "/login?error=oidc_unavailable", http.StatusFound)
		return
	}

	state, err := randomHexString(32)
	if err != nil {
		http.Redirect(w, r, "/login?error=oidc_failed", http.StatusFound)
		return
	}

	setOIDCLoginCookie(w, r, oidcStateCookieName, state)

	// Come back to the address the login started from, when it has a Redirect URL
	redirectIndex := redirectURLIndexForOrigin(r.URL.Query().Get("origin"))
	setOIDCLoginCookie(w, r, oidcRedirectCookieName, strconv.Itoa(redirectIndex))

	verifier := oauth2.GenerateVerifier()
	setOIDCLoginCookie(w, r, oidcVerifierCookieName, verifier)

	redirectURL := config.Current.Auth.OIDC.RedirectURLs[redirectIndex]
	http.Redirect(w, r, client.oauth2Cfg.AuthCodeURL(state,
		oauth2.SetAuthURLParam("redirect_uri", redirectURL),
		oauth2.S256ChallengeOption(verifier),
	), http.StatusFound)
}

// setOIDCLoginCookie keeps a value until the identity provider sends the browser back
func setOIDCLoginCookie(w http.ResponseWriter, r *http.Request, name, value string) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		Secure:   resolveSecure(r),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   oidcStateTTLSeconds,
	})
}

// redirectURLIndexForOrigin returns the Redirect URL with the same scheme and host as the origin.
// Only the configured URLs can be picked, an origin without one uses the first.
func redirectURLIndexForOrigin(origin string) int {
	originURL, err := url.Parse(strings.TrimSpace(origin))
	if err != nil || originURL.Host == "" {
		return 0
	}
	for i, redirect := range config.Current.Auth.OIDC.RedirectURLs {
		redirectURL, err := url.Parse(redirect)
		if err == nil && strings.EqualFold(redirectURL.Scheme, originURL.Scheme) && strings.EqualFold(redirectURL.Host, originURL.Host) {
			return i
		}
	}
	return 0
}

// redirectURLFromCookie returns the Redirect URL the login started with
func redirectURLFromCookie(r *http.Request) string {
	redirectURLs := config.Current.Auth.OIDC.RedirectURLs
	if cookie, err := r.Cookie(oidcRedirectCookieName); err == nil {
		if i, err := strconv.Atoi(cookie.Value); err == nil && i >= 0 && i < len(redirectURLs) {
			return redirectURLs[i]
		}
	}
	return redirectURLs[0]
}

// OIDCCallback godoc
// @Summary      OIDC Callback
// @Description  Handles the redirect back from the OIDC identity provider, exchanges the authorization code, verifies the ID token, and - if the authenticated identity is allowed - starts a browser session the same way password login does.
// @Tags         Auth
// @Success      302
// @Router       /api/auth/oidc/callback [get]
func OIDCCallback(w http.ResponseWriter, r *http.Request) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("OIDC Callback", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, logAction)
	defer logAction.Complete()

	clearOIDCStateCookie(w, r)

	client := loadOIDC()
	if client == nil {
		http.Redirect(w, r, "/login?error=oidc_unavailable", http.StatusFound)
		return
	}

	stateCookie, err := r.Cookie(oidcStateCookieName)
	if err != nil || stateCookie.Value == "" || stateCookie.Value != r.URL.Query().Get("state") {
		logAction.SetError("OIDC state mismatch", "The state parameter did not match - the login attempt may have expired or been tampered with", nil)
		http.Redirect(w, r, "/login?error=oidc_failed", http.StatusFound)
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		logAction.SetError("OIDC callback missing code", "", nil)
		http.Redirect(w, r, "/login?error=oidc_failed", http.StatusFound)
		return
	}

	verifierCookie, err := r.Cookie(oidcVerifierCookieName)
	if err != nil || verifierCookie.Value == "" {
		logAction.SetError("OIDC code verifier missing", "The login attempt may have expired, try logging in again", nil)
		http.Redirect(w, r, "/login?error=oidc_failed", http.StatusFound)
		return
	}

	oauth2Token, err := client.oauth2Cfg.Exchange(ctx, code,
		oauth2.SetAuthURLParam("redirect_uri", redirectURLFromCookie(r)),
		oauth2.VerifierOption(verifierCookie.Value),
	)
	if err != nil {
		logAction.SetError("OIDC code exchange failed", err.Error(), nil)
		http.Redirect(w, r, "/login?error=oidc_failed", http.StatusFound)
		return
	}

	rawIDToken, ok := oauth2Token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		logAction.SetError("OIDC token response missing id_token", "", nil)
		http.Redirect(w, r, "/login?error=oidc_failed", http.StatusFound)
		return
	}

	idToken, err := client.idVerifier.Verify(ctx, rawIDToken)
	if err != nil {
		logAction.SetError("OIDC ID token verification failed", err.Error(), nil)
		http.Redirect(w, r, "/login?error=oidc_failed", http.StatusFound)
		return
	}

	var claims struct {
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
	}
	if err := idToken.Claims(&claims); err != nil {
		logAction.SetError("Failed to parse OIDC claims", err.Error(), nil)
		http.Redirect(w, r, "/login?error=oidc_failed", http.StatusFound)
		return
	}

	// Fork: an unverified email can't satisfy the email allowlist, and AllowedGroups is enforced.
	if reason := forkOIDCDenied(idToken, claims.Email, claims.EmailVerified); reason != "" {
		logAction.SetError("OIDC identity not allowed", reason, map[string]any{"email": claims.Email})
		http.Redirect(w, r, "/login?error=oidc_not_allowed", http.StatusFound)
		return
	}

	if !isOIDCIdentityAllowed(claims.Email) {
		logAction.SetError("OIDC identity not allowed", "The authenticated email is not in the configured allowed emails/domains list", map[string]any{"email": claims.Email})
		http.Redirect(w, r, "/login?error=oidc_not_allowed", http.StatusFound)
		return
	}

	if err := IssueSessionCookie(w, r); err != nil {
		logAction.SetError("Failed to start session", err.Error(), nil)
		http.Redirect(w, r, "/login?error=oidc_failed", http.StatusFound)
		return
	}

	logAction.AppendResult("oidc_login_success", claims.Email)
	http.Redirect(w, r, "/", http.StatusFound)
}

// isOIDCIdentityAllowed checks the authenticated email against the configured allowlists.
// If both AllowedEmails and AllowedDomains are empty, any successfully authenticated IdP user
// is allowed - this is surfaced as a loud warning in the Settings UI, not hidden here.
func isOIDCIdentityAllowed(email string) bool {
	cfg := config.Current.Auth.OIDC
	if len(cfg.AllowedEmails) == 0 && len(cfg.AllowedDomains) == 0 {
		return true
	}

	emailLower := strings.ToLower(strings.TrimSpace(email))
	if emailLower == "" {
		return false
	}

	for _, allowed := range cfg.AllowedEmails {
		if strings.ToLower(strings.TrimSpace(allowed)) == emailLower {
			return true
		}
	}

	domain := ""
	if idx := strings.LastIndex(emailLower, "@"); idx != -1 {
		domain = emailLower[idx+1:]
	}
	for _, allowedDomain := range cfg.AllowedDomains {
		if strings.ToLower(strings.TrimSpace(allowedDomain)) == domain {
			return true
		}
	}

	return false
}

func clearOIDCStateCookie(w http.ResponseWriter, r *http.Request) {
	for _, name := range []string{oidcStateCookieName, oidcRedirectCookieName, oidcVerifierCookieName} {
		http.SetCookie(w, &http.Cookie{
			Name:     name,
			Value:    "",
			Path:     "/",
			HttpOnly: true,
			Secure:   resolveSecure(r),
			SameSite: http.SameSiteLaxMode,
			MaxAge:   -1,
		})
	}
}

func randomHexString(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

type authMethodsResponse struct {
	PasswordEnabled bool `json:"password_enabled"`
	OIDCEnabled     bool `json:"oidc_enabled"`
}

// GetAuthMethods godoc
// @Summary      Get Available Auth Methods
// @Description  Public, minimal endpoint the login page uses to know which login methods to show (password / OIDC SSO) before the user is authenticated.
// @Tags         Auth
// @Produce      json
// @Success      200  {object}  httpx.JSONResponse{data=authMethodsResponse}
// @Router       /api/config/auth-methods [get]
func GetAuthMethods(w http.ResponseWriter, r *http.Request) {
	_, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	ensureOIDC()
	httpx.SendResponse(w, ld, authMethodsResponse{
		PasswordEnabled: config.AuthEnabled() && config.Current.Auth.PasswordEnabled,
		OIDCEnabled:     OIDCAvailable(),
	})
}
