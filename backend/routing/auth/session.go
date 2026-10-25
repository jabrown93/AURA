package routes_auth

import (
	"aura/config"
	"net/http"
	"time"
)

// SessionCookieName is the name of the browser session cookie used for interactive login
// (password or OIDC). It carries a signed JWT, but unlike the old bearer-token-in-JSON-body
// flow, it's HttpOnly so it can't be read by page JS, and it's never returned in a response body.
const SessionCookieName = "aura_session"

const sessionTTL = 24 * time.Hour

// IssueSessionCookie signs a new session JWT (sub: "aura") and sets it as an HttpOnly,
// SameSite=Lax cookie on the response. Used by both password login and the OIDC callback.
func IssueSessionCookie(w http.ResponseWriter, r *http.Request) error {
	now := time.Now()
	claims := map[string]any{
		"sub": "aura",
		"iat": now.Unix(),
		"exp": now.Add(sessionTTL).Unix(),
	}

	_, signedToken, err := TokenAuth.Encode(claims)
	if err != nil {
		return err
	}

	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    signedToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   resolveSecure(r),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(sessionTTL.Seconds()),
	})
	return nil
}

// ClearSessionCookie expires the session cookie. Safe to call even if no session exists.
func ClearSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   resolveSecure(r),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

// resolveSecure decides whether a cookie should carry the Secure attribute for this request.
// This app is very commonly self-hosted on a plain-HTTP LAN (http://192.168.x.x:PORT), where a
// hardcoded Secure=true would make the browser silently discard the cookie and login would
// appear to fail with no clear error. Default ("auto") reflects the actual inbound transport.
func resolveSecure(r *http.Request) bool {
	switch config.Current.Auth.SessionCookieSecure {
	case "always":
		return true
	case "never":
		return false
	}

	if r.TLS != nil {
		return true
	}
	if config.Current.Auth.TrustProxyForCookieSecure && r.Header.Get("X-Forwarded-Proto") == "https" {
		return true
	}
	return false
}
