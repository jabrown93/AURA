package silo

import (
	"aura/cache"
	"aura/config"
	"aura/logging"
	"aura/mediux"
	"aura/models"
	"aura/utils/httpx"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"strconv"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// SiloCollection is a collection in Silo's admin collection list
type SiloCollection struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	Description  string `json:"description"`
	ItemCount    int    `json:"item_count"`
	LibraryID    string `json:"library_id"`
	PosterURL    string `json:"poster_url"`   // Signed path, empty when none
	BackdropURL  string `json:"backdrop_url"` // Signed path, empty when none
	SourceConfig struct {
		Mode         string      `json:"mode"`
		CollectionID json.Number `json:"collection_id"` // The TMDB collection, for a TMDB collection
	} `json:"source_config"`
}

type SiloCollectionList struct {
	Items []SiloCollection `json:"items"`
}

// getLibraryCollections lists the collections of a Silo library
func getLibraryCollections(ctx context.Context, msConfig config.Config_MediaServer, libraryID string) ([]SiloCollection, logging.LogErrorInfo) {
	u, err := apiURL(msConfig.URL, "admin", "collections")
	if err != nil {
		message, help, detail := logging.Error_BaseUrlParsing(err)
		return nil, logging.LogErrorInfo{Message: message, Help: help, Detail: detail}
	}
	query := u.Query()
	query.Set("library_id", libraryID)
	u.RawQuery = query.Encode()

	_, respBody, Err := makeRequest(ctx, msConfig, u.String(), "GET", nil)
	if Err.Message != "" {
		return nil, Err
	}
	var siloResp SiloCollectionList
	Err = httpx.DecodeResponseToJSON(ctx, respBody, &siloResp, "Silo Collections Response")
	if Err.Message != "" {
		return nil, Err
	}
	return siloResp.Items, logging.LogErrorInfo{}
}

// libraryID asks Silo for the ID of one of aura's libraries, the config only has its title
func (s *Silo) libraryID(ctx context.Context, libraryTitle string) (string, logging.LogErrorInfo) {
	section := models.LibrarySection{LibrarySectionBase: models.LibrarySectionBase{Title: libraryTitle}}
	found, Err := s.GetLibrarySectionDetails(ctx, &section)
	if Err.Message != "" {
		return "", Err
	}
	if !found || section.ID == "" {
		return "", logging.LogErrorInfo{Message: fmt.Sprintf("Library '%s' was not found on the Silo server", libraryTitle)}
	}
	return section.ID, logging.LogErrorInfo{}
}

func (s *Silo) GetMovieCollections(ctx context.Context, library models.LibrarySection) (collections []models.CollectionItem, Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf("Silo: Fetching Movie Collections for Library '%s' [ID: %s]", library.Title, library.ID), logging.LevelDebug)
	defer logAction.Complete()

	collections = []models.CollectionItem{}
	if library.Type != "movie" {
		return collections, logging.LogErrorInfo{}
	}

	siloCollections, Err := getLibraryCollections(ctx, s.Config, library.ID)
	if Err.Message != "" {
		logAction.SetErrorFromInfo(Err)
		return collections, *logAction.Error
	}

	for _, siloCollection := range siloCollections {
		// Only collections with items in the library
		if siloCollection.ItemCount < 1 {
			continue
		}
		collectionItem := models.CollectionItem{
			RatingKey:    siloCollection.ID,
			Index:        siloCollection.ID,
			Title:        siloCollection.Title,
			Summary:      siloCollection.Description,
			ChildCount:   siloCollection.ItemCount,
			LibraryTitle: library.Title,
		}
		if siloCollection.SourceConfig.Mode == "tmdb_collection" {
			collectionItem.TMDB_ID = siloCollection.SourceConfig.CollectionID.String()
		}

		// Update the collections cache
		cache.CollectionsStore.UpsertCollection(&collectionItem)

		collections = append(collections, collectionItem)
	}
	return collections, logging.LogErrorInfo{}
}

