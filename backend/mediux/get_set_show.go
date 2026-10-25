package mediux

import (
	"aura/cache"
	"aura/logging"
	"aura/models"
	"aura/utils/httpx"
	"context"
	_ "embed"
)

//go:embed gen_show_set_by_setid.graphql
var queryShowSetBySetID string

type showSetBySetID_Response struct {
	Data   showSetBySetID_Data `json:"data"`
	Errors []ErrorResponse     `json:"errors,omitempty"`
}

type showSetBySetID_Data struct {
	Set showSetBySetID_ShowSetsByID `json:"show_sets_by_id"`
}

type showSetBySetID_ShowSetsByID struct {
	BaseMediuxShowSet
	Show BaseShowInfo `json:"show_id"`
}

func GetShowSetByID(ctx context.Context, setID string, itemLibraryTitle string, edition string) (set models.SetRef, includedItems map[string]models.IncludedItem, Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, "Get Show Set By ID", logging.LevelInfo)
	defer logAction.Complete()

	set = models.SetRef{}
	includedItems = map[string]models.IncludedItem{}
	Err = logging.LogErrorInfo{}

	// Send the GraphQL request
	rawResp, Err := makeGraphQLRequest(ctx, MediuxGraphQLQueryBody{
		Query:     queryShowSetBySetID,
		Variables: map[string]any{"set_id": setID},
		QueryName: "getShowSetBySetID",
	})
	if Err.Message != "" {
		return set, includedItems, Err
	}

	// Decode the response
	var showSetResponse showSetBySetID_Response
	Err = httpx.DecodeResponseToJSON(ctx, rawResp.Body(), &showSetResponse, "MediUX Show Set By ID Response")
	if Err.Message != "" {
		return set, includedItems, Err
	}

	// If any errors returned from MediUX, log and return
	if len(showSetResponse.Errors) > 0 {
		logAction.SetError("Errors returned from MediUX API", "Review the errors for more details",
			map[string]any{
				"set_id":    setID,
				"item_type": "show",
				"errors":    showSetResponse.Errors,
			})
		return set, includedItems, *logAction.Error
	}

	mediuxShowSet := showSetResponse.Data.Set

	// 1) Populate included items with the show info
	baseItem := convertMediuxBaseItemToResponseBaseItem(mediuxShowSet.Show.BaseItemInfo, "show")
	baseItem.ReleaseDate = mediuxShowSet.Show.FirstAirDate
	includedItems[mediuxShowSet.Show.ID] = models.IncludedItem{MediuxInfo: baseItem}

	// Find the Media Item info from the cache
	mediaItem, found := cache.LibraryStore.GetMediaItemFromSectionByTMDBIDAndEdition(itemLibraryTitle, mediuxShowSet.Show.ID, edition)
	if found {
		includedItem := includedItems[mediuxShowSet.Show.ID]
		includedItem.MediaItem = *mediaItem
		includedItems[mediuxShowSet.Show.ID] = includedItem
	}

	// 2) Build SetRef for the show set
	setRef := models.SetRef{
		PosterSet: models.PosterSet{
			BaseSetInfo: convertMediuxBaseSetInfoToResponseBaseSetInfo(mediuxShowSet.BaseMediuxShowSet.BaseSetInfo, "show"),
			Images:      convertMediuxShowImagesToImageFiles(mediuxShowSet.BaseMediuxShowSet, mediuxShowSet.Show.ID),
		},
		ItemIDs: []string{mediuxShowSet.Show.ID},
	}

	return setRef, includedItems, Err
}
