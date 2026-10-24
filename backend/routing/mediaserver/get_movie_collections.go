package routes_ms

import (
	"aura/config"
	"aura/logging"
	"aura/mediaserver"
	"aura/mediaserver/ej"
	"aura/models"
	"aura/utils/httpx"
	"context"
	"net/http"
)

type GetMovieCollections_Response struct {
	Collections []models.CollectionItem `json:"collections"`
}

// GetMovieCollections godoc
// @Summary      Get Movie Collections
// @Description  Retrieve a list of movie collections from the media server. This endpoint fetches all movie collections available in the media server's movie libraries, allowing clients to display and interact with the collections of movies configured on the media server.
// @Tags         MediaServer
// @Accept       json
// @Produce      json
// @Security     SessionCookie
// @Security     ApiKeyAuth
// @Failure      401  {object}  httpx.UnauthorizedResponse "Unauthorized (only when Auth.Enabled=true)"
// @Success      200  {object}  httpx.JSONResponse{data=GetMovieCollections_Response}
// @Failure      500  {object}  httpx.JSONResponse "Internal Server Error"
// @Router       /api/mediaserver/collections [get]
func GetMovieCollections(w http.ResponseWriter, r *http.Request) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Get Movie Collections", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, logAction)
	var response GetMovieCollections_Response

	var Err logging.LogErrorInfo
	switch config.Current.MediaServer.Type {
	case "Plex", "Silo":
		logging.DevMsg("Fetching movie collections from each movie library")
		response.Collections, Err = getLibraryMovieCollections(ctx)
		if Err.Message != "" {
			httpx.SendResponse(w, ld, response)
			return
		}
	case "Emby", "Jellyfin":
		logging.DevMsg("Fetching movie collections from Emby/Jellyfin media server")
		response.Collections, Err = getEmbyJellyfinMovieCollections(ctx)
		if Err.Message != "" {
			httpx.SendResponse(w, ld, response)
			return
		}
	default:
		logAction.SetError("Unsupported media server type", "The configured media server type is not supported for fetching collections", map[string]any{
			"media_server_type": config.Current.MediaServer.Type,
		})
		httpx.SendResponse(w, ld, response)
		return
	}

	httpx.SendResponse(w, ld, response)
}

// getLibraryMovieCollections gets the collections of each movie library aura uses
func getLibraryMovieCollections(ctx context.Context) (collections []models.CollectionItem, Err logging.LogErrorInfo) {
	collections = []models.CollectionItem{}

	// Get all Movie Library Sections
	sections, Err := mediaserver.GetLibrarySections(ctx, &config.Current.MediaServer)
	if Err.Message != "" {
		return collections, Err
	}

	if len(sections) == 0 {
		Err = logging.LogErrorInfo{
			Message: "No library sections found on the media server",
			Help:    "Ensure your media server has libraries configured and try again",
		}
		return collections, Err
	}

	movieLibraries := []models.LibrarySection{}
	for _, section := range sections {
		if section.Type != "movie" {
			continue
		}
		// Check to see if this section is the list of configured libraries
		currentSections := config.Current.MediaServer.Libraries
		if len(currentSections) > 0 {
			found := false
			for _, currentSection := range currentSections {
				if currentSection.Title == section.Title {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		movieLibraries = append(movieLibraries, section)
	}

	if len(movieLibraries) == 0 {
		Err = logging.LogErrorInfo{
			Message: "No movie libraries found on the media server",
			Help:    "Add a movie library to your media server and try again",
		}
		return collections, Err
	}

	logging.DevMsgf("Found %d movie libraries on the media server", len(movieLibraries))
	for _, library := range movieLibraries {
		logging.DevMsgf("Fetching collections for library: %s", library.Title)
		libCollections, err := mediaserver.GetMovieCollections(ctx, library)
		if err.Message != "" {
			return collections, err
		}
		collections = append(collections, libCollections...)
	}

	logging.DevMsgf("Total collections found: %d", len(collections))
	return collections, Err
}

func getEmbyJellyfinMovieCollections(ctx context.Context) (collections []models.CollectionItem, Err logging.LogErrorInfo) {
	collections = []models.CollectionItem{}

	// Get the Media Server Collection Section
	collectionSection, Err := ej.GetMovieCollectionSection(ctx, config.Current.MediaServer)
	if Err.Message != "" {
		return collections, Err
	}

	logging.DevMsgf("Found collection section: %s (ID: %s)", collectionSection.Title, collectionSection.ID)
	// Fetch Collection Items from the Collection Section
	collections, Err = mediaserver.GetMovieCollections(ctx, collectionSection)
	if Err.Message != "" {
		return collections, Err
	}

	logging.DevMsgf("Total collections found: %d", len(collections))

	return collections, Err
}
