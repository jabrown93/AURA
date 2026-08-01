package routes_auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/jwtauth/v5"
	"golang.org/x/oauth2"
)

const (
	// oidcTxCookieName holds the in-flight authorization request.
	oidcTxCookieName = "aura_oidc_tx"

	// oidcTxTTL bounds how long a user may take to authenticate at the provider.
	oidcTxTTL = 10 * time.Minute

	// tokenTypeOIDCTx marks a transaction token so it can never be replayed as a session.
	tokenTypeOIDCTx = "oidc_tx"
)

var errNoOIDCTransaction = errors.New("no OIDC transaction in progress")

// oidcTransaction is the per-sign-in state that has to survive the round trip to the
// provider: the CSRF state, the ID token nonce, and the PKCE code verifier.
type oidcTransaction struct {
	State    string
	Nonce    string
	Verifier string
}

// startOIDCTransaction mints a transaction and stores it in a short-lived signed cookie.
// Keeping it in the cookie rather than in server memory means sign-in survives a restart
// and needs no cleanup of abandoned attempts.
func startOIDCTransaction(w http.ResponseWriter, r *http.Request) (*oidcTransaction, error) {
	if TokenAuth == nil {
		return nil, ErrTokenAuthNotConfigured
	}

	state, err := randomToken()
	if err != nil {
		return nil, err
	}
	nonce, err := randomToken()
	if err != nil {
		return nil, err
	}

	tx := &oidcTransaction{
		State:    state,
		Nonce:    nonce,
		Verifier: oauth2.GenerateVerifier(),
	}

	now := time.Now()
	_, signed, err := TokenAuth.Encode(map[string]any{
		"sub":   "oidc-tx",
		"typ":   tokenTypeOIDCTx,
		"state": tx.State,
		"nonce": tx.Nonce,
		"cv":    tx.Verifier,
		"iat":   now.Unix(),
		"exp":   now.Add(oidcTxTTL).Unix(),
	})
	if err != nil {
		return nil, err
	}

	http.SetCookie(w, &http.Cookie{
		Name:     oidcTxCookieName,
		Value:    signed,
		Path:     "/",
		MaxAge:   int(oidcTxTTL.Seconds()),
		HttpOnly: true,
		// Lax, not Strict: the provider returns the user by cross-site navigation and
		// Strict would withhold the cookie exactly when it is needed.
		SameSite: http.SameSiteLaxMode,
		Secure:   RequestIsSecure(r),
	})

	return tx, nil
}

// consumeOIDCTransaction validates the callback against the stored transaction. The cookie
// is always cleared, so a transaction can only ever be used once.
func consumeOIDCTransaction(w http.ResponseWriter, r *http.Request, stateFromCallback string) (*oidcTransaction, error) {
	clearOIDCTransaction(w, r)

	if TokenAuth == nil {
		return nil, ErrTokenAuthNotConfigured
	}

	cookie, err := r.Cookie(oidcTxCookieName)
	if err != nil || cookie.Value == "" {
		return nil, errNoOIDCTransaction
	}

	token, err := jwtauth.VerifyToken(TokenAuth, cookie.Value)
	if err != nil || token == nil {
		return nil, errors.New("OIDC transaction is invalid or expired")
	}

	var typ string
	if err := token.Get("typ", &typ); err != nil || typ != tokenTypeOIDCTx {
		return nil, errors.New("OIDC transaction token has the wrong type")
	}

	tx := &oidcTransaction{}
	if err := token.Get("state", &tx.State); err != nil {
		return nil, errors.New("OIDC transaction is missing its state")
	}
	if err := token.Get("nonce", &tx.Nonce); err != nil {
		return nil, errors.New("OIDC transaction is missing its nonce")
	}
	if err := token.Get("cv", &tx.Verifier); err != nil {
		return nil, errors.New("OIDC transaction is missing its code verifier")
	}

	if subtle.ConstantTimeCompare([]byte(tx.State), []byte(stateFromCallback)) != 1 {
		return nil, errors.New("state does not match the request that started sign-in")
	}

	return tx, nil
}

func clearOIDCTransaction(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     oidcTxCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   RequestIsSecure(r),
	})
}

func randomToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
