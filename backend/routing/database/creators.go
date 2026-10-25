package routes_db

import (
	"aura/database"
	"aura/logging"
	"aura/utils/httpx"
	"net/http"
)

type getSavedSetCreatorsResponse struct {
	Creators []database.CreatorSetCount `json:"creators"`
}

// GetSavedSetCreators godoc
// @Summary      Get Saved Set Creators
// @Description  The creators of the saved sets and how many of their sets are saved, the most saved sets first.
// @Tags         Database
// @Produce      json
// @Security     SessionCookie
// @Security     ApiKeyAuth
// @Failure      401  {object}  httpx.UnauthorizedResponse "Unauthorized (only when Auth.Enabled=true)"
// @Success      200  {object}  httpx.JSONResponse{data=getSavedSetCreatorsResponse}
// @Failure      500  {object}  httpx.JSONResponse "Internal Server Error"
// @Router       /api/db/creators [get]
func GetSavedSetCreators(w http.ResponseWriter, r *http.Request) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Get Saved Set Creators", logging.LevelDebug)
	ctx = logging.WithCurrentAction(ctx, logAction)

	var response getSavedSetCreatorsResponse
	creators, Err := database.GetSavedSetCreators(ctx)
	if Err.Message != "" {
		logAction.SetErrorFromInfo(Err)
		httpx.SendResponse(w, ld, response)
		return
	}
	response.Creators = creators
	httpx.SendResponse(w, ld, response)
}
