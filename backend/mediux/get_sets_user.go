package mediux

import (
	"aura/logging"
	"aura/models"
	"aura/utils/httpx"
	"context"
	_ "embed"
	"time"
)

//go:embed gen_user_sets.graphql
var queryAllUserSets string

type allUserSet_Response struct {
	Data   allUserSet_Data `json:"data"`
	Errors []ErrorResponse `json:"errors,omitempty"`
}

type allUserSet_Data struct {
	ShowSets       []showSetBySetID_ShowSetsByID   `json:"show_sets"`
	MovieSets      []movieSetBySetID_MovieSetsByID `json:"movie_sets"`
	CollectionSets []userBoxset_Collection_Set     `json:"collection_sets"`
	Boxsets        []userBoxset                    `json:"boxsets"`
}

type userBoxset struct {
	BaseSetInfo
	ShowSets       []showSetBySetID_ShowSetsByID   `json:"show_sets"`
	MovieSets      []movieSetBySetID_MovieSetsByID `json:"movie_sets"`
	CollectionSets []userBoxset_Collection_Set     `json:"collection_sets"`
}

type userBoxset_Collection_Set struct {
	BaseSetInfo
	Posters   []userBoxset_MovieImageItem `json:"movie_posters"`
	Backdrops []userBoxset_MovieImageItem `json:"movie_backdrops"`
}

type userBoxset_MovieImageItem struct {
	ImageAsset
	Movie BaseMovieInfo `json:"movie"`
}

