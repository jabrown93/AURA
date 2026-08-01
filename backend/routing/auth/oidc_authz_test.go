package routes_auth

import (
	"aura/config"
	"testing"
)

func TestExtractIdentity(t *testing.T) {
	tests := []struct {
		name        string
		claims      map[string]any
		groupsClaim string
		wantSubject string
		wantEmail   string
		wantGroups  []string
	}{
		{
			name:        "list of groups",
			claims:      map[string]any{"sub": "abc", "email": "user@example.com", "groups": []any{"admins", "users"}},
			groupsClaim: "groups",
			wantSubject: "abc",
			wantEmail:   "user@example.com",
			wantGroups:  []string{"admins", "users"},
		},
		{
			name:        "single group as a bare string",
			claims:      map[string]any{"sub": "abc", "groups": "admins"},
			groupsClaim: "groups",
			wantSubject: "abc",
			wantGroups:  []string{"admins"},
		},
		{
			name:        "custom groups claim",
			claims:      map[string]any{"sub": "abc", "roles": []any{"aura-admin"}},
			groupsClaim: "roles",
			wantSubject: "abc",
			wantGroups:  []string{"aura-admin"},
		},
		{
			name:        "non-string entries are skipped",
			claims:      map[string]any{"sub": "abc", "groups": []any{"admins", 42, nil}},
			groupsClaim: "groups",
			wantSubject: "abc",
			wantGroups:  []string{"admins"},
		},
		{
			name:        "missing groups claim",
			claims:      map[string]any{"sub": "abc"},
			groupsClaim: "groups",
			wantSubject: "abc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractIdentity(tt.claims, tt.groupsClaim)

			if got.Subject != tt.wantSubject {
				t.Errorf("Subject = %q, want %q", got.Subject, tt.wantSubject)
			}
			if got.Email != tt.wantEmail {
				t.Errorf("Email = %q, want %q", got.Email, tt.wantEmail)
			}
			if len(got.Groups) != len(tt.wantGroups) {
				t.Fatalf("Groups = %v, want %v", got.Groups, tt.wantGroups)
			}
			for i, group := range tt.wantGroups {
				if got.Groups[i] != group {
					t.Errorf("Groups[%d] = %q, want %q", i, got.Groups[i], group)
				}
			}
		})
	}
}

func TestAuthorizeIdentity(t *testing.T) {
	identity := oidcIdentity{
		Subject: "8f14e45f",
		Email:   "user@example.com",
		Groups:  []string{"media", "admins"},
	}

	tests := []struct {
		name      string
		cfg       config.Config_OIDC
		identity  oidcIdentity
		wantAllow bool
	}{
		{
			name:      "no allowlist defers to the provider",
			cfg:       config.Config_OIDC{},
			identity:  identity,
			wantAllow: true,
		},
		{
			name:      "matching group",
			cfg:       config.Config_OIDC{AllowedGroups: []string{"admins"}},
			identity:  identity,
			wantAllow: true,
		},
		{
			name:      "matching email is case insensitive",
			cfg:       config.Config_OIDC{AllowedEmails: []string{"User@Example.com"}},
			identity:  identity,
			wantAllow: true,
		},
		{
			name:      "matching subject",
			cfg:       config.Config_OIDC{AllowedSubjects: []string{"8f14e45f"}},
			identity:  identity,
			wantAllow: true,
		},
		{
			name:      "any one list may match",
			cfg:       config.Config_OIDC{AllowedGroups: []string{"nobody"}, AllowedEmails: []string{"user@example.com"}},
			identity:  identity,
			wantAllow: true,
		},
		{
			name:      "no match is denied",
			cfg:       config.Config_OIDC{AllowedGroups: []string{"nobody"}},
			identity:  identity,
			wantAllow: false,
		},
		{
			name:      "empty claims never satisfy an allowlist",
			cfg:       config.Config_OIDC{AllowedEmails: []string{""}},
			identity:  oidcIdentity{Subject: "abc"},
			wantAllow: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := authorizeIdentity(tt.cfg, tt.identity)
			if (err == nil) != tt.wantAllow {
				t.Fatalf("authorizeIdentity() error = %v, wantAllow %v", err, tt.wantAllow)
			}
		})
	}
}
