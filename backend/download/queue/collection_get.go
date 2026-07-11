package downloadqueue

import (
	"aura/logging"
	"aura/models"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"strings"
)

// GetCollectionQueueItems reads the collection download queue subfolder and
// returns its entries categorized by status, using the same "error_"/"warning_"
// filename-prefix scheme as the media-item queue.
func GetCollectionQueueItems(ctx context.Context) (inProgressItems []models.CollectionQueueItem, warningItems []models.CollectionQueueItem, errorItems []models.CollectionQueueItem, Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Get Collection Download Queue Items", logging.LevelInfo)
	defer logAction.Complete()

	Err = logging.LogErrorInfo{}
	inProgressItems = []models.CollectionQueueItem{}
	warningItems = []models.CollectionQueueItem{}
	errorItems = []models.CollectionQueueItem{}

	files, readErr := os.ReadDir(CollectionFolderPath)
	if readErr != nil {
		logAction.SetError("Failed to read collection download queue folder",
			"Ensure that the application has permission to read the download queue folder",
			map[string]any{
				"error": readErr.Error(),
				"path":  CollectionFolderPath,
			})
		return inProgressItems, warningItems, errorItems, Err
	}

	if len(files) == 0 {
		logAction.AppendResult("message", "No items found in the collection download queue")
		return inProgressItems, warningItems, errorItems, Err
	}

	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".json") {
			continue
		}

		_, subAction := logging.AddSubActionToContext(ctx, fmt.Sprintf("Processing file: %s", file.Name()), logging.LevelDebug)

		filePath := path.Join(CollectionFolderPath, file.Name())
		subAction.AppendResult("file_path", filePath)

		content, readFileErr := os.ReadFile(filePath)
		if readFileErr != nil {
			subAction.SetError("Failed to read file",
				"Ensure that the application has permission to read files from the download queue folder",
				map[string]any{
					"error": readFileErr.Error(),
					"file":  filePath,
				})
			subAction.Complete()
			continue
		}

		var item models.CollectionQueueItem
		if decodeErr := json.Unmarshal(content, &item); decodeErr != nil {
			subAction.SetError("Failed to decode JSON content",
				"Ensure that the files in the collection download queue folder contain valid JSON with the correct structure",
				map[string]any{
					"error": decodeErr.Error(),
					"file":  filePath,
				})
			subAction.Complete()
			continue
		}

		if strings.HasPrefix(file.Name(), "error_") {
			errorItems = append(errorItems, item)
		} else if strings.HasPrefix(file.Name(), "warning_") {
			warningItems = append(warningItems, item)
		} else {
			inProgressItems = append(inProgressItems, item)
		}

		subAction.Complete()
	}

	logAction.AppendResult("in_progress_count", len(inProgressItems))
	logAction.AppendResult("warning_count", len(warningItems))
	logAction.AppendResult("error_count", len(errorItems))
	return inProgressItems, warningItems, errorItems, Err
}
