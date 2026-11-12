package mediaserver

import (
	"aura/cache"
	"aura/config"
	"aura/database"
	"aura/logging"
	"aura/mediux"
	"aura/models"
	"context"
	"strings"
)

// autoIgnoreItemsWithoutSets shares the temporary-ignore job's schedule and recovery path.
func autoIgnoreItemsWithoutSets(ctx context.Context) logging.LogErrorInfo {
	if !config.Current.UserPreferences.AutoIgnoreItemsWithoutSets {
		return logging.LogErrorInfo{}
	}
	var scanErr logging.LogErrorInfo
	for _, item := range cache.LibraryStore.GetAllMediaItems() {
		if err := ctx.Err(); err != nil {
			return logging.LogErrorInfo{Message: "Auto-ignore scan cancelled", Detail: map[string]any{"error": err.Error()}}
		}
		if strings.TrimSpace(item.TMDB_ID) == "" || (item.Type != "movie" && item.Type != "show") {
			continue
		}
		ignored, _, _, dbErr := database.CheckIfMediaItemExists(ctx, item.TMDB_ID, item.LibraryTitle, item.Edition)
		if dbErr.Message != "" {
			return dbErr
		}
		if ignored {
			continue
		}
		var sets []models.SetRef
		var lookupErr logging.LogErrorInfo
		if item.Type == "show" {
			sets, _, lookupErr = mediux.GetShowItemSets(ctx, item.TMDB_ID, item.LibraryTitle, item.Edition)
		} else {
			included := map[string]models.IncludedItem{}
			sets, lookupErr = mediux.GetMovieItemSets(ctx, item.TMDB_ID, item.LibraryTitle, item.Edition, &included)
			if lookupErr.Message == "" && len(sets) == 0 {
				sets, lookupErr = mediux.GetMovieItemCollectionSets(ctx, item.TMDB_ID, item.LibraryTitle, item.Edition, &included)
			}
		}
		if lookupErr.Message != "" {
			logging.LOGGER.Warn().Timestamp().
				Str("tmdb_id", item.TMDB_ID).
				Str("library_title", item.LibraryTitle).
				Str("edition", item.Edition).
				Str("error", lookupErr.Message).
				Msg("Skipping auto-ignore item after failed MediUX lookup")
			scanErr = lookupErr
			continue
		}
		if len(sets) == 0 {
			if dbErr = database.AutoIgnoreMediaItem(ctx, item.TMDB_ID, item.LibraryTitle, item.Edition); dbErr.Message != "" {
				return dbErr
			}
		}
	}
	return scanErr
}
