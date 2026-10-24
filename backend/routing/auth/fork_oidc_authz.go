package routes_auth

import (
	"aura/config"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
)

// forkOIDCDenied returns why a verified ID token may not sign in, or "" if it may.
// Fork: upstream reads email_verified but never checks it, so an IdP that lets users set an
// arbitrary unverified email would pass the email allowlist.
func forkOIDCDenied(idToken *oidc.IDToken, email string, emailVerified bool) string {
	cfg := config.Current.Auth.OIDC
	emailAllowlist := len(cfg.AllowedEmails) > 0 || len(cfg.AllowedDomains) > 0
	if emailAllowlist && email != "" && !emailVerified {
		return "The IdP did not mark the email as verified, so it can't satisfy the email allowlist"
	}
	if len(cfg.AllowedGroups) == 0 {
		return ""
	}
	claim := cfg.GroupsClaim
	if claim == "" {
		claim = "groups"
	}
	var claims map[string]any
	if err := idToken.Claims(&claims); err != nil {
		return "Failed to read the groups claim"
	}
	if !hasAllowedGroup(claims[claim], cfg.AllowedGroups) {
		return "The user is not in any of the allowed groups"
	}
	return ""
}

// hasAllowedGroup accepts a claim that is a list of strings or a single string.
func hasAllowedGroup(raw any, allowed []string) bool {
	var groups []string
	switch v := raw.(type) {
	case string:
		groups = []string{v}
	case []any:
		for _, g := range v {
			if s, ok := g.(string); ok {
				groups = append(groups, s)
			}
		}
	}
	for _, g := range groups {
		for _, a := range allowed {
			if strings.EqualFold(strings.TrimSpace(g), strings.TrimSpace(a)) {
				return true
			}
		}
	}
	return false
}
