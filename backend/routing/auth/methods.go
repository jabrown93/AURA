package routes_auth

import (
	"aura/config"
	"aura/logging"
	"aura/utils/httpx"
	"net/http"
)

type authMethodsResponse struct {
	AuthEnabled     bool `json:"auth_enabled"`     // Whether authentication is enforced at all
	PasswordEnabled bool `json:"password_enabled"` // Whether the password form should be shown
}

// GetAuthMethods godoc
// @Summary      Get Auth Methods
// @Description  Report which sign-in methods are available. Public so the login page can render before the user is authenticated.
// @Tags         Auth
// @Produce      json
// @Success      200  {object}  httpx.JSONResponse{data=routes_auth.authMethodsResponse}
// @Router       /api/auth/methods [get]
func GetAuthMethods(w http.ResponseWriter, r *http.Request) {
	_, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Get Auth Methods", logging.LevelTrace)
	defer logAction.Complete()

	httpx.SendResponse(w, ld, buildAuthMethodsResponse())
}

func buildAuthMethodsResponse() authMethodsResponse {
	return authMethodsResponse{
		AuthEnabled:     config.Current.Auth.Enabled,
		PasswordEnabled: config.Current.Auth.Enabled && config.Current.Auth.Password != "",
	}
}
