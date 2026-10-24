package routes_download

import (
	"aura/database"
	"aura/logging"
	"aura/utils/httpx"
	"net/http"
)

type RemoveDownloadHistoryEntry_Request struct {
	ID int64 `json:"id"`
}

type RemoveDownloadHistoryEntry_Response struct {
	Result string `json:"result"`
}

// RemoveDownloadHistoryEntry godoc
// @Summary      Download History - Remove Entry
// @Description  Remove a single Download History entry by ID.
// @Tags         Download
// @Accept       json
// @Produce      json
// @Param        req  body      RemoveDownloadHistoryEntry_Request  true  "History Remove Item Request"
// @Security     SessionCookie
// @Security     ApiKeyAuth
// @Failure      401  {object}  httpx.UnauthorizedResponse "Unauthorized (only when Auth.Enabled=true)"
// @Success      200           {object}  httpx.JSONResponse{data=RemoveDownloadHistoryEntry_Response}
// @Failure      500           {object}  httpx.JSONResponse "Internal Server Error"
// @Router       /api/download/history/item [delete]
func RemoveDownloadHistoryEntry(w http.ResponseWriter, r *http.Request) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Download History - Remove Entry", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, logAction)
	var req RemoveDownloadHistoryEntry_Request
	var response RemoveDownloadHistoryEntry_Response

	Err := httpx.DecodeRequestBodyToJSON(ctx, r.Body, &req, "History Remove Item - Decode Request Body")
	if Err.Message != "" {
		httpx.SendResponse(w, ld, response)
		return
	}

	validateAction := logAction.AddSubAction("Validate Delete Entry", logging.LevelDebug)
	if req.ID == 0 {
		validateAction.SetError("Invalid Delete Entry request",
			"Ensure that the request body contains a valid id",
			map[string]any{"id": req.ID})
		validateAction.Complete()
		httpx.SendResponse(w, ld, response)
		return
	}
	validateAction.Complete()

	Err = database.DeleteDownloadHistoryEntry(ctx, req.ID)
	if Err.Message != "" {
		httpx.SendResponse(w, ld, response)
		return
	}

	response.Result = "Removed entry from download history"
	httpx.SendResponse(w, ld, response)
}

type RemoveAllDownloadHistory_Response struct {
	Result string `json:"result"`
}

// RemoveAllDownloadHistory godoc
// @Summary      Download History - Clear All
// @Description  Remove all Download History entries.
// @Tags         Download
// @Accept       json
// @Produce      json
// @Security     SessionCookie
// @Security     ApiKeyAuth
// @Failure      401  {object}  httpx.UnauthorizedResponse "Unauthorized (only when Auth.Enabled=true)"
// @Success      200           {object}  httpx.JSONResponse{data=RemoveAllDownloadHistory_Response}
// @Failure      500           {object}  httpx.JSONResponse "Internal Server Error"
// @Router       /api/download/history [delete]
func RemoveAllDownloadHistory(w http.ResponseWriter, r *http.Request) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Download History - Clear All", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, logAction)
	var response RemoveAllDownloadHistory_Response

	Err := database.DeleteAllDownloadHistory(ctx)
	if Err.Message != "" {
		httpx.SendResponse(w, ld, response)
		return
	}

	response.Result = "Cleared download history"
	httpx.SendResponse(w, ld, response)
}
