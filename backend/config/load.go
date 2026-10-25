package config

import (
	"aura/logging"
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.yaml.in/yaml/v3"
)

// Store saves and loads the config values
type Store interface {
	// Get all stored config values
	GetAllConfigValues(ctx context.Context) (values map[string]string, Err logging.LogErrorInfo)

	// Save the config values. Only changed values are written.
	// If replace is true, any stored value not in values is removed.
	SaveConfigValues(ctx context.Context, values map[string]string, replace bool) (Err logging.LogErrorInfo)
}

var store Store

func SetStore(s Store) {
	store = s
}

const (
	// Database file path, relative to the config path. Defaults to AURA.db.
	DatabasePathEnv = "DATABASE_PATH"

	// Set to true to import config.yaml again, replacing the stored config
	ImportConfigEnv = "AURA_IMPORT_CONFIG"

	defaultDatabaseFile = "AURA.db"
)

func init() {
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "/config"
	}
	ConfigPath = configPath

	// Check if the config path exists
	if _, err := os.Stat(ConfigPath); os.IsNotExist(err) {
		err := os.MkdirAll(ConfigPath, os.ModePerm)
		if err != nil {
			panic(fmt.Sprintf("failed to create config directory: %s", err.Error()))
		}
	}
}

// Load loads the config from the database.
// On first start, config.yaml is imported and renamed to config.yaml.imported.
// After that, config.yaml is ignored unless AURA_IMPORT_CONFIG is true.
func Load(ctx context.Context) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Loading Configuration", logging.LevelInfo)
	defer logAction.Complete()

	Loaded = false

	if store == nil {
		logAction.SetError("Config store not initialized", "The database must be initialized before loading the config", nil)
		return
	}

	// Sub-action: Get the stored config values
	actionGet := logAction.AddSubAction("Get config values from database", logging.LevelTrace)
	values, Err := store.GetAllConfigValues(ctx)
	if Err.Message != "" {
		actionGet.SetError("Failed to get config values from database", Err.Message, Err.Detail)
		return
	}
	actionGet.Complete()

	// Check for a config.yaml file to import
	yamlFile, yamlFileExists := findConfigYAMLFile()
	if yamlFileExists {
		reimport := strings.EqualFold(os.Getenv(ImportConfigEnv), "true")
		switch {
		case len(values) == 0:
			logging.LOGGER.Info().Timestamp().Str("path", yamlFile).Msg("Importing config file into the database")
			importConfigYAMLFile(ctx, yamlFile, false)
			return
		case reimport:
			logging.LOGGER.Warn().Timestamp().Str("path", yamlFile).Msgf("%s is true, replacing the config in the database with the config file", ImportConfigEnv)
			importConfigYAMLFile(ctx, yamlFile, true)
			return
		default:
			logAction.AppendWarning("message", fmt.Sprintf("Config file '%s' is ignored, as the config is stored in the database. Set %s=true to import it.", yamlFile, ImportConfigEnv))
			logging.LOGGER.Warn().Timestamp().Str("path", yamlFile).
				Msgf("Config file is ignored, as the config is stored in the database. Set %s=true to import it, replacing the current config.", ImportConfigEnv)
		}
	}

	// If there is no config yet, save the default config
	if len(values) == 0 {
		actionDefault := logAction.AddSubAction("Save default config", logging.LevelInfo)
		base := DefaultConfig()
		Err := base.Save(ctx)
		if Err.Message != "" {
			actionDefault.SetError("Failed to save default config", Err.Message, Err.Detail)
			return
		}
		actionDefault.Complete()
		logging.LOGGER.Info().Timestamp().Msg("Default config saved to the database")
		Current = base
		Loaded = true
		return
	}

	// Sub-action: Build the config from the stored values
	actionBuild := logAction.AddSubAction("Build config from database values", logging.LevelTrace)
	cfg, decodeErrors, unknownKeys := ConfigFromValues(values)
	for key, decodeErr := range decodeErrors {
		actionBuild.AppendWarning(key, fmt.Sprintf("Invalid stored value, using the default value: %s", decodeErr))
		logging.LOGGER.Warn().Timestamp().Str("key", key).Str("error", decodeErr).Msg("Invalid stored config value, using the default value")
	}
	if len(unknownKeys) > 0 {
		logging.LOGGER.Debug().Timestamp().Strs("keys", unknownKeys).Msg("Ignoring stored config values that are no longer used")
	}
	cfg.LoadMainMediaServer()
	actionBuild.Complete()

	Current = cfg
	Loaded = true
}

