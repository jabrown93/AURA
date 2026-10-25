package config

import (
	"context"
	"testing"

	"aura/logging"
)

type memStore struct{ values map[string]string }

func (m *memStore) GetAllConfigValues(context.Context) (map[string]string, logging.LogErrorInfo) {
	return m.values, logging.LogErrorInfo{}
}

func (m *memStore) SaveConfigValues(_ context.Context, values map[string]string, _ bool) logging.LogErrorInfo {
	m.values = values
	return logging.LogErrorInfo{}
}

func TestKeepStoredUserPreferences(t *testing.T) {
	old := store
	t.Cleanup(func() { store = old })
	saved := DefaultConfig()
	saved.UserPreferences.AutoUpgradeSets = true
	values, err := saved.ToValues()
	if err != nil {
		t.Fatal(err)
	}
	store = &memStore{values: values}

	cfg := DefaultConfig()
	if !keepStoredUserPreferences(context.Background(), []byte("MediaServer:\n  URL: http://new\n"), &cfg) {
		t.Fatal("stored preferences were not kept")
	}
	if !cfg.UserPreferences.AutoUpgradeSets {
		t.Error("AutoUpgradeSets not carried over")
	}

	cfg = DefaultConfig()
	if keepStoredUserPreferences(context.Background(), []byte("UserPreferences:\n  AutoUpgradeSets: false\n"), &cfg) {
		t.Error("preferences set in the file must win")
	}
}

func TestMigrateLegacyAutoDownload(t *testing.T) {
	cfg, err := ParseConfigYAML([]byte("AutoDownload:\n  Enabled: false\n  Cron: \"0 3 * * *\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	if j := cfg.Jobs.AutoDownload; j.Enabled == nil || *j.Enabled || j.Cron != "0 3 * * *" {
		t.Errorf("Jobs.AutoDownload = %+v, want disabled at 0 3 * * *", j)
	}
	cfg, _ = ParseConfigYAML([]byte("AutoDownload:\n  Enabled: false\nJobs:\n  AutoDownload:\n    Cron: \"0 1 * * *\"\n"))
	if j := cfg.Jobs.AutoDownload; j.Enabled != nil || j.Cron != "0 1 * * *" {
		t.Errorf("new key must win, got %+v", j)
	}
}

func TestRestoreMediaServerTokensBindsURL(t *testing.T) {
	current := []Config_MediaServer{{ID: "a", URL: "http://plex", ApiToken: "real"}, {ID: "b", URL: "http://jf", ApiToken: "real2"}}
	updated := []Config_MediaServer{{ID: "a", URL: "http://plex", ApiToken: "***eal"}, {ID: "b", URL: "http://evil", ApiToken: "***al2"}}
	RestoreMediaServerTokens(current, updated)
	if updated[0].ApiToken != "real" {
		t.Errorf("same URL: token = %q, want restored", updated[0].ApiToken)
	}
	if updated[1].ApiToken != "" {
		t.Errorf("changed URL: token = %q, want cleared", updated[1].ApiToken)
	}
}
