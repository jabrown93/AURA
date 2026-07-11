package downloadqueue

import (
	"aura/logging"
	"aura/models"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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

	// Sanitize the attacker-controlled Collection fields into safe single path
	// segments, then verify the resulting name is a local (non-traversing) path
	// element before writing. Defense in depth: sanitizeQueueSegment already
	// strips path separators, and filepath.IsLocal rejects anything that could
	// still escape CollectionFolderPath (CodeQL go/path-injection).
	timestamp := time.Now().Unix()
	baseName := fmt.Sprintf("%s_%s_%d.json",
		sanitizeQueueSegment(item.CollectionItem.LibraryTitle),
		sanitizeQueueSegment(item.CollectionItem.RatingKey),
		timestamp,
	)
	if !filepath.IsLocal(baseName) {
		logAction.SetError("Refusing to write collection queue entry with a non-local file name",
			"The collection's library title or rating key produced an unsafe file name",
			map[string]any{
				"file": baseName,
			})
		return *logAction.Error
	}
	fileName := filepath.Join(CollectionFolderPath, baseName)

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
