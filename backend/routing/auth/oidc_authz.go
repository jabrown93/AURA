package routes_auth

import (
	"aura/config"
	"encoding/json"
	"fmt"
	"strings"
)

// oidcIdentity is the part of an ID token AURA makes decisions on.
type oidcIdentity struct {
	Subject string
	Email   string
	Groups  []string
}

// extractIdentity pulls the identity out of raw ID token claims. The groups claim name is
// configurable because providers disagree on it, and its value may be a list or a single
// string.
func extractIdentity(claims map[string]any, groupsClaim string) oidcIdentity {
	identity := oidcIdentity{}

	if sub, ok := claims["sub"].(string); ok {
		identity.Subject = sub
	}
	if email, ok := claims["email"].(string); ok {
		identity.Email = email
	}

	switch raw := claims[groupsClaim].(type) {
	case string:
		if raw != "" {
			identity.Groups = []string{raw}
		}
	case []any:
		for _, item := range raw {
			if group, ok := item.(string); ok && group != "" {
				identity.Groups = append(identity.Groups, group)
			}
		}
	case []string:
		identity.Groups = raw
	case json.Number:
		identity.Groups = []string{raw.String()}
	}

	return identity
}

// authorizeIdentity applies the configured allowlists.
//
// With no list configured, any user the provider authenticates is allowed in and access is
// governed by the provider's own application policy. With any list configured, the user
// must match at least one entry in at least one list.
func authorizeIdentity(cfg config.Config_OIDC, identity oidcIdentity) error {
	if !cfg.HasAllowlist() {
		return nil
	}

	if containsFold(cfg.AllowedSubjects, identity.Subject) {
		return nil
	}
	if containsFold(cfg.AllowedEmails, identity.Email) {
		return nil
	}
	for _, group := range identity.Groups {
		if containsFold(cfg.AllowedGroups, group) {
			return nil
		}
	}

	return fmt.Errorf("user %q is not on any allowlist", identityLabel(identity))
}

// identityLabel names the user for logs, preferring email over the opaque subject.
func identityLabel(identity oidcIdentity) string {
	if identity.Email != "" {
		return identity.Email
	}
	return identity.Subject
}

func containsFold(haystack []string, needle string) bool {
	if needle == "" {
		return false
	}
	for _, candidate := range haystack {
		if strings.EqualFold(strings.TrimSpace(candidate), needle) {
			return true
		}
	}
	return false
}
