package config

import (
	"aura/logging"
	"context"
	"sync"
)

// UpdateLock makes sure only one config update runs at a time, so updates can't overwrite each other
var UpdateLock sync.Mutex

func (config *Config) Save(ctx context.Context) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Saving Config to Database", logging.LevelDebug)
	defer logAction.Complete()

	if store == nil {
		logAction.SetError("Config store not initialized", "The database must be initialized before saving the config", nil)
		return *logAction.Error
	}

	// Copy the main media server into MediaServers
	config.SyncMediaServers()

	// Sub-action: Convert config to values
	// The User ID is not saved, as it is retrieved from the media server on startup
	subActionConvert := logAction.AddSubAction("Convert Config to Values", logging.LevelTrace)
	values, err := config.ToValues()
	if err != nil {
		subActionConvert.SetError("Failed to convert config to values", err.Error(), nil)
		return *subActionConvert.Error
	}
	subActionConvert.Complete()

	// Sub-action: Save values to the database
	subActionSave := logAction.AddSubAction("Save Config Values to Database", logging.LevelTrace)
	Err = store.SaveConfigValues(ctx, values, false)
	if Err.Message != "" {
		subActionSave.SetError("Failed to save config to database", Err.Message, Err.Detail)
		return *subActionSave.Error
	}
	subActionSave.Complete()

	return logging.LogErrorInfo{}
}
