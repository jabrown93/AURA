package autodownload

import (
	"aura/database"
	"aura/logging"
	"aura/mediux"
	"aura/models"
	"aura/utils"
	"context"
)

// collectionCandidateSet is a saved collection set in a library that is eligible for
// auto-adding newly present members, together with a representative saved item's TMDB
// ID (used only to satisfy the MediUX lookup signature; it does not filter results when
// fetched with itemOnly=false).
type collectionCandidateSet struct {
	dbSet  models.DBPosterSetDetail
	tmdbID string
}

// ApplyCollectionSetsForNewMovie is invoked when a movie is (potentially) newly added to
// the library. For every saved collection set in the movie's library that has both
// Auto Download and Auto-add new collection items enabled, it applies that set's images
// to any members now present in the library but not yet saved — including this movie —
// and persists them, reusing handleCollectionAutoAddNewItems. This is the same work the
// daily auto-download job performs, triggered immediately instead of on the next poll.
//
// The movie is expected to already be resolved and present in the library cache (the
// callers resolve it via mediaserver.GetMediaItemDetails or the refreshed section cache),
// since handleCollectionAutoAddNewItems resolves each member's MediaItem from the cache.
func ApplyCollectionSetsForNewMovie(ctx context.Context, newMovie *models.MediaItem) {
	if newMovie == nil {
		return
	}

	ctx, action := logging.AddSubActionToContext(ctx,
		"Checking for collection sets to apply to newly added movie "+utils.MediaItemInfo(*newMovie),
		logging.LevelDebug)
	defer action.Complete()

	if newMovie.Type != "movie" || newMovie.TMDB_ID == "" || newMovie.LibraryTitle == "" || newMovie.RatingKey == "" {
		action.AppendResult("skipped", "movie is missing type/tmdb_id/library_title/rating_key")
		return
	}

	// If this movie already has saved sets, the existing re-apply paths (auto-download
	// recheck, Plex websocket re-apply, Sonarr/Radarr) own it — nothing to stage here.
	existingForMovie, Err := database.GetAllSavedSets(ctx, models.DBFilter{
		ItemTMDB_ID:      newMovie.TMDB_ID,
		ItemLibraryTitle: newMovie.LibraryTitle,
	})
	if Err.Message != "" {
		action.AppendWarning("saved_sets_lookup_error", Err.Message)
		return
	}
	if len(existingForMovie.Items) > 0 {
		action.AppendResult("skipped", "movie already has saved sets")
		return
	}

	candidates := collectionCandidateSetsForLibrary(ctx, newMovie.LibraryTitle)
	if len(candidates) == 0 {
		action.AppendResult("result", "no eligible collection sets in library")
		return
	}

	for _, candidate := range candidates {
		mediuxSet, includedItems, setErr := mediux.GetMovieCollectionSetByID(ctx, candidate.dbSet.ID, candidate.tmdbID, newMovie.LibraryTitle, false)
		if setErr.Message != "" {
			action.AppendWarning("collection_set_fetch_error", map[string]any{
				"set_id": candidate.dbSet.ID,
				"error":  setErr.Message,
			})
			continue
		}
		handleCollectionAutoAddNewItems(ctx, candidate.dbSet, includedItems, mediuxSet)
	}
}

// collectionCandidateSetsForLibrary returns the distinct saved collection sets in the
// given library that have both Auto Download and Auto-add new collection items enabled,
// keyed so each set is fetched once.
func collectionCandidateSetsForLibrary(ctx context.Context, libraryTitle string) []collectionCandidateSet {
	saved, Err := database.GetAllSavedSets(ctx, models.DBFilter{
		ItemLibraryTitle: libraryTitle,
		ItemsPerPage:     -1,
	})
	if Err.Message != "" {
		logging.LOGGER.Warn().Timestamp().
			Str("library_title", libraryTitle).
			Str("error", Err.Message).
			Msg("Collection auto-add: failed to look up saved sets for library")
		return nil
	}

	seen := map[string]struct{}{}
	candidates := make([]collectionCandidateSet, 0)
	for _, savedItem := range saved.Items {
		for _, dbSet := range savedItem.PosterSets {
			if dbSet.Type != "collection" || !dbSet.AutoDownload || !dbSet.AutoAddNewCollectionItems {
				continue
			}
			if _, exists := seen[dbSet.ID]; exists {
				continue
			}
			seen[dbSet.ID] = struct{}{}
			candidates = append(candidates, collectionCandidateSet{
				dbSet:  dbSet,
				tmdbID: savedItem.MediaItem.TMDB_ID,
			})
		}
	}
	return candidates
}

// HasEligibleCollectionSets reports whether the given library has at least one saved
// collection set with Auto Download and Auto-add new collection items enabled. The Radarr
// webhook calls it as a gate to avoid the (multi-second, retrying) background resolve when
// nothing could ever be applied. Note it is not free: it loads and unmarshals every saved
// set in the library (GetAllSavedSets with ItemsPerPage=-1), so on large libraries it is
// itself one of the more expensive steps of the webhook — acceptable because it runs at
// most once per import and short-circuits the far costlier resolve loop.
func HasEligibleCollectionSets(ctx context.Context, libraryTitle string) bool {
	return len(collectionCandidateSetsForLibrary(ctx, libraryTitle)) > 0
}
