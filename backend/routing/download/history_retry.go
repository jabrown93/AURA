package routes_download

import (
	downloadqueue "aura/download/queue"
	"aura/logging"
	"aura/utils/httpx"
	"fmt"
	"net/http"
)

type RetryDownloadHistoryEntry_Request struct {
	ID int64 `json:"id"`
}

type RetryDownloadHistoryEntry_Response struct {
	Result string `json:"result"`
}

// RetryDownloadHistoryEntry godoc
// @Summary      Download History - Retry Entry
// @Description  Add a failed download's set back to the Download Queue, from the item's saved set. The entry is cleared once the set downloads without failures.
// @Tags         Download
// @Accept       json
// @Produce      json
// @Param        req  body      RetryDownloadHistoryEntry_Request  true  "History Retry Item Request"
// @Security     SessionCookie
// @Security     ApiKeyAuth
// @Failure      401  {object}  httpx.UnauthorizedResponse "Unauthorized (only when Auth.Enabled=true)"
// @Success      200           {object}  httpx.JSONResponse{data=RetryDownloadHistoryEntry_Response}
// @Failure      500           {object}  httpx.JSONResponse "Internal Server Error"
// @Router       /api/download/history/retry [post]
func RetryDownloadHistoryEntry(w http.ResponseWriter, r *http.Request) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Download History - Retry Entry", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, logAction)
	var req RetryDownloadHistoryEntry_Request
	var response RetryDownloadHistoryEntry_Response

	Err := httpx.DecodeRequestBodyToJSON(ctx, r.Body, &req, "History Retry Item - Decode Request Body")
	if Err.Message != "" {
		httpx.SendResponse(w, ld, response)
		return
	}

	if req.ID == 0 {
		logAction.SetError("Invalid Retry Entry request",
			"Ensure that the request body contains a valid id",
			map[string]any{"id": req.ID})
		httpx.SendResponse(w, ld, response)
		return
	}

	title, Err := downloadqueue.RetryFailedDownload(ctx, req.ID)
	if Err.Message != "" {
		httpx.SendResponse(w, ld, response)
		return
	}

	response.Result = fmt.Sprintf("Added %s to the Download Queue", title)
	httpx.SendResponse(w, ld, response)
}
