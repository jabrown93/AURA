package mediux

import (
	"aura/cache"
	"aura/logging"
	"aura/models"
	"aura/utils/httpx"
	"context"
	_ "embed"
)

//go:embed gen_movie_collection_set_by_setid.graphql
var queryMovieCollectionSetBySetID string

type movieCollectionSetBySetID_Response struct {
	Data   movieCollectionSetBySetID_Data `json:"data"`
	Errors []ErrorResponse                `json:"errors,omitempty"`
}

type movieCollectionSetBySetID_Data struct {
	Collection movieCollectionSetBySetID_CollectionSetsByID `json:"collection_sets_by_id"`
}

type movieCollectionSetBySetID_CollectionSetsByID struct {
	BaseSetInfo
	CollectionID movieCollectionSetBySetID_Collection `json:"collection_id"`
}

type movieCollectionSetBySetID_Collection struct {
	Movies []movieCollectionSetBySetID_Movie `json:"movies"`
}

type movieCollectionSetBySetID_Movie struct {
	BaseMovieInfo
	Posters   []ImageAsset `json:"posters"`
	Backdrops []ImageAsset `json:"backdrops"`
}

func GetMovieCollectionSetByID(ctx context.Context, setID string, tmdbID string, itemLibraryTitle string, edition string, itemOnly bool) (set models.SetRef, includedItems map[string]models.IncludedItem, Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Get Movie Collection Set By Set ID", logging.LevelInfo)
	defer logAction.Complete()

	set = models.SetRef{}
	includedItems = map[string]models.IncludedItem{}
	Err = logging.LogErrorInfo{}

	// Send the GraphQL request
	resp, Err := makeGraphQLRequest(ctx, MediuxGraphQLQueryBody{
		Query:     queryMovieCollectionSetBySetID,
		Variables: map[string]any{"collection_set_id": setID, "collection_set_id_str": setID},
		QueryName: "getMovieCollectionSetBySetID",
	})
	if Err.Message != "" {
		return set, includedItems, Err
	}

	// Decode the response
	var movieCollectionSetResponse movieCollectionSetBySetID_Response
	Err = httpx.DecodeResponseToJSON(ctx, resp.Body(), &movieCollectionSetResponse, "MediUX Movie Collection Set By Set ID Response")
	if Err.Message != "" {
		return set, includedItems, Err
	}

	// If any errors returned from MediUX, log and return
	if len(movieCollectionSetResponse.Errors) > 0 {
		logAction.SetError("Errors returned from MediUX API", "Review the errors for more details",
			map[string]any{
				"collection_set_id": setID,
				"set_type":          "collection",
				"errors":            movieCollectionSetResponse.Errors,
			})
		return set, includedItems, *logAction.Error
	}

	// 1) Populate included items with the movie info
	if includedItems == nil {
		includedItems = map[string]models.IncludedItem{}
	}
	mediuxMovies := movieCollectionSetResponse.Data.Collection.CollectionID.Movies
	var itemIDs []string
	images := []models.ImageFile{}

	for _, movie := range mediuxMovies {
		if itemOnly && movie.ID != tmdbID {
			continue
		}
		// If the item is already included, skip
		if _, exists := includedItems[movie.ID]; exists {
			continue
		}
		baseItem := convertMediuxBaseItemToResponseBaseItem(movie.BaseItemInfo, "movie")
		baseItem.ReleaseDate = movie.ReleaseDate
		includedItems[movie.ID] = models.IncludedItem{MediuxInfo: baseItem}
		itemIDs = append(itemIDs, movie.ID)

		// Find the Media Item info from the cache. The primary item being browsed
		// (matching tmdbID) resolves to the exact edition; other collection
		// members are distinct movies, so we fall back to an arbitrary match.
		var mediaItem *models.MediaItem
		var found bool
		if movie.ID == tmdbID {
			mediaItem, found = cache.LibraryStore.GetMediaItemFromSectionByTMDBIDAndEdition(itemLibraryTitle, movie.ID, edition)
		} else {
			mediaItem, found = cache.LibraryStore.GetMediaItemFromSectionByTMDBID(itemLibraryTitle, movie.ID)
		}
		if found {
			includedItem := includedItems[movie.ID]
			includedItem.MediaItem = *mediaItem
			includedItems[movie.ID] = includedItem
		}

		// Posters
		for _, poster := range movie.Posters {
			img := convertMediuxImageAssetToImageFile(&poster, "poster")
			if img != nil {
				img.ItemTMDB_ID = movie.ID
				images = append(images, *img)
			}
		}
		// Backdrops
		for _, backdrop := range movie.Backdrops {
			img := convertMediuxImageAssetToImageFile(&backdrop, "backdrop")
			if img != nil {
				img.ItemTMDB_ID = movie.ID
				images = append(images, *img)
			}
		}
	}

	// 2) Build the collection set ref
	mediuxSet := movieCollectionSetResponse.Data.Collection
	setRef := models.SetRef{
		PosterSet: models.PosterSet{
			BaseSetInfo: convertMediuxBaseSetInfoToResponseBaseSetInfo(mediuxSet.BaseSetInfo, "collection"),
			Images:      images},
		ItemIDs: itemIDs,
	}

	return setRef, includedItems, Err
}
