package downloadqueue

import (
	"aura/config"
	"aura/database"
	"aura/logging"
	"aura/models"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// enqueueLegacyJob is swapped out by tests.
var enqueueLegacyJob = database.EnqueueDownloadQueueJob

// ImportLegacyQueueFiles moves pending jobs from the old file-based queue (download-queue/*.json)
// into the database queue, then moves each file into download-queue/imported. Fork: upstream's v6->v7
// creates the queue tables without them, so queued downloads were stranded on disk. Entries that had
// already failed (error_/warning_) and the old collection queue are left in place and logged.
func ImportLegacyQueueFiles(ctx context.Context) {
	dir := filepath.Join(config.ConfigPath, "download-queue")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	imported, skipped := 0, 0
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".json") {
			if e.IsDir() && name == "collections" {
				skipped++
			}
			continue
		}
		if strings.HasPrefix(name, "error_") || strings.HasPrefix(name, "warning_") {
			skipped++
			continue
		}
		file := filepath.Join(dir, name)
		data, err := os.ReadFile(file)
		var item models.DBSavedItem
		if err == nil {
			err = json.Unmarshal(data, &item)
		}
		if err != nil || item.MediaItem.TMDB_ID == "" {
			logging.LOGGER.Warn().Timestamp().Str("file", file).Msg("Skipping unreadable legacy download queue file")
			skipped++
			continue
		}
		if _, Err := enqueueLegacyJob(ctx, item); Err.Message != "" {
			logging.LOGGER.Error().Timestamp().Str("file", file).Str("error", Err.Message).Msg("Failed to import legacy download queue file")
			continue
		}
		done := filepath.Join(dir, "imported")
		if err := os.MkdirAll(done, 0o755); err == nil {
			_ = os.Rename(file, filepath.Join(done, name))
		}
		imported++
	}
	if imported > 0 || skipped > 0 {
		logging.LOGGER.Info().Timestamp().Int("imported", imported).Int("left_in_place", skipped).
			Str("folder", dir).Msg("Imported pending jobs from the old download queue folder")
	}
}
