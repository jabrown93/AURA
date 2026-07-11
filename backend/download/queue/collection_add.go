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
	"time"
)

// AddCollectionToQueue writes a CollectionQueueItem to the collection download
// queue as a JSON file named "LibraryTitle_RatingKey_timestamp.json".
//
// Collection entries are keyed by RatingKey (not TMDB ID like the media-item
// queue) because a native media-server Collection is not guaranteed to have a
// TMDB ID, whereas RatingKey is always present and unique per media server.
func AddCollectionToQueue(ctx context.Context, item models.CollectionQueueItem) (Err logging.LogErrorInfo) {
	_, logAction := logging.AddSubActionToContext(ctx,
		fmt.Sprintf("Add Collection Queue Entry for '%s' [%s]",
			item.CollectionItem.Title, item.CollectionItem.RatingKey),
		logging.LevelDebug)
	defer logAction.Complete()

	Err = logging.LogErrorInfo{}

	timestamp := time.Now().Unix()
	fileName := path.Join(CollectionFolderPath, fmt.Sprintf("%s_%s_%d.json",
		strings.ReplaceAll(item.CollectionItem.LibraryTitle, " ", `_`),
		item.CollectionItem.RatingKey,
		timestamp,
	))

	jsonData, marshalErr := json.Marshal(item)
	if marshalErr != nil {
		logAction.SetError("Failed to marshal Collection Queue Item to JSON",
			"Ensure that the Collection Queue Item can be converted to JSON",
			map[string]any{
				"error": marshalErr.Error(),
				"item":  item,
			})
		return *logAction.Error
	}

	if writeErr := os.WriteFile(fileName, jsonData, 0644); writeErr != nil {
		logAction.SetError("Failed to write Collection Queue Item to download queue file",
			"Ensure that the application has permission to write to the download queue folder",
			map[string]any{
				"error": writeErr.Error(),
				"file":  fileName,
			})
		return *logAction.Error
	}

	logAction.AppendResult("file", fileName)
	return Err
}