func (s *Silo) GetMovieCollectionChildrenItems(ctx context.Context, collection *models.CollectionItem) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf(
		"Silo: Fetching Collection Children for '%s' | %s [%s | %s]",
		collection.Title, collection.LibraryTitle, collection.TMDB_ID, collection.RatingKey,
	), logging.LevelDebug)
	defer logAction.Complete()

	libraryID, Err := s.libraryID(ctx, collection.LibraryTitle)
	if Err.Message != "" {
		logAction.SetErrorFromInfo(Err)
		return *logAction.Error
	}

	// The items are in the library cache, by Silo's content ID
	store := cache.LibraryStore
	if !s.Config.IsMain() {
		store = cache.ServerLibraryStore(s.Config.ID)
	}

	items := []models.MediaItem{}
	cursor := ""
	for {
		u, err := apiURL(s.Config.URL, "library", libraryID, "collections", collection.RatingKey, "items")
		if err != nil {
			logAction.SetError(logging.Error_BaseUrlParsing(err))
			return *logAction.Error
		}
		query := u.Query()
		query.Set("limit", strconv.Itoa(200))
		if cursor != "" {
			query.Set("cursor", cursor)
		}
		u.RawQuery = query.Encode()

		_, respBody, Err := makeRequest(ctx, s.Config, u.String(), "GET", nil)
		if Err.Message != "" {
			logAction.SetErrorFromInfo(Err)
			return *logAction.Error
		}
		var siloResp SiloCatalogResponse
		Err = httpx.DecodeResponseToJSON(ctx, respBody, &siloResp, "Silo Collection Items Response")
		if Err.Message != "" {
			logAction.SetErrorFromInfo(Err)
			return *logAction.Error
		}

		for _, siloItem := range siloResp.Items {
			if siloItem.Type != "movie" {
				continue
			}
			item, found := store.GetMediaItemFromSectionByRatingKey(collection.LibraryTitle, siloItem.ContentID)
			if !found {
				logAction.AppendWarning(fmt.Sprintf("'%s' not found", siloItem.Title), "It isn't in the library cache yet")
				continue
			}
			itemCopy := *item

			// Update the collection cache with the item
			cache.CollectionsStore.UpdateMediaItemInCollectionByIndex(collection.Index, &itemCopy)
			items = append(items, itemCopy)
		}

		if !siloResp.Page.HasMore || siloResp.Page.NextCursor == "" {
			break
		}
		cursor = siloResp.Page.NextCursor
	}

	collection.MediaItems = items
	return logging.LogErrorInfo{}
}

func (s *Silo) GetCollectionItemImage(ctx context.Context, collection *models.CollectionItem, imageType string) (imageData []byte, Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf("Silo: Fetching %s for Collection '%s'", imageType, collection.Title), logging.LevelDebug)
	defer logAction.Complete()

	libraryID, Err := s.libraryID(ctx, collection.LibraryTitle)
	if Err.Message != "" {
		logAction.SetErrorFromInfo(Err)
		return nil, *logAction.Error
	}

	// The artwork links are signed and short-lived, so get them now
	siloCollections, Err := getLibraryCollections(ctx, s.Config, libraryID)
	if Err.Message != "" {
		logAction.SetErrorFromInfo(Err)
		return nil, *logAction.Error
	}
	artworkPath := ""
	for _, siloCollection := range siloCollections {
		if siloCollection.ID == collection.RatingKey {
			artworkPath = siloCollection.PosterURL
			if imageType == "backdrop" {
				artworkPath = siloCollection.BackdropURL
			}
		}
	}
	if artworkPath == "" {
		logAction.SetError("Silo Server does not have this image", "The collection may not have this image yet",
			map[string]any{"collection": collection.Title, "image_type": imageType})
		return nil, *logAction.Error
	}
	return getArtwork(ctx, s.Config, artworkPath)
}

func (s *Silo) ApplyCollectionImage(ctx context.Context, collectionItem *models.CollectionItem, imageFile models.ImageFile) (Err logging.LogErrorInfo) {
	ctx, logAction := logging.AddSubActionToContext(ctx, fmt.Sprintf(
		"Silo: Applying %s Image to Collection '%s' | %s [%s]",
		cases.Title(language.English).String(imageFile.Type), collectionItem.Title, collectionItem.LibraryTitle, collectionItem.RatingKey,
	), logging.LevelDebug)
	defer logAction.Complete()

	// Silo downloads the image from MediUX
	imageURL, Err := mediux.ConstructImageUrl(ctx, imageFile.ID, imageFile.Modified.String(), mediux.ImageQualityOriginal)
	if Err.Message != "" {
		logAction.SetErrorFromInfo(Err)
		return *logAction.Error
	}

	kind := "poster"
	if imageFile.Type == "collection_backdrop" {
		kind = "backdrop"
	}
	u, err := apiURL(s.Config.URL, "admin", "collections", collectionItem.RatingKey, kind)
	if err != nil {
		logAction.SetError(logging.Error_BaseUrlParsing(err))
		return *logAction.Error
	}

	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	if err := form.WriteField("source_url", imageURL); err != nil {
		logAction.SetError("Failed to build the image request", "", map[string]any{"error": err.Error()})
		return *logAction.Error
	}
	if err := form.Close(); err != nil {
		logAction.SetError("Failed to build the image request", "", map[string]any{"error": err.Error()})
		return *logAction.Error
	}

	_, _, Err = makeRequestWithHeaders(ctx, s.Config, u.String(), "PUT", body.Bytes(), map[string]string{"Content-Type": form.FormDataContentType()})
	if Err.Message != "" {
		logAction.SetErrorFromInfo(Err)
		return *logAction.Error
	}
	return logging.LogErrorInfo{}
}
