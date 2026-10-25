package mediux

import (
	"aura/cache"
	"aura/logging"
	"aura/models"
	"aura/utils/httpx"
	"context"
	_ "embed"
)

//go:embed gen_show_sets_by_tmdbid.graphql
var queryShowSetsByTMDBID string

type showSetsByTMDBID_Response struct {
	Data   showSetsByTMDBID_Data `json:"data"`
	Errors []ErrorResponse       `json:"errors,omitempty"`
}

type showSetsByTMDBID_Data struct {
	Show showSetsByTMDBID_ShowsByID `json:"shows_by_id"`
}

type showSetsByTMDBID_ShowsByID struct {
	BaseShowInfo
	Sets []BaseMediuxShowSet `json:"show_sets,omitempty"`
}

type BaseMediuxShowSet struct {
	BaseSetInfo
	ShowPoster    []ImageAsset `json:"show_poster"`
	ShowBackdrop  []ImageAsset `json:"show_backdrop"`
	SeasonPosters []ImageAsset `json:"season_posters"`
	Titlecards    []ImageAsset `json:"titlecards"`
}

func GetShowItemSets(ctx context.Context, tmdbID string, itemLibraryTitle string, edition string) (sets []models.SetRef, includedItems map[string]models.IncludedItem, Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Get Show Item Sets", logging.LevelInfo)
	defer logAction.Complete()

	sets = []models.SetRef{}
	includedItems = map[string]models.IncludedItem{}
	Err = logging.LogErrorInfo{}

	// Send the GraphQL request
	resp, Err := makeGraphQLRequest(ctx, MediuxGraphQLQueryBody{
		Query:     queryShowSetsByTMDBID,
		Variables: map[string]any{"tmdb_id": tmdbID},
		QueryName: "getShowItemSetsByTMDBID",
	})
	if Err.Message != "" {
		return sets, includedItems, Err
	}

	// Decode the response
	var showSetsResponse showSetsByTMDBID_Response
	Err = httpx.DecodeResponseToJSON(ctx, resp.Body(), &showSetsResponse, "MediUX Show Sets By TMDB ID Response")
	if Err.Message != "" {
		return sets, includedItems, Err
	}

	// If any errors returned from MediUX, log and return
	if len(showSetsResponse.Errors) > 0 {
		logAction.SetError("Errors returned from MediUX API", "Review the errors for more details",
			map[string]any{
				"tmdb_id":   tmdbID,
				"item_type": "show",
				"errors":    showSetsResponse.Errors,
			})
		return sets, includedItems, *logAction.Error
	}

	mediuxShow := showSetsResponse.Data.Show

	// If no show sets, return
	logAction.AppendResult("sets_found", len(mediuxShow.Sets))
	if len(mediuxShow.Sets) == 0 {
		return sets, includedItems, Err
	}

	// If the TMDB ID from MediUX does not match the requested TMDB ID, return error
	if mediuxShow.ID != tmdbID {
		logAction.SetError("TMDB ID mismatch in MediUX response", "The TMDB ID returned from MediUX does not match the requested show TMDB ID.",
			map[string]any{
				"requested_tmdb_id": tmdbID,
				"response_tmdb_id":  mediuxShow.ID,
			})
		return sets, includedItems, *logAction.Error
	}

	// 1) Populate included items with the show info
	if includedItems == nil {
		includedItems = map[string]models.IncludedItem{}
	}
	baseItem := convertMediuxBaseItemToResponseBaseItem(mediuxShow.BaseItemInfo, "show")
	baseItem.ReleaseDate = mediuxShow.FirstAirDate
	includedItem := includedItems[mediuxShow.ID]
	includedItem.MediuxInfo = baseItem

	// Find the Media Item info from the cache
	mediaItem, found := cache.LibraryStore.GetMediaItemFromSectionByTMDBIDAndEdition(itemLibraryTitle, tmdbID, edition)
	if found {
		includedItem.MediaItem = *mediaItem
		includedItems[mediuxShow.ID] = includedItem
	}

	// 2) Build the sets (one SetRef per set in MediUX)
	for _, set := range mediuxShow.Sets {
		setRef := models.SetRef{
			PosterSet: models.PosterSet{
				BaseSetInfo: models.BaseSetInfo{
					ID:               set.ID,
					Title:            set.SetTitle,
					Type:             "show",
					UserCreated:      set.UserCreated.Username,
					DateCreated:      set.DateCreated,
					DateUpdated:      set.DateUpdated,
					Popularity:       set.Popularity,
					PopularityGlobal: set.PopularityGlobal,
				},
				Images: convertMediuxShowImagesToImageFiles(set, mediuxShow.ID),
			},
			ItemIDs: []string{mediuxShow.ID},
		}
		sets = append(sets, setRef)
	}
	logAction.AppendResult("sets_returned", len(sets))
	return sets, includedItems, Err
}
