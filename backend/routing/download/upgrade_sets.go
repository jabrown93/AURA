package routes_download

import (
	autodownload "aura/download/auto"
	"aura/logging"
	"aura/utils/httpx"
	"net/http"
)

type UpgradeSets_Request struct {
	Items []autodownload.UpgradeItem `json:"items"` // The saved items to upgrade, empty upgrades every saved item
}

// UpgradeSets godoc
// @Summary      Upgrade Saved Sets
// @Description  Replaces saved sets with a set from a higher ranked favorite creator, for the items given or every saved item. Runs even when Automatically Upgrade Sets is off. Sets with Auto Upgrade off are kept. The new sets are added to the Download Queue.
// @Tags         Download
// @Accept       json
// @Produce      json
// @Param        req  body      UpgradeSets_Request  false  "The saved items to upgrade"
// @Security     SessionCookie
// @Security     ApiKeyAuth
// @Failure      401  {object}  httpx.UnauthorizedResponse "Unauthorized (only when Auth.Enabled=true)"
// @Success      200  {object}  httpx.JSONResponse{data=autodownload.UpgradeSavedSetsResult}
// @Failure      500  {object}  httpx.JSONResponse "Internal Server Error"
// @Router       /api/download/upgrade-sets [post]
func UpgradeSets(w http.ResponseWriter, r *http.Request) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Upgrade Saved Sets", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, logAction)

	var req UpgradeSets_Request
	if r.ContentLength > 0 {
		if Err := httpx.DecodeRequestBodyToJSON(ctx, r.Body, &req, "Upgrade Saved Sets - Decode Request Body"); Err.Message != "" {
			httpx.SendResponse(w, ld, nil)
			return
		}
	}

	result, Err := autodownload.UpgradeSets(ctx, req.Items)
	if Err.Message != "" {
		logAction.SetErrorFromInfo(Err)
	}
	httpx.SendResponse(w, ld, result)
}
