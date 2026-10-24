package mediux

import (
	"aura/cache"
	"aura/logging"
	"aura/models"
	"aura/utils/httpx"
	"context"
	_ "embed"
)

//go:embed gen_movie_sets_by_tmdbid.graphql
var queryMovieSetsByTMDBID string

type movieSetsByTMDBID_Response struct {
	Data   movieSetsByTMDBID_Data `json:"data"`
	Errors []ErrorResponse        `json:"errors,omitempty"`
}

type movieSetsByTMDBID_Data struct {
	Movie movieSetsByTMDBID_MoviesByID `json:"movies_by_id"`
}

type movieSetsByTMDBID_MoviesByID struct {
	BaseMovieInfo
	Sets []BaseMediuxMovieSet `json:"movie_sets,omitempty"`
}

type BaseMediuxMovieSet struct {
	BaseSetInfo
	MoviePoster   []ImageAsset `json:"movie_poster"`
	MovieBackdrop []ImageAsset `json:"movie_backdrop"`
}

func GetMovieItemSets(ctx context.Context, tmdbID string, itemLibraryTitle string, edition string, setItems *map[string]models.IncludedItem) (sets []models.SetRef, Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Get Movie Item Sets", logging.LevelInfo)
	defer logAction.Complete()

	sets = []models.SetRef{}
	Err = logging.LogErrorInfo{}

	// Send the GraphQL request
	resp, Err := makeGraphQLRequest(ctx, MediuxGraphQLQueryBody{
		Query:     queryMovieSetsByTMDBID,
		Variables: map[string]any{"tmdb_id": tmdbID},
		QueryName: "getMovieItemSetsByTMDBID",
	})
	if Err.Message != "" {
		return sets, Err
	}

	// Decode the response
	var movieSetsResponse movieSetsByTMDBID_Response
	Err = httpx.DecodeResponseToJSON(ctx, resp.Body(), &movieSetsResponse, "MediUX Movie Sets By TMDB ID Response")
	if Err.Message != "" {
		return sets, Err
	}

	// If any errors returned from MediUX, log and return
	if len(movieSetsResponse.Errors) > 0 {
		logAction.SetError("Errors returned from MediUX API", "Review the errors for more details",
			map[string]any{
				"tmdb_id":   tmdbID,
				"item_type": "movie",
				"errors":    movieSetsResponse.Errors,
			})
		return sets, *logAction.Error
	}

	mediuxMovie := movieSetsResponse.Data.Movie
	logAction.AppendResult("sets_found", len(mediuxMovie.Sets))

	// If no movie sets, return
	if len(mediuxMovie.Sets) == 0 {
		return sets, Err
	}

	// If the TMDB ID from MediUX does not match the requested TMDB ID, return error
	if mediuxMovie.ID != tmdbID {
		logAction.SetError("TMDB ID mismatch in MediUX response", "The TMDB ID returned from MediUX does not match the requested movie TMDB ID.",
			map[string]any{
				"requestedTMDBID": tmdbID,
				"returnedTMDBID":  mediuxMovie.ID,
			})
		return sets, *logAction.Error
	}

	// 1) Populate included items with the movie info
	if *setItems == nil {
		*setItems = map[string]models.IncludedItem{}
	}
	baseItem := convertMediuxBaseItemToResponseBaseItem(mediuxMovie.BaseItemInfo, "movie")
	baseItem.ReleaseDate = mediuxMovie.ReleaseDate
	includedItem := (*setItems)[mediuxMovie.ID]
	includedItem.MediuxInfo = baseItem
	(*setItems)[mediuxMovie.ID] = includedItem

	// Find the Media Item info from the cache
	mediaItem, found := cache.LibraryStore.GetMediaItemFromSectionByTMDBIDAndEdition(itemLibraryTitle, tmdbID, edition)
	if found {
		includedItem := (*setItems)[mediuxMovie.ID]
		includedItem.MediaItem = *mediaItem
		(*setItems)[mediuxMovie.ID] = includedItem
	}

	// 2) Build the sets (one SetRef per set in MediUX)
	for _, mediuxSet := range mediuxMovie.Sets {
		setRef := models.SetRef{
			PosterSet: models.PosterSet{
				BaseSetInfo: models.BaseSetInfo{
					ID:               mediuxSet.ID,
					Title:            mediuxSet.SetTitle,
					Type:             "movie",
					UserCreated:      mediuxSet.UserCreated.Username,
					DateCreated:      mediuxSet.DateCreated,
					DateUpdated:      mediuxSet.DateUpdated,
					Popularity:       mediuxSet.Popularity,
					PopularityGlobal: mediuxSet.PopularityGlobal,
				},
				Images: convertMediuxMovieImagesToImageFiles(mediuxSet, mediuxMovie.ID),
			},
			ItemIDs: []string{mediuxMovie.ID},
		}
		sets = append(sets, setRef)
	}
	logAction.AppendResult("sets_returned", len(sets))
	return sets, Err
}
