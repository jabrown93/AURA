package routes_auth

import (
	"aura/config"
	"aura/logging"
	"aura/utils/httpx"
	"net/http"

	"github.com/go-chi/jwtauth/v5"
)

type sessionResponse struct {
	Authenticated bool   `json:"authenticated"`     // Whether the caller holds a usable session
	Subject       string `json:"subject,omitempty"` // "sub" claim of the active token
}

// GetSession godoc
// @Summary      Get Session
// @Description  Report whether the caller has a valid session. Always returns 200 so the UI can probe without tripping the global 401 redirect.
// @Tags         Auth
// @Produce      json
// @Success      200  {object}  httpx.JSONResponse{data=routes_auth.sessionResponse}
// @Router       /api/auth/session [get]
func GetSession(w http.ResponseWriter, r *http.Request) {
	_, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Get Session", logging.LevelTrace)
	defer logAction.Complete()

	httpx.SendResponse(w, ld, resolveSession(r))
}

func resolveSession(r *http.Request) sessionResponse {
	if !config.Current.Auth.Enabled {
		return sessionResponse{Authenticated: true}
	}

	if TokenAuth == nil {
		return sessionResponse{}
	}

	token, err := jwtauth.VerifyRequest(TokenAuth, r, jwtauth.TokenFromHeader, TokenFromSessionCookie)
	if err != nil || token == nil {
		return sessionResponse{}
	}

	subject, ok := token.Subject()
	if !ok || subject == "" {
		return sessionResponse{}
	}

	var typ string
	if err := token.Get("typ", &typ); err == nil && !IsSessionTokenType(typ) {
		return sessionResponse{}
	}

	return sessionResponse{Authenticated: true, Subject: subject}
}
