package routes_silo_plugin

import (
	"aura/config"
	"aura/logging"
	"aura/siloplugin"
	"aura/utils/httpx"
	"net/http"

	"github.com/go-chi/chi/v5"
)

type getStatusResponse struct {
	AppVersion string `json:"app_version"`
}

// GetStatus godoc
// @Summary      Silo Plugin Status
// @Description  Lets the aura Silo plugin check its URL and API key.
// @Tags         Silo Plugin
// @Produce      json
// @Security     SessionCookie
// @Security     ApiKeyAuth
// @Failure      401  {object}  httpx.UnauthorizedResponse "Unauthorized (only when Auth.Enabled=true)"
// @Success      200  {object}  httpx.JSONResponse{data=getStatusResponse}
// @Router       /api/plugin/silo/status [get]
func GetStatus(w http.ResponseWriter, r *http.Request) {
	_, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	ld.AddAction("Silo Plugin Status", logging.LevelTrace).Complete()
	httpx.SendResponse(w, ld, getStatusResponse{AppVersion: config.AppVersion})
}

type getImagesResponse struct {
	Images []siloplugin.Image `json:"images"`
}

// GetImages godoc
// @Summary      Get Images for the Silo Plugin
// @Description  The MediUX images for a movie or show in aura's libraries, the best ranked set first: saved sets, then favorite creator rank, then MediUX popularity. For a refresh, a type with saved images only has those and the other types only have favorite creator images; the gallery has every image. Each image has a preview URL and a signed download path; Silo fetching the download path saves the set in aura.
// @Tags         Silo Plugin
// @Produce      json
// @Param        tmdb_id    query     string  true  "TMDB ID of the movie or show"
// @Param        item_type  query     string  true  "movie, show or series"
// @Param        purpose    query     string  false "refresh (default) offers saved and favorite creator images only, gallery offers every image"
// @Security     SessionCookie
// @Security     ApiKeyAuth
// @Failure      401  {object}  httpx.UnauthorizedResponse "Unauthorized (only when Auth.Enabled=true)"
// @Success      200  {object}  httpx.JSONResponse{data=getImagesResponse}
// @Failure      500  {object}  httpx.JSONResponse "Internal Server Error"
// @Router       /api/plugin/silo/images [get]
func GetImages(w http.ResponseWriter, r *http.Request) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Get Images for the Silo Plugin", logging.LevelDebug)
	ctx = logging.WithCurrentAction(ctx, logAction)

	var response getImagesResponse
	tmdbID := r.URL.Query().Get("tmdb_id")
	itemType := siloplugin.NormalizeItemType(r.URL.Query().Get("item_type"))
	if tmdbID == "" || (itemType != "movie" && itemType != "show") {
		logAction.SetError("Missing or Invalid Query Parameters", "tmdb_id is required and item_type must be movie, show or series",
			map[string]any{"tmdb_id": tmdbID, "item_type": itemType})
		httpx.SendResponse(w, ld, response)
		return
	}

	purpose := r.URL.Query().Get("purpose")
	if purpose != siloplugin.PurposeGallery {
		purpose = siloplugin.PurposeRefresh
	}
	images, Err := siloplugin.ItemImages(ctx, tmdbID, itemType, purpose)
	if Err.Message != "" {
		logAction.SetErrorFromInfo(Err)
		httpx.SendResponse(w, ld, response)
		return
	}
	response.Images = images
	httpx.SendResponse(w, ld, response)
}

// Download godoc
// @Summary      Download an Image for the Silo Plugin
// @Description  For a signed download path from Get Images, adds the set to the Download Queue for items that don't have it saved, then returns the original image. With notify=1 it only queues the set, the plugin calls it that way when Silo uses an image. Public, the signed token is the auth.
// @Tags         Silo Plugin
// @Produce      image/jpeg
// @Param        token   path      string  true   "Signed download token"
// @Param        notify        query     string  false  "1 to only queue the set, without the image"
// @Param        autodownload  query     string  false  "1 to save the set with Auto Download on"
// @Success      200  {string}  string "Image data"
// @Failure      500  {object}  httpx.JSONResponse "Internal Server Error"
// @Router       /api/plugin/silo/download/{token} [get]
func Download(w http.ResponseWriter, r *http.Request) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), "/api/plugin/silo/download")
	// The token is long and signed, keep it out of the logs
	if ld.Route != nil {
		ld.Route.Path = "/api/plugin/silo/download/{token}"
	}
	logAction := ld.AddAction("Download an Image for the Silo Plugin", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, logAction)

	token, err := siloplugin.VerifyDownloadToken(ctx, chi.URLParam(r, "token"))
	if err != nil {
		logAction.SetError("Invalid Download Token", err.Error(), nil)
		httpx.SendResponse(w, ld, nil)
		return
	}
	logAction.AppendResult("tmdb_id", token.TMDB_ID)
	logAction.AppendResult("set_id", token.SetID)
	logAction.AppendResult("image_type", token.ImageType)

	autoDownload := r.URL.Query().Get("autodownload") == "1"
	logAction.AppendResult("auto_download", autoDownload)

	// The plugin tells aura that Silo used the image, Silo downloads it from MediUX
	if r.URL.Query().Get("notify") == "1" {
		siloplugin.QueueInBackground(token, autoDownload)
		logAction.AppendResult("notify", true)
		logAction.Complete()
		w.WriteHeader(http.StatusNoContent)
		return
	}

	imageData, imageType, Err := siloplugin.Download(ctx, token, autoDownload)
	if Err.Message != "" {
		logAction.SetErrorFromInfo(Err)
		httpx.SendResponse(w, ld, nil)
		return
	}
	logAction.Complete()

	w.Header().Set("Content-Type", imageType)
	w.WriteHeader(http.StatusOK)
	w.Write(imageData)
}
