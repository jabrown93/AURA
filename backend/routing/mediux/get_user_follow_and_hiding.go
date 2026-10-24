package routes_mediux

import (
	"aura/logging"
	"aura/mediux"
	"aura/models"
	"aura/utils/httpx"
	"net/http"
)

type GetUserFollowingAndHiding_Response struct {
	Users []models.MediuxUserInfo `json:"users"`
}

// GetUserFollowingAndHiding godoc
// @Summary      Get Mediux User Following and Hiding
// @Description  Retrieve the list of users that the current user is following or hiding in Mediux. This endpoint returns an array of user information, including their username, display name, and whether they are being followed or hidden by the current user. This allows clients to display the user's social connections and preferences within the Mediux ecosystem in the UI.
// @Tags         Mediux
// @Accept       json
// @Produce      json
// @Security     SessionCookie
// @Security     ApiKeyAuth
// @Failure      401  {object}  httpx.UnauthorizedResponse "Unauthorized (only when Auth.Enabled=true)"
// @Success	  200  {object}  httpx.JSONResponse{data=GetUserFollowingAndHiding_Response}
// @Failure	  500  {object}  httpx.JSONResponse "Internal Server Error"
// @Router       /api/mediux/user [get]
func GetUserFollowingAndHiding(w http.ResponseWriter, r *http.Request) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Get MediUX User Following and Hiding", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, logAction)
	var response GetUserFollowingAndHiding_Response
	users, Err := mediux.GetUserFollowingAndHiding(ctx)
	if Err.Message != "" {
		httpx.SendResponse(w, ld, response)
		return
	}
	response.Users = users

	httpx.SendResponse(w, ld, response)
}
