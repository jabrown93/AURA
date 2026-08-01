package config

import (
	"aura/logging"
	"context"
	"testing"
)

// testContext mirrors how handlers set up a logging context before calling validation;
// the validators log into the current action and need one to exist.
func testContext() context.Context {
	ctx, ld := logging.CreateLoggingContext(context.Background(), "config validation test")
	return logging.WithCurrentAction(ctx, ld.AddAction("Validate", logging.LevelTrace))
}

// argon2idHash is a valid hash of an arbitrary password, used to exercise the password branch.
const argon2idHash = "$argon2id$v=19$m=65536,t=1,p=2$c29tZXNhbHR2YWx1ZQ$Zm9vYmFyYmF6cXV4Zm9vYmFyYmF6cXV4Zm9vYmFyYmF6cXV4MDA"

func validOIDC() Config_OIDC {
	return Config_OIDC{
		Enabled:      true,
		IssuerURL:    "https://auth.example.com/application/o/aura/",
		ClientID:     "aura",
		ClientSecret: "secret",
		RedirectURL:  "https://aura.example.com/api/auth/oidc/callback",
	}
}

func TestValidateAuth(t *testing.T) {
	tests := []struct {
		name string
		auth Config_Auth
		want bool
	}{
		{
			name: "auth disabled needs nothing",
			auth: Config_Auth{Enabled: false},
			want: true,
		},
		{
			name: "password only",
			auth: Config_Auth{Enabled: true, Password: argon2idHash},
			want: true,
		},
		{
			name: "oidc only",
			auth: Config_Auth{Enabled: true, OIDC: validOIDC()},
			want: true,
		},
		{
			name: "password and oidc together",
			auth: Config_Auth{Enabled: true, Password: argon2idHash, OIDC: validOIDC()},
			want: true,
		},
		{
			name: "auth enabled with no method locks everyone out",
			auth: Config_Auth{Enabled: true},
			want: false,
		},
		{
			name: "password that is not an argon2id hash",
			auth: Config_Auth{Enabled: true, Password: "hunter2"},
			want: false,
		},
		{
			name: "oidc enabled but not configured",
			auth: Config_Auth{Enabled: true, OIDC: Config_OIDC{Enabled: true}},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auth := tt.auth
			if got := ValidateAuth(testContext(), &auth); got != tt.want {
				t.Fatalf("ValidateAuth() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateOIDC(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Config_OIDC)
		want   bool
	}{
		{name: "fully configured", mutate: func(*Config_OIDC) {}, want: true},
		{name: "disabled is always valid", mutate: func(o *Config_OIDC) { *o = Config_OIDC{} }, want: true},
		{name: "missing issuer", mutate: func(o *Config_OIDC) { o.IssuerURL = "" }, want: false},
		{name: "missing client id", mutate: func(o *Config_OIDC) { o.ClientID = "" }, want: false},
		{name: "missing client secret", mutate: func(o *Config_OIDC) { o.ClientSecret = "" }, want: false},
		{name: "missing redirect", mutate: func(o *Config_OIDC) { o.RedirectURL = "" }, want: false},
		{name: "issuer without a scheme", mutate: func(o *Config_OIDC) { o.IssuerURL = "auth.example.com" }, want: false},
		{name: "relative redirect", mutate: func(o *Config_OIDC) { o.RedirectURL = "/api/auth/oidc/callback" }, want: false},
		{name: "whitespace only value", mutate: func(o *Config_OIDC) { o.ClientID = "   " }, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oidc := validOIDC()
			tt.mutate(&oidc)
			if got := ValidateOIDC(testContext(), &oidc); got != tt.want {
				t.Fatalf("ValidateOIDC() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOIDCDefaults(t *testing.T) {
	empty := Config_OIDC{}

	if empty.GroupsClaimOrDefault() != DefaultOIDCGroupsClaim {
		t.Errorf("GroupsClaimOrDefault() = %q, want %q", empty.GroupsClaimOrDefault(), DefaultOIDCGroupsClaim)
	}
	if empty.ButtonLabelOrDefault() != DefaultOIDCButtonLabel {
		t.Errorf("ButtonLabelOrDefault() = %q, want %q", empty.ButtonLabelOrDefault(), DefaultOIDCButtonLabel)
	}
	if empty.HasAllowlist() {
		t.Error("an unconfigured OIDC block must not report an allowlist")
	}

	// Without "openid" the provider will not issue an ID token at all.
	scopes := Config_OIDC{Scopes: []string{"profile"}}.ScopesOrDefault()
	if len(scopes) != 2 || scopes[0] != "openid" || scopes[1] != "profile" {
		t.Fatalf("ScopesOrDefault() = %v, want openid to be added", scopes)
	}
}
