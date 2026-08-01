package routes_auth

import (
	"aura/logging"
	"net/http"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

// Sign-in failures are reported to the UI as codes rather than provider text: the browser
// is mid-redirect, so there is no JSON response to carry an error, and reflecting provider
// strings into the login page would be handing an attacker a paintbrush.
const (
	oidcErrorDisabled  = "oidc_disabled"
	oidcErrorProvider  = "oidc_provider"
	oidcErrorState     = "oidc_state"
	oidcErrorExchange  = "oidc_exchange"
	oidcErrorVerify    = "oidc_verify"
	oidcErrorForbidden = "oidc_forbidden"
	oidcErrorInternal  = "oidc_internal"
)

// StartOIDCLogin godoc
// @Summary      Start OIDC Login
// @Description  Redirect the browser to the configured OpenID Connect provider
// @Tags         Auth
// @Success      302  "Redirect to the identity provider"
// @Router       /api/auth/oidc/login [get]
func StartOIDCLogin(w http.ResponseWriter, r *http.Request) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Start OIDC Login", logging.LevelInfo)
	defer func() {
		logAction.Complete()
		ld.Log()
	}()

	client, err := getOIDCClient(ctx)
	if err != nil {
		logAction.SetError("Failed to start OIDC login", err.Error(), nil)
		if err == ErrOIDCNotEnabled {
			redirectToLoginError(w, r, oidcErrorDisabled)
			return
		}
		redirectToLoginError(w, r, oidcErrorProvider)
		return
	}

	tx, err := startOIDCTransaction(w, r)
	if err != nil {
		logAction.SetError("Failed to start OIDC login", err.Error(), nil)
		redirectToLoginError(w, r, oidcErrorInternal)
		return
	}

	authURL := client.oauth2Config.AuthCodeURL(
		tx.State,
		oidc.Nonce(tx.Nonce),
		oauth2.S256ChallengeOption(tx.Verifier),
	)

	http.Redirect(w, r, authURL, http.StatusFound)
}

// redirectToLoginError sends the browser back to the login page with a code it can explain.
func redirectToLoginError(w http.ResponseWriter, r *http.Request, code string) {
	http.Redirect(w, r, "/login?error="+code, http.StatusFound)
}
