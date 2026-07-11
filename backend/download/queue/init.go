package downloadqueue

import (
	"aura/config"
	"aura/logging"
	"aura/utils"
	"context"
	"os"
	"path"
	"time"
)

type Status string

const (
	LAST_STATUS_SUCCESS    Status = "Success"
	LAST_STATUS_WARNING    Status = "Warning"
	LAST_STATUS_ERROR      Status = "Error"
	LAST_STATUS_IDLE       Status = "Idle - Queue Empty"
	LAST_STATUS_PROCESSING Status = "Processing"
)

var (
	LatestInfo = struct {
		Time     time.Time
		Status   Status
		Message  string
		Errors   []string
		Warnings []string
	}{}

	FolderPath string = ""

	// CollectionFolderPath is a subfolder of FolderPath that holds queued
	// collection-image entries (models.CollectionQueueItem). It is a child
	// directory of FolderPath, so the media-item queue code — which does a
	// non-recursive os.ReadDir and skips directories — never picks these files
	// up, and vice versa.
	CollectionFolderPath string = ""
)

type FileIssues struct {
	Errors   []string
	Warnings []string
}

func init() {
	ctx, ld := logging.CreateLoggingContext(context.Background(), "Download Queue Init")
	defer ld.Log()
	logAction := ld.AddAction("Initializing Download Queue", logging.LevelTrace)
	ctx = logging.WithCurrentAction(ctx, logAction)
	defer logAction.Complete()

	FolderPath = path.Join(config.ConfigPath, "download-queue")
	CollectionFolderPath = path.Join(FolderPath, "collections")

	// Create the download queue folder if it doesn't exist
	Err := utils.CreateFolderIfNotExists(ctx, FolderPath)
	if Err.Message != "" {
		os.Exit(1)
	}

	// Create the collection sub-folder if it doesn't exist
	Err = utils.CreateFolderIfNotExists(ctx, CollectionFolderPath)
	if Err.Message != "" {
		os.Exit(1)
	}
}
