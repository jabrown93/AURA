package config

import "slices"

const (
	// DefaultOIDCGroupsClaim is the claim most providers use for group membership.
	DefaultOIDCGroupsClaim = "groups"

	// DefaultOIDCButtonLabel is shown on the login page when no label is configured.
	DefaultOIDCButtonLabel = "Sign in with SSO"
)

// defaultOIDCScopes are requested when the config does not name any. "profile" and "email"
// are what the allowlist checks read.
var defaultOIDCScopes = []string{"openid", "profile", "email"}

// GroupsClaimOrDefault returns the ID token claim to read group membership from.
func (o Config_OIDC) GroupsClaimOrDefault() string {
	if o.GroupsClaim == "" {
		return DefaultOIDCGroupsClaim
	}
	return o.GroupsClaim
}

// ButtonLabelOrDefault returns the label for the login page's SSO button.
func (o Config_OIDC) ButtonLabelOrDefault() string {
	if o.ButtonLabel == "" {
		return DefaultOIDCButtonLabel
	}
	return o.ButtonLabel
}

// ScopesOrDefault returns the scopes to request, always including "openid" since the
// provider will not issue an ID token without it.
func (o Config_OIDC) ScopesOrDefault() []string {
	if len(o.Scopes) == 0 {
		return slices.Clone(defaultOIDCScopes)
	}
	scopes := slices.Clone(o.Scopes)
	if !slices.Contains(scopes, "openid") {
		scopes = append([]string{"openid"}, scopes...)
	}
	return scopes
}

// HasAllowlist reports whether any authorization constraint is configured. When false, any
// user the provider authenticates is allowed in.
func (o Config_OIDC) HasAllowlist() bool {
	return len(o.AllowedGroups) > 0 || len(o.AllowedEmails) > 0 || len(o.AllowedSubjects) > 0
}
