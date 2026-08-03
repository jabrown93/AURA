package config

import (
	"aura/logging"
	"context"
	"fmt"
	"os"
	"path"

	"go.yaml.in/yaml/v3"
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

func LoadYAML(ctx context.Context) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Loading YAML configuration file", logging.LevelInfo)
	defer logAction.Complete()

	// Use an environment variable to determine the config path
	// By default, it will use /config
	// This is useful for testing and local development
	// In Docker, the config path is set to /config
	actionCheck := logAction.AddSubAction("Check for config.yaml file", logging.LevelTrace)
	defer actionCheck.Complete()

	yamlFileExtensions := []string{"yaml", "yml"}
	yamlFileExists := false
	var yamlFile string
	for _, ext := range yamlFileExtensions {
		yamlFile = path.Join(ConfigPath, fmt.Sprintf("config.%s", ext))
		if _, err := os.Stat(yamlFile); err == nil {
			yamlFileExists = true
			break
		} else {
			logging.LOGGER.Warn().Timestamp().Str("path", yamlFile).Msgf("Config file with extension .%s not found", ext)
		}
	}
	if !yamlFileExists {
		createBaseConfigErr := createBaseConfig(ctx)
		if !createBaseConfigErr {
			logAction.SetError("Failed to create base config.yaml file", "An error occurred while creating the base config.yaml file. Please check the logs for more details.", nil)
			return
		}
		logging.LOGGER.Info().Timestamp().Str("path", yamlFile).Msg("Base config.yaml file created successfully")
	}
	actionCheck.Complete()

	actionRead := logAction.AddSubAction("Read config.yaml file", logging.LevelTrace)
	data := []byte{}
	data, err := os.ReadFile(yamlFile)
	if err != nil {
		actionRead.SetError(fmt.Sprintf("failed to read config.yaml file: %s", err.Error()), err.Error(), nil)
		return
	}
	actionRead.Complete()

	// Sub-action: Parse config.yaml file
	var config Config
	actionParse := logAction.AddSubAction("Parse config.yaml file", logging.LevelTrace)
	if err := yaml.Unmarshal(data, &config); err != nil {
		actionParse.SetError(fmt.Sprintf("failed to parse config.yaml file: %s", err.Error()), err.Error(), nil)
		return
	}
	actionParse.Complete()

	Current = config
	Loaded = true
}

func createBaseConfig(ctx context.Context) (success bool) {
	// Create a base config with default values
	ctx, logAction := logging.AddSubActionToContext(ctx, "Creating base configuration", logging.LevelInfo)
	defer logAction.Complete()

	// Set default values for the config
	base := DefaultConfig()

	Err := base.Save(ctx)
	if Err.Message != "" {
		return false
	}
	return true
}
