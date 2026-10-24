package siloplugin

import (
	"aura/cache"
	autodownload "aura/download/auto"
	"aura/logging"
	"aura/mediaserver"
	"aura/mediux"
	"aura/models"
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// How often the media server can be asked for new items for an item aura doesn't have
const newItemsCheckInterval = 30 * time.Second

var (
	newItemsCheckMu   sync.Mutex
	lastNewItemsCheck time.Time
)

// checkForNewItems adds the main media server's new items to aura's libraries, at most every 30 seconds.
// Requests at the same time wait for the one check. It reports whether the caller should look again.
func checkForNewItems(ctx context.Context) bool {
	newItemsCheckMu.Lock()
	defer newItemsCheckMu.Unlock()
	if time.Since(lastNewItemsCheck) < newItemsCheckInterval {
		return true
	}
	lastNewItemsCheck = time.Now()
	_, Err := checkNewItems(ctx)
	return Err.Message == ""
}

// Image is one MediUX image for an item, for the Silo plugin
type Image struct {
	ID            string    `json:"id"`
	Kind          string    `json:"kind"` // poster, backdrop, season_poster or titlecard
	SeasonNumber  *int      `json:"season_number,omitempty"`
	EpisodeNumber *int      `json:"episode_number,omitempty"`
	Language      string    `json:"language"` // Language name from MediUX, empty for textless
	Modified      time.Time `json:"modified"`
	SetID         string    `json:"set_id"`
	SetTitle      string    `json:"set_title"`
	SetType       string    `json:"set_type"`
	Creator       string    `json:"creator"`
	SetRank       int       `json:"set_rank"` // 1 is the best set for the item
	Saved         bool      `json:"saved"`    // From a saved set, with its type selected
	PreviewURL    string    `json:"preview_url"`
	OriginalURL   string    `json:"original_url"`  // Public, Silo can only download artwork from public addresses on a refresh
	DownloadPath  string    `json:"download_path"` // Signed, the plugin calls it with notify=1 when Silo uses the image
}

// What Silo asks for images for
const (
	PurposeRefresh = "refresh" // Silo picks the images itself, so only saved and favorite creator images are offered
	PurposeGallery = "gallery" // The image picker, every image is offered
)

// Set in tests
var (
	getFavorites      = autodownload.FavoriteCreators
	getMovieSets      = mediux.GetMovieItemSets
	getCollectionSets = mediux.GetMovieItemCollectionSets
	getShowSets       = mediux.GetShowItemSets
	rankSets          = autodownload.RankSets
	checkNewItems     = mediaserver.CheckForNewMediaItems
	previewURL        = func(ctx context.Context, image models.ImageFile) (string, logging.LogErrorInfo) {
		return mediux.ConstructImageUrl(ctx, image.ID, image.Modified.String(), mediux.ImageQualityThumb)
	}
	originalURL = func(ctx context.Context, image models.ImageFile) (string, logging.LogErrorInfo) {
		return mediux.ConstructImageUrl(ctx, image.ID, image.Modified.String(), mediux.ImageQualityOriginal)
	}
)

// NormalizeItemType turns Silo's series into aura's show
func NormalizeItemType(itemType string) string {
	if itemType == "series" {
		return "show"
	}
	return itemType
}

// libraryItems returns aura's items for the TMDB ID and type, in every library
func libraryItems(tmdbID, itemType string) []models.MediaItem {
	items := []models.MediaItem{}
	for _, item := range cache.LibraryStore.GetMediaItemsByTMDBID(tmdbID) {
		if item.Type == itemType {
			items = append(items, item)
		}
	}
	return items
}

// itemSets returns every MediUX set with the item, with only the item's own images
func itemSets(ctx context.Context, item models.MediaItem) ([]models.SetRef, logging.LogErrorInfo) {
	var sets []models.SetRef
	switch item.Type {
	case "show":
		showSets, _, Err := getShowSets(ctx, item.TMDB_ID, item.LibraryTitle, item.Edition)
		if Err.Message != "" {
			return nil, Err
		}
		sets = showSets
	case "movie":
		setItems := map[string]models.IncludedItem{}
		movieSets, Err := getMovieSets(ctx, item.TMDB_ID, item.LibraryTitle, item.Edition, &setItems)
		if Err.Message != "" {
			return nil, Err
		}
		collectionSets, Err := getCollectionSets(ctx, item.TMDB_ID, item.LibraryTitle, item.Edition, &setItems)
		if Err.Message != "" {
			return nil, Err
		}
		sets = append(movieSets, collectionSets...)
	default:
		return nil, logging.LogErrorInfo{Message: fmt.Sprintf("Unknown item type '%s'", item.Type)}
	}

	// Collection sets have every movie's images, keep only this item's
	for i := range sets {
		images := []models.ImageFile{}
		for _, image := range sets[i].Images {
			if (image.ItemTMDB_ID == "" || image.ItemTMDB_ID == item.TMDB_ID) && models.IsKnownImageType(image.Type) {
				images = append(images, image)
			}
		}
		sets[i].Images = images
	}
	return sets, logging.LogErrorInfo{}
}

// savedSets combines the saved sets of the item in every library
func savedSets(items []models.MediaItem) []models.DBSavedSet {
	saved := []models.DBSavedSet{}
	seen := map[string]int{}
	for _, item := range items {
		for _, set := range item.DBSavedSets {
			if i, ok := seen[set.ID]; ok {
				saved[i].SelectedTypes = mergeSelectedTypes(saved[i].SelectedTypes, set.SelectedTypes)
				continue
			}
			seen[set.ID] = len(saved)
			saved = append(saved, set)
		}
	}
	return saved
}

// ItemImages returns the MediUX images for the item, the best ranked set first.
// For a refresh, a type with saved images only has those, the other types only have favorite creator images.
func ItemImages(ctx context.Context, tmdbID, itemType, purpose string) ([]Image, logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf("Silo Plugin: Getting Images for %s %s (%s)", itemType, tmdbID, purpose), logging.LevelDebug)
	defer logAction.Complete()

	itemType = NormalizeItemType(itemType)
	items := libraryItems(tmdbID, itemType)
	// A new item may not be in aura's libraries yet
	if len(items) == 0 && checkForNewItems(ctx) {
		items = libraryItems(tmdbID, itemType)
	}
	if len(items) == 0 {
		logAction.AppendResult("skipped", "the item isn't in aura's libraries")
		return []Image{}, logging.LogErrorInfo{}
	}

	// Ranked as one item, with the saved sets of every library
	item := items[0]
	item.DBSavedSets = savedSets(items)
	sets, Err := itemSets(ctx, item)
	if Err.Message != "" {
		logAction.SetErrorFromInfo(Err)
		return nil, Err
	}
	sets = rankSets(ctx, item, sets)

	selectedBySet := map[string]models.SelectedTypes{}
	for _, set := range item.DBSavedSets {
		selectedBySet[set.ID] = set.SelectedTypes
	}

	refresh := purpose != PurposeGallery
	favorites := map[string]bool{}
	if refresh {
		for _, username := range getFavorites(ctx) {
			favorites[strings.ToLower(username)] = true
		}
	}

	// The types the saved sets cover
	savedKinds := map[string]bool{}
	for _, set := range sets {
		if selected, ok := selectedBySet[set.ID]; ok {
			for _, image := range set.Images {
				if typeSelected(selected, image.Type, image.SeasonNumber) {
					savedKinds[kindKey(image)] = true
				}
			}
		}
	}

	images := []Image{}
	for rank, set := range sets {
		for _, image := range set.Images {
			selected, saved := selectedBySet[set.ID]
			saved = saved && typeSelected(selected, image.Type, image.SeasonNumber)
			if refresh && !saved && (savedKinds[kindKey(image)] || !favorites[strings.ToLower(set.UserCreated)]) {
				continue
			}
			preview, Err := previewURL(ctx, image)
			if Err.Message != "" {
				logAction.AppendWarning(image.ID, Err.Message)
				continue
			}
			original, Err := originalURL(ctx, image)
			if Err.Message != "" {
				logAction.AppendWarning(image.ID, Err.Message)
				continue
			}
			signed, err := SignDownloadToken(ctx, DownloadToken{
				TMDB_ID:       item.TMDB_ID,
				ItemType:      item.Type,
				SetID:         set.ID,
				ImageID:       image.ID,
				ImageType:     image.Type,
				Modified:      image.Modified,
				SeasonNumber:  image.SeasonNumber,
				EpisodeNumber: image.EpisodeNumber,
				Creator:       set.UserCreated,
				Refresh:       refresh,
			})
			if err != nil {
				logAction.SetError("Failed to sign the download URL", err.Error(), nil)
				return nil, *logAction.Error
			}
			images = append(images, Image{
				ID:            image.ID,
				Kind:          image.Type,
				SeasonNumber:  image.SeasonNumber,
				EpisodeNumber: image.EpisodeNumber,
				Language:      image.Language,
				Modified:      image.Modified,
				SetID:         set.ID,
				SetTitle:      set.Title,
				SetType:       set.Type,
				Creator:       set.UserCreated,
				SetRank:       rank + 1,
				Saved:         saved,
				PreviewURL:    preview,
				OriginalURL:   original,
				DownloadPath:  "/api/plugin/silo/download/" + signed,
			})
		}
	}
	logAction.AppendResult("sets", len(sets))
	logAction.AppendResult("images", len(images))
	return images, logging.LogErrorInfo{}
}

