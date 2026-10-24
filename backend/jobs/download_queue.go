package jobs

import (
	downloadqueue "aura/download/queue"
	"context"
)

func StartDownloadQueueWorker(ctx context.Context) {
	downloadqueue.StartWorker(ctx)
}

func StopDownloadQueueWorker(ctx context.Context) {
	downloadqueue.StopWorker(ctx)
}
