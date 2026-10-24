package downloadqueue

import (
	"aura/database"
	"aura/logging"
	"context"
	"fmt"
)

func RemoveFromQueue(ctx context.Context, jobID int64) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx,
		fmt.Sprintf("Remove Download Queue Job %d", jobID),
		logging.LevelDebug)
	defer logAction.Complete()

	Err = logging.LogErrorInfo{}

	deleteErr := database.DeleteDownloadQueueJob(ctx, jobID)
	if deleteErr.Message != "" {
		logAction.SetErrorFromInfo(deleteErr)
		return *logAction.Error
	}

	QueueBroadcaster.Publish(QueueEvent{Type: "job_removed", Job: &database.DownloadQueueJob{ID: jobID}})

	return Err
}

func RemoveAllFromQueue(ctx context.Context) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Remove All Download Queue Jobs", logging.LevelDebug)
	defer logAction.Complete()

	Err = logging.LogErrorInfo{}

	deleteErr := database.DeleteAllDownloadQueueJobs(ctx)
	if deleteErr.Message != "" {
		logAction.SetErrorFromInfo(deleteErr)
		return *logAction.Error
	}

	QueueBroadcaster.Publish(QueueEvent{Type: "queue_cleared"})

	return Err
}
