package mediux

import (
	"aura/cache"
	"aura/logging"
	"aura/models"
	"aura/utils/httpx"
	"context"
	_ "embed"
)

//go:embed gen_items_with_sets.graphql
var queryItemsWithSets string

func PreLoadMediuxItemsWithSets(ctx context.Context) logging.LogErrorInfo {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Preloading MediUX Items with Sets", logging.LevelTrace)
	defer logAction.Complete()

	// Send the request
	URL := "https://api.mediux.io/lists/content_ids"
	resp, respBody, Err := makeRequest(ctx, URL, "GET", nil, "", false)
	if Err.Message != "" {
		logAction.SetErrorFromInfo(Err)
		logging.LOGGER.Error().Timestamp().Msgf("Failed to make request to MediUX API for items with sets: %s", Err.Message)
		return *logAction.Error
	}
	defer resp.Body.Close()

	currentMoviesCount, currentShowsCount := cache.MediuxItems.GetCountMediuxItems()
	logAction.AppendResult("current_movies_count", currentMoviesCount)
	logAction.AppendResult("current_shows_count", currentShowsCount)

	var mediuxResp models.MediuxContentIdsResponse

	// Decode the complete response before publishing the new cache snapshot.
	Err = httpx.DecodeResponseToJSON(ctx, respBody, &mediuxResp, "MediUX Items with Sets Response Decoding")
	if Err.Message != "" {
		logAction.SetErrorFromInfo(Err)
		logging.LOGGER.Error().Timestamp().Msgf("Failed to decode MediUX items with sets response: %s", Err.Message)
		return *logAction.Error
	}

	moviesCount := len(mediuxResp.Movies)
	showsCount := len(mediuxResp.Shows)

	cache.MediuxItems.StoreMediuxItems(mediuxResp.Movies, mediuxResp.Shows)
	logEvent := logging.LOGGER.Info().Timestamp()
	if currentMoviesCount > 0 {
		logEvent = logEvent.Int("current_movies_count", currentMoviesCount)
	}
	if currentShowsCount > 0 {
		logEvent = logEvent.Int("current_shows_count", currentShowsCount)
	}
	logEvent.Int("new_movies_count", moviesCount).Int("new_shows_count", showsCount).Msgf("Loaded %d items with sets from MediUX API", moviesCount+showsCount)
	return logging.LogErrorInfo{}
}
