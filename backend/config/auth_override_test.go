package config

import "testing"

func TestAuthEnabled(t *testing.T) {
	previous := Current
	t.Cleanup(func() { Current = previous })

	Current.Auth.Enabled = true
	t.Setenv(DisableAuthEnv, "")
	if !AuthEnabled() {
		t.Error("AuthEnabled() = false, want true when enabled in the config")
	}

	// AURA_DISABLE_AUTH turns authentication off without changing the config
	t.Setenv(DisableAuthEnv, "TRUE")
	if AuthEnabled() || !AuthDisabledByEnv() {
		t.Error("AURA_DISABLE_AUTH=true should turn authentication off")
	}
	if !Current.Auth.Enabled {
		t.Error("AURA_DISABLE_AUTH should not change the config")
	}

	Current.Auth.Enabled = false
	t.Setenv(DisableAuthEnv, "")
	if AuthEnabled() {
		t.Error("AuthEnabled() = true, want false when disabled in the config")
	}
}
