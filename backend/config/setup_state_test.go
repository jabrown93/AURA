package config

import "testing"

func TestIsSetUp(t *testing.T) {
	if DefaultConfig().IsSetUp() {
		t.Error("the default config should not count as set up")
	}
	cfg := DefaultConfig()
	cfg.Mediux.ApiToken = "token"
	if !cfg.IsSetUp() {
		t.Error("a config with a MediUX token should count as set up")
	}
}
