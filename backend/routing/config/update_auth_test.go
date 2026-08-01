package routes_config

import (
	"aura/config"
	"aura/logging"
	"context"
	"testing"
)

const argon2idHash = "$argon2id$v=19$m=65536,t=1,p=2$c29tZXNhbHR2YWx1ZQ$Zm9vYmFyYmF6cXV4Zm9vYmFyYmF6cXV4Zm9vYmFyYmF6cXV4MDA"

func testContext() context.Context {
	ctx, ld := logging.CreateLoggingContext(context.Background(), "config update test")
	return logging.WithCurrentAction(ctx, ld.AddAction("Update", logging.LevelTrace))
}

// The UI is served masked secrets. Sending one back means "leave it alone" - taking it at
// face value would overwrite the stored secret with its own mask.
func TestCheckConfigDifferencesAuthKeepsMaskedSecrets(t *testing.T) {
	oldAuth := config.Config_Auth{
		Enabled:  true,
		Password: argon2idHash,
		OIDC: config.Config_OIDC{
			Enabled:      true,
			IssuerURL:    "https://auth.example.com/application/o/aura/",
			ClientID:     "aura",
			ClientSecret: "super-secret-value",
			RedirectURL:  "https://aura.example.com/api/auth/oidc/callback",
		},
	}

	newAuth := oldAuth
	newAuth.Password = config.MaskToken(oldAuth.Password)
	newAuth.OIDC.ClientSecret = config.MaskToken(oldAuth.OIDC.ClientSecret)

	changed, valid := checkConfigDifferences_Auth(testContext(), oldAuth, &newAuth)

	if newAuth.Password != argon2idHash {
		t.Errorf("password = %q, want the stored hash to be preserved", newAuth.Password)
	}
	if newAuth.OIDC.ClientSecret != "super-secret-value" {
		t.Errorf("client secret = %q, want the stored secret to be preserved", newAuth.OIDC.ClientSecret)
	}
	if changed {
		t.Error("re-sending masked secrets must not count as a change")
	}
	if !valid {
		t.Error("config must stay valid when masked secrets are restored")
	}
}

func TestCheckConfigDifferencesAuthAcceptsNewSecrets(t *testing.T) {
	oldAuth := config.Config_Auth{Enabled: true, Password: argon2idHash}

	replacement := "$argon2id$v=19$m=65536,t=1,p=2$YW5vdGhlcnNhbHR2YWw$Zm9vYmFyYmF6cXV4Zm9vYmFyYmF6cXV4Zm9vYmFyYmF6cXV4MDA"
	newAuth := oldAuth
	newAuth.Password = replacement

	changed, valid := checkConfigDifferences_Auth(testContext(), oldAuth, &newAuth)

	if newAuth.Password != replacement {
		t.Errorf("password = %q, want the new hash", newAuth.Password)
	}
	if !changed {
		t.Error("a new password must be reported as a change")
	}
	if !valid {
		t.Error("a valid argon2id hash must pass validation")
	}
}

// Clearing the field is how a user moves to OIDC-only, so it must not be confused with a
// masked value.
func TestCheckConfigDifferencesAuthAllowsClearingThePassword(t *testing.T) {
	oldAuth := config.Config_Auth{Enabled: true, Password: argon2idHash}

	newAuth := oldAuth
	newAuth.Password = ""
	newAuth.OIDC = config.Config_OIDC{
		Enabled:      true,
		IssuerURL:    "https://auth.example.com/application/o/aura/",
		ClientID:     "aura",
		ClientSecret: "super-secret-value",
		RedirectURL:  "https://aura.example.com/api/auth/oidc/callback",
	}

	changed, valid := checkConfigDifferences_Auth(testContext(), oldAuth, &newAuth)

	if newAuth.Password != "" {
		t.Errorf("password = %q, want it cleared", newAuth.Password)
	}
	if !changed || !valid {
		t.Errorf("changed = %v, valid = %v; want both true", changed, valid)
	}
}
