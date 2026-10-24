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