func GetAllUserSets(ctx context.Context, username string) (creatorSets models.CreatorSetsResponse, Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Get All User Sets", logging.LevelInfo)
	defer logAction.Complete()

	creatorSets = models.CreatorSetsResponse{}
	Err = logging.LogErrorInfo{}

	// Send the GraphQL request
	resp, Err := makeGraphQLRequest(ctx, MediuxGraphQLQueryBody{
		Query:     queryAllUserSets,
		Variables: map[string]any{"username": username},
		QueryName: "getAllUserSets",
	})
	if Err.Message != "" {
		return creatorSets, Err
	}

	// Decode the response
	var userSetsResponse allUserSet_Response
	Err = httpx.DecodeResponseToJSON(ctx, resp.Body(), &userSetsResponse, "MediUX All User Sets Response")
	if Err.Message != "" {
		return creatorSets, Err
	}

	// If any errors returned from MediUX, log and return
	if len(userSetsResponse.Errors) > 0 {
		logAction.SetError("Errors returned from MediUX API", "Review the errors for more details",
			map[string]any{
				"username": username,
				"errors":   userSetsResponse.Errors,
			})
		return creatorSets, Err
	}

	logAction.AppendResult("Found user sets", map[string]int{
		"show_sets":       len(userSetsResponse.Data.ShowSets),
		"movie_sets":      len(userSetsResponse.Data.MovieSets),
		"collection_sets": len(userSetsResponse.Data.CollectionSets),
		"boxsets":         len(userSetsResponse.Data.Boxsets),
	})

	// If there is no show sets, movie sets, collection sets, or boxsets, return empty
	if len(userSetsResponse.Data.ShowSets) == 0 &&
		len(userSetsResponse.Data.MovieSets) == 0 &&
		len(userSetsResponse.Data.CollectionSets) == 0 &&
		len(userSetsResponse.Data.Boxsets) == 0 {
		return creatorSets, Err
	}

	creatorSets.IncludedItems = map[string]models.IncludedItem{}

	// Show Sets
	for _, mediuxShowSet := range userSetsResponse.Data.ShowSets {
		if _, exists := creatorSets.IncludedItems[mediuxShowSet.Show.ID]; !exists {
			baseItem := convertMediuxBaseItemToResponseBaseItem(mediuxShowSet.Show.BaseItemInfo, "show")
			baseItem.ReleaseDate = mediuxShowSet.Show.FirstAirDate
			creatorSets.IncludedItems[baseItem.TMDB_ID] = models.IncludedItem{MediuxInfo: baseItem}
		}
		setRef := models.SetRef{
			PosterSet: models.PosterSet{
				BaseSetInfo: models.BaseSetInfo{
					ID:               mediuxShowSet.ID,
					Title:            mediuxShowSet.SetTitle,
					Type:             "show",
					UserCreated:      mediuxShowSet.UserCreated.Username,
					DateCreated:      mediuxShowSet.DateCreated,
					DateUpdated:      mediuxShowSet.DateUpdated,
					Popularity:       mediuxShowSet.Popularity,
					PopularityGlobal: mediuxShowSet.PopularityGlobal,
				},
				Images: convertMediuxShowImagesToImageFiles(mediuxShowSet.BaseMediuxShowSet, mediuxShowSet.Show.ID),
			},
			ItemIDs: []string{mediuxShowSet.Show.ID},
		}
		creatorSets.ShowSets = append(creatorSets.ShowSets, setRef)
	}

	// Movie Sets
	for _, mediuxMovieSet := range userSetsResponse.Data.MovieSets {
		if _, exists := creatorSets.IncludedItems[mediuxMovieSet.Movie.ID]; !exists {
			baseItem := convertMediuxBaseItemToResponseBaseItem(mediuxMovieSet.Movie.BaseItemInfo, "movie")
			baseItem.ReleaseDate = mediuxMovieSet.Movie.ReleaseDate
			creatorSets.IncludedItems[baseItem.TMDB_ID] = models.IncludedItem{MediuxInfo: baseItem}
		}
		setRef := models.SetRef{
			PosterSet: models.PosterSet{
				BaseSetInfo: models.BaseSetInfo{
					ID:               mediuxMovieSet.ID,
					Title:            mediuxMovieSet.SetTitle,
					Type:             "movie",
					UserCreated:      mediuxMovieSet.UserCreated.Username,
					DateCreated:      mediuxMovieSet.DateCreated,
					DateUpdated:      mediuxMovieSet.DateUpdated,
					Popularity:       mediuxMovieSet.Popularity,
					PopularityGlobal: mediuxMovieSet.PopularityGlobal,
				},
				Images: convertMediuxMovieImagesToImageFiles(mediuxMovieSet.BaseMediuxMovieSet, mediuxMovieSet.Movie.ID),
			},
			ItemIDs: []string{mediuxMovieSet.Movie.ID},
		}
		creatorSets.MovieSets = append(creatorSets.MovieSets, setRef)
	}

	// Collection Sets
	for _, mediuxCollectionSet := range userSetsResponse.Data.CollectionSets {
		var itemIDs []string
		images := []models.ImageFile{}
		movieIDSet := map[string]struct{}{}

		// Posters
		for _, poster := range mediuxCollectionSet.Posters {
			movie := poster.Movie
			if _, exists := creatorSets.IncludedItems[movie.ID]; !exists {
				baseItem := convertMediuxBaseItemToResponseBaseItem(movie.BaseItemInfo, "movie")
				baseItem.ReleaseDate = movie.ReleaseDate
				creatorSets.IncludedItems[baseItem.TMDB_ID] = models.IncludedItem{MediuxInfo: baseItem}
			}
			if _, exists := movieIDSet[movie.ID]; !exists {
				itemIDs = append(itemIDs, movie.ID)
				movieIDSet[movie.ID] = struct{}{}
			}
			img := convertMediuxImageAssetToImageFile(&poster.ImageAsset, "poster")
			if img != nil {
				img.ItemTMDB_ID = movie.ID
				images = append(images, *img)
			}
		}

		// Backdrops (if present)
		for _, backdrop := range mediuxCollectionSet.Backdrops {
			movie := backdrop.Movie
			if _, exists := creatorSets.IncludedItems[movie.ID]; !exists {
				baseItem := convertMediuxBaseItemToResponseBaseItem(movie.BaseItemInfo, "movie")
				baseItem.ReleaseDate = movie.ReleaseDate
				creatorSets.IncludedItems[baseItem.TMDB_ID] = models.IncludedItem{MediuxInfo: baseItem}
			}
			if _, exists := movieIDSet[movie.ID]; !exists {
				itemIDs = append(itemIDs, movie.ID)
				movieIDSet[movie.ID] = struct{}{}
			}
			img := convertMediuxImageAssetToImageFile(&backdrop.ImageAsset, "backdrop")
			if img != nil {
				img.ItemTMDB_ID = movie.ID
				images = append(images, *img)
			}
		}

		setRef := models.SetRef{
			PosterSet: models.PosterSet{
				BaseSetInfo: models.BaseSetInfo{
					ID:               mediuxCollectionSet.ID,
					Title:            mediuxCollectionSet.SetTitle,
					Type:             "collection",
					UserCreated:      mediuxCollectionSet.UserCreated.Username,
					DateCreated:      mediuxCollectionSet.DateCreated,
					DateUpdated:      mediuxCollectionSet.DateUpdated,
					Popularity:       mediuxCollectionSet.Popularity,
					PopularityGlobal: mediuxCollectionSet.PopularityGlobal,
				},
				Images: images,
			},
			ItemIDs: itemIDs,
		}
		creatorSets.CollectionSets = append(creatorSets.CollectionSets, setRef)
	}

	// Boxsets
	for _, mediuxBoxset := range userSetsResponse.Data.Boxsets {
		var setIDs map[string][]string = make(map[string][]string)
		for _, showSet := range mediuxBoxset.ShowSets {
			setIDs["show"] = append(setIDs["show"], showSet.ID)
		}
		for _, movieSet := range mediuxBoxset.MovieSets {
			setIDs["movie"] = append(setIDs["movie"], movieSet.ID)
		}
		for _, collectionSet := range mediuxBoxset.CollectionSets {
			setIDs["collection"] = append(setIDs["collection"], collectionSet.ID)
		}

		if mediuxBoxset.DateUpdated.IsZero() {
			// Go through the boxset sets and get the most recent date updated to use for the boxset
			var mostRecentDateUpdated time.Time
			for _, showSet := range mediuxBoxset.ShowSets {
				if showSet.DateUpdated.After(mostRecentDateUpdated) {
					mostRecentDateUpdated = showSet.DateUpdated
				}
			}
			for _, movieSet := range mediuxBoxset.MovieSets {
				if movieSet.DateUpdated.After(mostRecentDateUpdated) {
					mostRecentDateUpdated = movieSet.DateUpdated
				}
			}
			for _, collectionSet := range mediuxBoxset.CollectionSets {
				if collectionSet.DateUpdated.After(mostRecentDateUpdated) {
					mostRecentDateUpdated = collectionSet.DateUpdated
				}
			}
			if mostRecentDateUpdated.IsZero() {
				logging.LOGGER.Warn().Timestamp().Str("boxset_id", mediuxBoxset.ID).Msg("All sets in boxset have zero value for DateUpdated, skipping boxset")
				continue
			}
			mediuxBoxset.DateUpdated = mostRecentDateUpdated
		}

		boxsetRef := models.BoxsetRef{
			BaseSetInfo: models.BaseSetInfo{
				ID:               mediuxBoxset.ID,
				Title:            mediuxBoxset.BoxsetTitle,
				Type:             "boxset",
				UserCreated:      mediuxBoxset.UserCreated.Username,
				DateCreated:      mediuxBoxset.DateCreated,
				DateUpdated:      mediuxBoxset.DateUpdated,
				Popularity:       mediuxBoxset.Popularity,
				PopularityGlobal: mediuxBoxset.PopularityGlobal,
			},
			SetIDs: setIDs,
		}
		creatorSets.Boxsets = append(creatorSets.Boxsets, boxsetRef)
	}

	logAction.AppendResult("size", map[string]int64{
		"mediux_response_bytes": resp.Size(),
	})
	return creatorSets, Err
}
