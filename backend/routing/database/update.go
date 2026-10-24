package routes_db

import (
	"aura/database"
	"aura/logging"
	"aura/models"
	"aura/utils/httpx"
	"net/http"
)

type updateItemRequest struct {
	Complete   bool               `json:"complete"`
	UpdateItem models.DBSavedItem `json:"update_item"`
}

type updateItemResponse struct {
	Result string `json:"result"`
}

// UpdateItemInDB godoc
// @Summary      Update Item In Database
// @Description  Update a Media Item and its associated Poster Sets in the database. Poster Sets marked with "to_delete" will be removed, while others will be upserted.
// @Tags         Database
// @Accept       json
// @Produce      json
// @Param        req  body      updateItemRequest  true  "Update Item Request"
// @Security     SessionCookie
// @Security     ApiKeyAuth
// @Failure      401  {object}  httpx.UnauthorizedResponse "Unauthorized (only when Auth.Enabled=true)"
// @Success      200           {object}  httpx.JSONResponse{data=updateItemResponse}
// @Failure      500           {object}  httpx.JSONResponse "Internal Server Error"
// @Router       /api/db [patch]
func UpdateItemInDB(w http.ResponseWriter, r *http.Request) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Update Item In Database", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, logAction)

	var req updateItemRequest
	var response updateItemResponse

	// Parse the request body

	Err := httpx.DecodeRequestBodyToJSON(ctx, r.Body, &req, "Update Item - Decode Request Body")
	if Err.Message != "" {
		httpx.SendResponse(w, ld, response)
		return
	}
	logAction.AppendResult("complete", req.Complete)

	for _, ps := range req.UpdateItem.PosterSets {
		if ps.ToDelete {
			// Delete the poster set
			Err := database.DeletePosterSetForMediaItem(ctx, req.UpdateItem.MediaItem.TMDB_ID, req.UpdateItem.MediaItem.LibraryTitle, req.UpdateItem.MediaItem.Edition, ps.ID)
			if Err.Message != "" {
				httpx.SendResponse(w, ld, response)
				return
			}
		} else {
			// Upsert the poster set
			Err := database.UpsertSavedItem(ctx, req.UpdateItem)
			if Err.Message != "" {
				httpx.SendResponse(w, ld, response)
				return
			}
		}
	}

	response.Result = "ok"
	httpx.SendResponse(w, ld, response)
}
