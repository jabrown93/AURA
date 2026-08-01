package routes_auth

import (
	"net/http"
	"strings"
	"time"
)

const (
	// SessionCookieName is the cookie the browser stores the AURA JWT in.
	SessionCookieName = "aura_session"

	// SessionTTL matches the "exp" claim set on issued tokens.
	SessionTTL = 24 * time.Hour
)

// TokenFromSessionCookie extracts the AURA JWT from the session cookie so it can be
// passed to jwtauth.Verify alongside the Authorization header extractor.
func TokenFromSessionCookie(r *http.Request) string {
	cookie, err := r.Cookie(SessionCookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}

// SetSessionCookie stores the JWT in an HttpOnly cookie so it is unreadable from JavaScript.
func SetSessionCookie(w http.ResponseWriter, r *http.Request, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(SessionTTL.Seconds()),
		HttpOnly: true,
		// Lax rather than Strict: the OIDC provider redirects the browser back to the
		// callback as a cross-site navigation, and Strict would withhold the cookie.
		SameSite: http.SameSiteLaxMode,
		Secure:   RequestIsSecure(r),
	})
}

// ClearSessionCookie expires the session cookie.
func ClearSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   RequestIsSecure(r),
	})
}

// RequestIsSecure reports whether the client reached us over TLS, directly or through a
// reverse proxy. Browsers drop Secure cookies sent over plain HTTP, so the flag has to be
// decided per request rather than hardcoded.
func RequestIsSecure(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		// Proxies may append to the header when chained; the client-facing scheme is first.
		first, _, _ := strings.Cut(proto, ",")
		return strings.EqualFold(strings.TrimSpace(first), "https")
	}
	return false
}
