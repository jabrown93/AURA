package routes_auth

import (
	"aura/logging"
	"aura/utils/httpx"
	"net/http"
)

type logoutResponse struct {
	LoggedOut bool `json:"logged_out"`
}

// Logout godoc
// @Summary      Auth Logout
// @Description  Clear the browser session cookie. Safe to call even if no session exists.
// @Tags         Auth
// @Produce      json
// @Success      200  {object}  httpx.JSONResponse{data=logoutResponse}
// @Router       /api/logout [post]
func Logout(w http.ResponseWriter, r *http.Request) {
	_, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("User Logout", logging.LevelInfo)
	defer logAction.Complete()

	ClearSessionCookie(w, r)

	httpx.SendResponse(w, ld, logoutResponse{LoggedOut: true})
}
