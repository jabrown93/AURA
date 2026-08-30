package config

import (
	"aura/logging"
	"context"
	"os"
	"path"

	"go.yaml.in/yaml/v3"
)

func (config *Config) Save(ctx context.Context) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Saving Config to File", logging.LevelDebug)
	defer logAction.Complete()

	// Clear the User ID before saving
	// This is done so that it is loaded on startup
	config.MediaServer.UserID = ""

	// Sub-action: Marshal config to YAML
	subActionMarshal := logAction.AddSubAction("Marshal Config to YAML", logging.LevelTrace)
	data, marshalErr := yaml.Marshal(config)
	if marshalErr != nil {
		subActionMarshal.SetError("Failed to marshal config to YAML", marshalErr.Error(), nil)
		logAction.Status = logging.StatusError
		return *subActionMarshal.Error
	}
	subActionMarshal.Complete()

	// Sub-action: Write config to file
	subActionWrite := logAction.AddSubAction("Write Config to File", logging.LevelTrace)
	configFile := path.Join(ConfigPath, "config.yaml")
	if writeErr := os.WriteFile(configFile, data, 0o600); writeErr != nil {
		subActionWrite.SetError("Failed to write config to file", writeErr.Error(), nil)
		logAction.Status = logging.StatusError
		return *subActionWrite.Error
	}
	// WriteFile only applies its mode when creating the file, so an existing config keeps
	// whatever mode it already had. Not fatal: /config is often a network share where
	// chmod is unsupported.
	if chmodErr := os.Chmod(configFile, 0o600); chmodErr != nil {
		logging.LOGGER.Warn().Timestamp().Str("error", chmodErr.Error()).Msg("Could not restrict config.yaml permissions")
	}
	subActionWrite.Complete()

	return logging.LogErrorInfo{}
}
