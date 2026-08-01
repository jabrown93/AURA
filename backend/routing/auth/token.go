package routes_auth

import (
	"errors"
	"time"
)

// ErrTokenAuthNotConfigured is returned when a token is requested before the signing key is loaded.
var ErrTokenAuthNotConfigured = errors.New("authentication is not configured")

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
	claims["iat"] = now.Unix()
	claims["exp"] = now.Add(SessionTTL).Unix()

	_, signedToken, err := TokenAuth.Encode(claims)
	if err != nil {
		return "", err
	}
	return signedToken, nil
}
