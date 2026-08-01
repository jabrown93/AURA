package routes_auth

import (
	"aura/config"
	"aura/logging"
	"crypto/subtle"
	"net/http"

	"golang.org/x/oauth2"
)

// HandleOIDCCallback godoc
// @Summary      OIDC Callback
// @Description  Complete the OpenID Connect code exchange and start a session
// @Tags         Auth
// @Param        code   query  string  true   "Authorization code"
// @Param        state  query  string  true   "Opaque state issued when sign-in started"
// @Success      302  "Redirect to the app on success, or to /login with an error code"
// @Router       /api/auth/oidc/callback [get]
func HandleOIDCCallback(w http.ResponseWriter, r *http.Request) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Complete OIDC Login", logging.LevelInfo)
	defer func() {
		logAction.Complete()
		ld.Log()
	}()

	query := r.URL.Query()

	// The provider reports refusals here rather than by failing the redirect.
	if providerErr := query.Get("error"); providerErr != "" {
		logAction.SetError("Identity provider rejected the sign-in", providerErr, map[string]any{
			"error_description": query.Get("error_description"),
		})
		clearOIDCTransaction(w, r)
		redirectToLoginError(w, r, oidcErrorProvider)
		return
	}

	client, err := getOIDCClient(ctx)
	if err != nil {
		logAction.SetError("Failed to complete OIDC login", err.Error(), nil)
		clearOIDCTransaction(w, r)
		if err == ErrOIDCNotEnabled {
			redirectToLoginError(w, r, oidcErrorDisabled)
			return
		}
		redirectToLoginError(w, r, oidcErrorProvider)
		return
	}

	tx, err := consumeOIDCTransaction(w, r, query.Get("state"))
	if err != nil {
		logAction.SetError("OIDC state check failed", err.Error(), nil)
		redirectToLoginError(w, r, oidcErrorState)
		return
	}

	code := query.Get("code")
	if code == "" {
		logAction.SetError("OIDC callback is missing the authorization code", "The provider redirected back without a code", nil)
		redirectToLoginError(w, r, oidcErrorState)
		return
	}

	oauth2Token, err := client.oauth2Config.Exchange(ctx, code, oauth2.VerifierOption(tx.Verifier))
	if err != nil {
		logAction.SetError("Failed to exchange the authorization code", err.Error(), nil)
		redirectToLoginError(w, r, oidcErrorExchange)
		return
	}

	rawIDToken, ok := oauth2Token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		logAction.SetError("Identity provider returned no ID token", "The token response did not include id_token", nil)
		redirectToLoginError(w, r, oidcErrorVerify)
		return
	}

	// Verifies signature against the provider's JWKS, plus issuer, audience and expiry.
	idToken, err := client.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		logAction.SetError("ID token verification failed", err.Error(), nil)
		redirectToLoginError(w, r, oidcErrorVerify)
		return
	}

	// Not covered by Verify: proves this token answers the request we started.
	if subtle.ConstantTimeCompare([]byte(idToken.Nonce), []byte(tx.Nonce)) != 1 {
		logAction.SetError("ID token nonce does not match", "The token does not belong to this sign-in attempt", nil)
		redirectToLoginError(w, r, oidcErrorVerify)
		return
	}

	var claims map[string]any
	if err := idToken.Claims(&claims); err != nil {
		logAction.SetError("Failed to read ID token claims", err.Error(), nil)
		redirectToLoginError(w, r, oidcErrorVerify)
		return
	}

	cfg := config.Current.Auth.OIDC
	identity := extractIdentity(claims, cfg.GroupsClaimOrDefault())
	if identity.Subject == "" {
		logAction.SetError("ID token has no subject", "The provider did not identify the user", nil)
		redirectToLoginError(w, r, oidcErrorVerify)
		return
	}

	if err := authorizeIdentity(cfg, identity); err != nil {
		logAction.SetError("Sign-in denied", err.Error(), map[string]any{
			"user":   identityLabel(identity),
			"groups": identity.Groups,
		})
		redirectToLoginError(w, r, oidcErrorForbidden)
		return
	}

	signedToken, err := IssueSessionToken(identity.Subject, map[string]any{
		"email":  identity.Email,
		"groups": identity.Groups,
		"idp":    "oidc",
	})
	if err != nil {
		logAction.SetError("Failed to generate token", err.Error(), nil)
		redirectToLoginError(w, r, oidcErrorInternal)
		return
	}

	SetSessionCookie(w, r, signedToken)
	logAction.AppendResult("user", identityLabel(identity))

	http.Redirect(w, r, "/", http.StatusFound)
}
