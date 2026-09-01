package mediux

import (
	"aura/logging"
	"context"
)

func PreloadMediuxUsers(ctx context.Context) logging.LogErrorInfo {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Preloading MediUX Users", logging.LevelTrace)
	defer logAction.Complete()

	users, Err := GetAllUsers(ctx)
	if Err.Message != "" {
		logAction.SetErrorFromInfo(Err)
		return *logAction.Error
	}

	logAction.AppendResult("users_count", len(users))
	return logging.LogErrorInfo{}
}
