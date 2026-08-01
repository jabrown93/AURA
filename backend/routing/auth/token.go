package routes_auth

import (
	"errors"
	"time"
)

// TokenTypeSession marks tokens that grant access. Short-lived tokens minted for other
// purposes are signed with the same key, so the type has to be checked before one is
// accepted as a session.
const TokenTypeSession = "session"

// ErrTokenAuthNotConfigured is returned when a token is requested before the signing key is loaded.
var ErrTokenAuthNotConfigured = errors.New("authentication is not configured")

// IsSessionTokenType reports whether a token's "typ" claim permits use as a session.
// Tokens issued before the claim existed carry no type and stay valid until they expire.
func IsSessionTokenType(typ string) bool {
	return typ == "" || typ == TokenTypeSession
}

// IssueSessionToken mints the AURA JWT used for both password and OIDC sessions.
// extraClaims are merged in first so they can never overwrite the standard claims.
func IssueSessionToken(subject string, extraClaims map[string]any) (string, error) {
	if TokenAuth == nil {
		return "", ErrTokenAuthNotConfigured
	}

	claims := make(map[string]any, len(extraClaims)+3)
	for k, v := range extraClaims {
		claims[k] = v
	}

	now := time.Now()
	claims["sub"] = subject
	claims["typ"] = TokenTypeSession
	claims["iat"] = now.Unix()
	claims["exp"] = now.Add(SessionTTL).Unix()

	_, signedToken, err := TokenAuth.Encode(claims)
	if err != nil {
		return "", err
	}
	return signedToken, nil
}
