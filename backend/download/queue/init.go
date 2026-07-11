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

// setLatestInfoTerminal records the terminal status of a processed queue entry on
// the shared LatestInfo, independent of notification delivery. SendNotification
// only updates LatestInfo when notifications are enabled — which is off by
// default — so without this the download queue status banner would stay stuck on
// "Processing..." after a successful run. Notification-enabled paths may still
// overwrite LatestInfo afterwards with their richer per-set message.
func setLatestInfoTerminal(message string, fileErrors, fileWarnings []string) {
	status := LAST_STATUS_SUCCESS
	if len(fileErrors) > 0 {
		status = LAST_STATUS_ERROR
	} else if len(fileWarnings) > 0 {
		status = LAST_STATUS_WARNING
	}
	if message == "" {
		message = "Unknown"
	}
	LatestInfo.Time = time.Now()
	LatestInfo.Status = status
	LatestInfo.Message = message
	LatestInfo.Errors = fileErrors
	LatestInfo.Warnings = fileWarnings
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
