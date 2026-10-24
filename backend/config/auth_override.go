package config

import (
	"os"
	"strings"
)

// Environment variable to turn off authentication when locked out
const DisableAuthEnv = "AURA_DISABLE_AUTH"

// AuthDisabledByEnv reports whether authentication is turned off with AURA_DISABLE_AUTH
func AuthDisabledByEnv() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv(DisableAuthEnv)), "true")
}

// AuthEnabled reports whether requests must be authenticated.
func AuthEnabled() bool {
	return Current.Auth.Enabled && !AuthDisabledByEnv()
}