// kindKey is the image's type, with special season posters apart from the other seasons
func kindKey(image models.ImageFile) string {
	if image.Type == "season_poster" && image.SeasonNumber != nil && *image.SeasonNumber == 0 {
		return "special_season_poster"
	}
	return image.Type
}

// typeSelected reports whether an image's type is one of the selected types
func typeSelected(selected models.SelectedTypes, imageType string, seasonNumber *int) bool {
	switch imageType {
	case "poster":
		return selected.Poster
	case "backdrop":
		return selected.Backdrop
	case "season_poster":
		if seasonNumber != nil && *seasonNumber == 0 {
			return selected.SpecialSeasonPoster
		}
		return selected.SeasonPoster
	case "titlecard":
		return selected.Titlecard
	}
	return false
}

// onlyType selects just the image's type
func onlyType(imageType string, seasonNumber *int) models.SelectedTypes {
	special := seasonNumber != nil && *seasonNumber == 0
	return models.SelectedTypes{
		Poster:              imageType == "poster",
		Backdrop:            imageType == "backdrop",
		SeasonPoster:        imageType == "season_poster" && !special,
		SpecialSeasonPoster: imageType == "season_poster" && special,
		Titlecard:           imageType == "titlecard",
	}
}

func mergeSelectedTypes(a, b models.SelectedTypes) models.SelectedTypes {
	return models.SelectedTypes{
		Poster:              a.Poster || b.Poster,
		Backdrop:            a.Backdrop || b.Backdrop,
		SeasonPoster:        a.SeasonPoster || b.SeasonPoster,
		SpecialSeasonPoster: a.SpecialSeasonPoster || b.SpecialSeasonPoster,
		Titlecard:           a.Titlecard || b.Titlecard,
	}
}
