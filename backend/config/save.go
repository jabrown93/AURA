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
	if writeErr := os.WriteFile(path.Join(ConfigPath, "config.yaml"), data, 0644); writeErr != nil {
		subActionWrite.SetError("Failed to write config to file", writeErr.Error(), nil)
		logAction.Status = logging.StatusError
		return *subActionWrite.Error
	}
	subActionWrite.Complete()

	return logging.LogErrorInfo{}
}
