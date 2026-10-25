package routes_ms

import (
	"aura/config"
	"aura/logging"
	"aura/mediaserver"
	"aura/models"
	"aura/utils/httpx"
	"net/http"
)

type GetLibrarySections_Response struct {
	Sections []models.LibrarySection `json:"sections"`
}

// GetLibrarySections godoc
// @Summary      Get Library Sections
// @Description  Retrieve a list of library sections from the configured media server. This endpoint fetches the available library sections, including their ID, title, type, and path, allowing clients to display and interact with the media libraries configured on the media server.
// @Tags         MediaServer
// @Accept       json
// @Produce      json
// @Security     SessionCookie
// @Security     ApiKeyAuth
// @Failure      401  {object}  httpx.UnauthorizedResponse "Unauthorized (only when Auth.Enabled=true)"
// @Success      200  {object}  httpx.JSONResponse{data=GetLibrarySections_Response}
// @Failure      500  {object}  httpx.JSONResponse "Internal Server Error"
// @Router       /api/mediaserver/libraries [get]
func GetLibrarySections(w http.ResponseWriter, r *http.Request) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Get Library Sections", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, logAction)
	var response GetLibrarySections_Response

	for _, libSection := range config.Current.MediaServer.Libraries {
		base := models.LibrarySectionBase{
			ID:    libSection.ID,
			Title: libSection.Title,
			Type:  libSection.Type,
			Paths: libSection.Paths,
		}
		librarySection := models.LibrarySection{
			LibrarySectionBase: base,
		}

		found, Err := mediaserver.GetLibrarySectionDetails(ctx, &librarySection)
		if Err.Message != "" || !found {
			continue
		}
		response.Sections = append(response.Sections, librarySection)
	}

	if len(response.Sections) == 0 {
		logAction.SetError("No library sections found", "No valid library sections could be found on the configured Media Server", nil)
		httpx.SendResponse(w, ld, response)
		return
	}
	httpx.SendResponse(w, ld, response)
}
