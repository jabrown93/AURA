package downloadqueue

import (
	"aura/cache"
	"aura/config"
	"aura/kometa"
	"aura/logging"
	"aura/mediaserver"
	"aura/models"
	"context"
	"fmt"
)

// Fork: Kometa-mode additions to the queue processor.

// preloadMissingImage reports whether an image that ImageIsSelected skipped should still be written
// as a Kometa asset: its type is selected, the set asks to preload missing seasons and episodes,
// and the show is on the server with at least one episode.
func preloadMissingImage(item models.MediaItem, set models.DBPosterSetDetail, image models.ImageFile) bool {
	if !set.ForcePreloadMissing || !config.Current.Images.Kometa.Enabled || !showHasAnyEpisode(item.Series) {
		return false
	}
	switch image.Type {
	case "season_poster":
		if image.SeasonNumber == nil {
			return false
		}
		if *image.SeasonNumber == 0 {
			return set.SelectedTypes.SpecialSeasonPoster
		}
		return set.SelectedTypes.SeasonPoster
	case "titlecard":
		// Both numbers are needed to build the Kometa file name
		return set.SelectedTypes.Titlecard && image.SeasonNumber != nil && image.EpisodeNumber != nil
	}
	return false
}

func showHasAnyEpisode(series *models.MediaItemSeries) bool {
	if series == nil {
		return false
	}
	for _, season := range series.Seasons {
		if len(season.Episodes) > 0 {
			return true
		}
	}
	return false
}

// reresolveStaleRatingKey retries the media-server lookup once with the library cache's current
// rating key after a 404. Plex reassigns keys when an item is removed and re-added, so a key valid
// at enqueue time can be dead when the job runs. Only runs after a genuine 404, so an entry whose
// key still resolves is never redirected to another edition sharing the TMDB ID.
func reresolveStaleRatingKey(ctx context.Context, item *models.MediaItem, found bool, mediaErr logging.LogErrorInfo, logAction *logging.LogAction) (bool, logging.LogErrorInfo) {
	if !mediaserver.IsItemNotFound(mediaErr) {
		return found, mediaErr
	}
	cached, ok := cache.LibraryStore.GetMediaItemFromSectionByTMDBIDAndEdition(item.LibraryTitle, item.TMDB_ID, item.Edition)
	if !ok || cached.RatingKey == "" || cached.RatingKey == item.RatingKey {
		return found, mediaErr
	}
	logAction.AppendResult("rating_key_refresh", fmt.Sprintf("%s -> %s (stale key re-resolved by TMDB %s)", item.RatingKey, cached.RatingKey, item.TMDB_ID))
	item.RatingKey = cached.RatingKey
	return mediaserver.GetMediaItemDetails(ctx, item)
}

// saveViaArrFallback writes the images to the Kometa asset folder named after the Sonarr/Radarr path
// when the media server no longer has the item. Only on a 404: transient and auth errors are left to
// fail so the download is retried.
func saveViaArrFallback(ctx context.Context, queueItem models.DBSavedItem, mediaErr logging.LogErrorInfo) (handled bool, warning string) {
	if !mediaserver.IsItemNotFound(mediaErr) {
		return false, ""
	}
	handled, _, kErr := kometa.SaveViaSonarrRadarrFallback(ctx, queueItem.MediaItem, queueItem.PosterSets)
	if handled && kErr.Message != "" {
		warning = fmt.Sprintf("kometa fallback: %s", kErr.Message)
	}
	return handled, warning
}