// importConfigYAMLFile imports a config.yaml file into the database and renames it
func importConfigYAMLFile(ctx context.Context, yamlFile string, replace bool) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Importing config file", logging.LevelInfo)
	defer logAction.Complete()

	// Sub-action: Read and parse the config file
	actionRead := logAction.AddSubAction("Read config file", logging.LevelTrace)
	data, err := os.ReadFile(yamlFile)
	if err != nil {
		actionRead.SetError("Failed to read config file", err.Error(), map[string]any{"path": yamlFile})
		return
	}
	cfg, err := ParseConfigYAML(data)
	if err != nil {
		actionRead.SetError("Failed to parse config file", err.Error(), map[string]any{"path": yamlFile})
		return
	}
	actionRead.Complete()

	// Sub-action: Save the config to the database
	actionSave := logAction.AddSubAction("Save config to database", logging.LevelTrace)
	if replace {
		keepStoredUserPreferences(ctx, data, &cfg) // Fork
	}
	values, err := cfg.ToValues()
	if err != nil {
		actionSave.SetError("Failed to convert config file", err.Error(), map[string]any{"path": yamlFile})
		return
	}
	Err := store.SaveConfigValues(ctx, values, replace)
	if Err.Message != "" {
		actionSave.SetError("Failed to save config to database", Err.Message, Err.Detail)
		return
	}
	actionSave.Complete()

	Current = cfg
	Loaded = true

	// Rename the config file so it isn't imported again
	importedFile := importedConfigYAMLFileName(yamlFile)
	if err := os.Rename(yamlFile, importedFile); err != nil {
		logAction.AppendWarning("rename_error", err.Error())
		logging.LOGGER.Warn().Timestamp().Str("path", yamlFile).Str("error", err.Error()).Msg("Config file imported, but could not be renamed")
		return
	}
	logging.LOGGER.Info().Timestamp().Str("path", importedFile).Msg("Config file imported into the database and renamed")
}

// findConfigYAMLFile returns the path of config.yaml or config.yml, if it exists
func findConfigYAMLFile() (yamlFile string, exists bool) {
	for _, ext := range []string{"yaml", "yml"} {
		yamlFile = path.Join(ConfigPath, fmt.Sprintf("config.%s", ext))
		if _, err := os.Stat(yamlFile); err == nil {
			return yamlFile, true
		}
	}
	return "", false
}

// importedConfigYAMLFileName returns the name to rename an imported config file to.
// The first import is config.yaml.imported, later imports add a timestamp.
func importedConfigYAMLFileName(yamlFile string) string {
	importedFile := yamlFile + ".imported"
	if _, err := os.Stat(importedFile); err == nil {
		importedFile = fmt.Sprintf("%s.%s", importedFile, time.Now().Format("20060102_150405"))
	}
	return importedFile
}

var databasePathCache struct {
	mu        sync.Mutex
	key       string
	path      string
	warnedFor string
}

// DatabasePath returns the path of the SQLite database file.
// Uses DATABASE_PATH, then Database.Path from an older config.yaml, then AURA.db in the config path.
func DatabasePath() string {
	envPath := os.Getenv(DatabasePathEnv)

	databasePathCache.mu.Lock()
	defer databasePathCache.mu.Unlock()
	cacheKey := ConfigPath + "\x00" + envPath
	if databasePathCache.key == cacheKey {
		return databasePathCache.path
	}

	dbPath := path.Join(ConfigPath, defaultDatabaseFile)
	if envPath != "" {
		dbPath = envPath
		if !filepath.IsAbs(envPath) {
			dbPath = path.Join(ConfigPath, envPath)
		}
	} else if legacyPath, yamlFile := legacyDatabasePath(); legacyPath != "" {
		dbPath = path.Join(ConfigPath, legacyPath)
		if databasePathCache.warnedFor != dbPath {
			logging.LOGGER.Warn().Timestamp().Str("path", dbPath).Str("config_file", yamlFile).
				Msgf("Using the database path from the config file. Set the %s environment variable to '%s', as config files are no longer used.", DatabasePathEnv, legacyPath)
			databasePathCache.warnedFor = dbPath
		}
	}

	databasePathCache.key = cacheKey
	databasePathCache.path = dbPath
	return dbPath
}

// legacyDatabasePath returns Database.Path from an older config.yaml or config.yaml.imported
func legacyDatabasePath() (dbPath string, yamlFile string) {
	candidates := []string{}
	for _, ext := range []string{"yaml", "yml"} {
		candidates = append(candidates, path.Join(ConfigPath, fmt.Sprintf("config.%s", ext)))
	}
	for _, ext := range []string{"yaml", "yml"} {
		candidates = append(candidates, path.Join(ConfigPath, fmt.Sprintf("config.%s.imported", ext)))
	}

	for _, candidate := range candidates {
		data, err := os.ReadFile(candidate)
		if err != nil {
			continue
		}
		var legacy struct {
			Database struct {
				Path string `yaml:"Path"`
			} `yaml:"Database"`
		}
		if err := yaml.Unmarshal(data, &legacy); err != nil {
			continue
		}
		if legacy.Database.Path != "" && legacy.Database.Path != defaultDatabaseFile {
			return legacy.Database.Path, candidate
		}
	}
	return "", ""
}
