package routes_download

import (
	"aura/cache"
	"aura/database"
	"aura/logging"
	"aura/utils/httpx"
	"net/http"
	"strconv"
)

type GetDownloadHistory_Response struct {
	Entries    []database.DownloadHistoryEntry `json:"entries"`
	Total      int                             `json:"total"`
	MaxRetries int                             `json:"max_retries"` // How many times a failed download is retried automatically
}

// GetDownloadHistory godoc
// @Summary      Download History - Get Entries
// @Description  Retrieve the failed downloads (one per poster set), with their warning/error status, per-image failure details, and how many times each was retried. A failed set is retried automatically after 5 minutes, 30 minutes and 2 hours.
// @Tags         Download
// @Accept       json
// @Produce      json
// @Param        limit   query     int  false  "Max entries to return (default 50, -1 for all)"
// @Param        offset  query     int  false  "Offset for pagination (default 0)"
// @Security     SessionCookie
// @Security     ApiKeyAuth
// @Failure      401  {object}  httpx.UnauthorizedResponse "Unauthorized (only when Auth.Enabled=true)"
// @Success      200  {object}  httpx.JSONResponse{data=GetDownloadHistory_Response}
// @Failure      500           {object}  httpx.JSONResponse "Internal Server Error"
// @Router       /api/download/history [get]
func GetDownloadHistory(w http.ResponseWriter, r *http.Request) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Download History - Get Entries", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, logAction)
	var response GetDownloadHistory_Response

	limit := 50
	if v, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && (v > 0 || v == -1) {
		limit = v
	}
	offset := 0
	if v, err := strconv.Atoi(r.URL.Query().Get("offset")); err == nil && v >= 0 {
		offset = v
	}

	entries, total, Err := database.GetDownloadHistory(ctx, limit, offset)
	if Err.Message != "" {
		httpx.SendResponse(w, ld, response)
		return
	}

	for i := range entries {
		if ratingKey, found := cache.LibraryStore.GetRatingKeyByTMDBID(entries[i].LibraryTitle, entries[i].TMDB_ID); found {
			entries[i].RatingKey = ratingKey
		}
	}

	response.Entries = entries
	response.Total = total
	response.MaxRetries = len(database.DownloadRetryDelays)
	httpx.SendResponse(w, ld, response)
}
