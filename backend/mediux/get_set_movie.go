package mediux

import (
	"aura/cache"
	"aura/logging"
	"aura/models"
	"aura/utils/httpx"
	"context"
	_ "embed"
)

//go:embed gen_movie_set_by_setid.graphql
var queryMovieSetBySetID string

type movieSetBySetID_Response struct {
	Data   movieSetBySetID_Data `json:"data"`
	Errors []ErrorResponse      `json:"errors,omitempty"`
}

type movieSetBySetID_Data struct {
	Set movieSetBySetID_MovieSetsByID `json:"movie_sets_by_id"`
}

type movieSetBySetID_MovieSetsByID struct {
	BaseMediuxMovieSet
	Movie BaseMovieInfo `json:"movie_id"`
}

func GetMovieSetByID(ctx context.Context, setID string, itemLibraryTitle string, edition string) (set models.SetRef, includedItems map[string]models.IncludedItem, Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Get Movie Set By ID", logging.LevelInfo)
	defer logAction.Complete()

	set = models.SetRef{}
	includedItems = map[string]models.IncludedItem{}
	Err = logging.LogErrorInfo{}

	// Send the GraphQL request
	resp, Err := makeGraphQLRequest(ctx, MediuxGraphQLQueryBody{
		Query:     queryMovieSetBySetID,
		Variables: map[string]any{"set_id": setID},
		QueryName: "getMovieSetBySetID",
	})
	if Err.Message != "" {
		return set, includedItems, Err
	}

	// Decode the response
	var movieSetResponse movieSetBySetID_Response
	Err = httpx.DecodeResponseToJSON(ctx, resp.Body(), &movieSetResponse, "MediUX Movie Set By Set ID Response")
	if Err.Message != "" {
		return set, includedItems, Err
	}

	// If any errors returned from MediUX, log and return
	if len(movieSetResponse.Errors) > 0 {
		logAction.SetError("Errors returned from MediUX API", "Review the errors for more details",
			map[string]any{
				"movie_set_id": setID,
				"item_type":    "movie",
				"errors":       movieSetResponse.Errors,
			})
		return set, includedItems, *logAction.Error
	}

	mediuxMovieSet := movieSetResponse.Data.Set

	// 1) Populate included items with the movie info
	if includedItems == nil {
		includedItems = map[string]models.IncludedItem{}
	}
	baseItem := convertMediuxBaseItemToResponseBaseItem(mediuxMovieSet.Movie.BaseItemInfo, "movie")
	baseItem.ReleaseDate = mediuxMovieSet.Movie.ReleaseDate
	includedItems[mediuxMovieSet.Movie.ID] = models.IncludedItem{MediuxInfo: baseItem}

	// Get the Media Item from the cache
	mediaItem, found := cache.LibraryStore.GetMediaItemFromSectionByTMDBIDAndEdition(itemLibraryTitle, mediuxMovieSet.Movie.ID, edition)
	if found {
		// Update the included item with additional info from the media item
		includedItem := includedItems[mediuxMovieSet.Movie.ID]
		includedItem.MediaItem = *mediaItem
		includedItems[mediuxMovieSet.Movie.ID] = includedItem
	}

	// 2) Build SetRef for the movie set
	setRef := models.SetRef{
		PosterSet: models.PosterSet{
			BaseSetInfo: convertMediuxBaseSetInfoToResponseBaseSetInfo(mediuxMovieSet.BaseMediuxMovieSet.BaseSetInfo, "movie"),
			Images:      convertMediuxMovieImagesToImageFiles(mediuxMovieSet.BaseMediuxMovieSet, mediuxMovieSet.Movie.ID),
		},
		ItemIDs: []string{mediuxMovieSet.Movie.ID},
	}

	set = setRef
	return set, includedItems, Err
}
