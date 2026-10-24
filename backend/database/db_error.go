package database

import (
	"aura/logging"
	"context"
)

// dbError logs a failed database call and returns its error
func dbError(ctx context.Context, action, message string, err error) logging.LogErrorInfo {
	_, logAction := logging.AddSubActionToContext(ctx, action, logging.LevelError)
	defer logAction.Complete()
	logAction.SetError(message, "", map[string]any{"error": err.Error()})
	return *logAction.Error
}
