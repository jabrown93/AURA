package routes_auth

import (
	"aura/config"
	"aura/logging"
	"aura/utils/httpx"
	"context"
	"net/http"
	"net/url"
)

type logoutResponse struct {
	LoggedOut bool `json:"logged_out"`
	// EndSessionURL is set when the provider should also be told to end its session.
	// The UI navigates there after logging out locally.
	EndSessionURL string `json:"end_session_url,omitempty"`
}

// Logout godoc
// @Summary      Auth Logout
// @Description  Clear the session cookie, and report where to end the provider session when OIDC single logout is enabled
// @Tags         Auth
// @Produce      json
// @Success      200  {object}  httpx.JSONResponse{data=routes_auth.logoutResponse}
// @Router       /api/logout [post]
func Logout(w http.ResponseWriter, r *http.Request) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("User Logout", logging.LevelInfo)
	defer logAction.Complete()

	ClearSessionCookie(w, r)

	httpx.SendResponse(w, ld, logoutResponse{
		LoggedOut:     true,
		EndSessionURL: endSessionURL(ctx),
	})
}

// endSessionURL builds the provider's RP-initiated logout URL, or returns "" when single
// logout is off or the provider does not advertise the endpoint.
//
// The local session is already gone by this point, so a provider that cannot be reached
// costs the user nothing beyond staying signed in at the IdP.
func endSessionURL(ctx context.Context) string {
	cfg := config.Current.Auth.OIDC
	if !config.Current.Auth.Enabled || !cfg.Enabled || !cfg.RPInitiatedLogout {
		return ""
	}

	client, err := getOIDCClient(ctx)
	if err != nil || client.endSessionURL == "" {
		return ""
	}

	parsed, err := url.Parse(client.endSessionURL)
	// The UI navigates to this value, so anything that is not a plain http(s) URL - a
	// javascript: scheme from a malformed discovery document, say - must not leave here.
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Host == "" {
		return ""
	}

	query := parsed.Query()
	query.Set("client_id", cfg.ClientID)
	// No id_token_hint: AURA does not keep the provider's ID token after sign-in, so the
	// client_id form of the request is the one available. Providers that require the
	// post-logout URL to be registered need this value allowlisted alongside the callback.
	if postLogout := postLogoutRedirectURL(cfg.RedirectURL); postLogout != "" {
		query.Set("post_logout_redirect_uri", postLogout)
	}
	parsed.RawQuery = query.Encode()

	return parsed.String()
}

// postLogoutRedirectURL points at the login page on the same origin as the configured
// callback. The callback URL is the source of truth because it is the one external URL the
// deployment has already had to get right.
func postLogoutRedirectURL(redirectURL string) string {
	parsed, err := url.Parse(redirectURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	return parsed.Scheme + "://" + parsed.Host + "/login"
}
