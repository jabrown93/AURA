package config

import (
	"context"

	"go.yaml.in/yaml/v3"
)

// Fork: a deployment that re-imports a Git-owned config.yaml on every start (AURA_IMPORT_CONFIG)
// would otherwise reset the preferences set in the UI, like favorite creators and saved page
// views, each restart. Unless the file sets UserPreferences itself, the stored ones are kept.
func keepStoredUserPreferences(ctx context.Context, data []byte, cfg *Config) bool {
	var top map[string]any
	if yaml.Unmarshal(data, &top) == nil {
		if _, ok := top["UserPreferences"]; ok {
			return false
		}
	}
	stored, Err := store.GetAllConfigValues(ctx)
	if Err.Message != "" || len(stored) == 0 {
		return false
	}
	storedCfg, _, _ := ConfigFromValues(stored)
	cfg.UserPreferences = storedCfg.UserPreferences
	return true
}

// migrateLegacyAutoDownload carries an older config's top-level AutoDownload into Jobs.AutoDownload
// when the file doesn't set the new key. Fork: otherwise a disabled or rescheduled auto-download
// silently comes back on at the default schedule after the upgrade.
func migrateLegacyAutoDownload(data []byte, cfg *Config) {
	var older struct {
		AutoDownload *struct {
			Enabled *bool  `yaml:"Enabled"`
			Cron    string `yaml:"Cron"`
		} `yaml:"AutoDownload"`
	}
	if yaml.Unmarshal(data, &older) != nil || older.AutoDownload == nil {
		return
	}
	job := &cfg.Jobs.AutoDownload
	if job.Enabled != nil || job.Cron != "" {
		return
	}
	job.Enabled = older.AutoDownload.Enabled
	job.Cron = older.AutoDownload.Cron
}
