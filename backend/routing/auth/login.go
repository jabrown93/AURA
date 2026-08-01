package routes_auth

import (
	"aura/config"
	"aura/logging"
	"aura/utils/httpx"
	"net/http"

	"github.com/alexedwards/argon2id"
)

type loginRequest struct {
	Password string `json:"password"`
}

type loginResponse struct {
	Token string `json:"token"`
}

// Login godoc
// @Summary      Auth Login
// @Description  Authenticate user and return a JWT token
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        req  body      loginRequest  true  "Login Request"
// @Success      200           {object}  httpx.JSONResponse{data=loginResponse}
// @Failure      500           {object}  httpx.JSONResponse "Internal Server Error"
// @Router       /api/login [post]
func AttemptLogin(w http.ResponseWriter, r *http.Request) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("User Login", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, logAction)

	var req loginRequest
	var response loginResponse

	if !config.Current.Auth.Enabled {
		httpx.SendResponse(w, ld, "Authentication is disabled")
		return
	}

	if TokenAuth == nil {
		logAction.SetError("Authentication not configured", "The authentication system is not properly configured", nil)
		httpx.SendResponse(w, ld, response)
		return
	}

	Err := httpx.DecodeRequestBodyToJSON(ctx, r.Body, &req, "Login Request")
	if Err.Message != "" {
		httpx.SendResponse(w, ld, response)
		return
	}

	// Compare password
	ok, err := argon2id.ComparePasswordAndHash(req.Password, config.Current.Auth.Password)
	if err != nil || !ok {
		logAction.SetError("Invalid credentials", "The provided password is incorrect", map[string]any{
			"error": err,
		})
		httpx.SendResponse(w, ld, response)
		return
	}

	signedToken, err := IssueSessionToken("aura", nil)
	if err != nil {
		logAction.SetError("Failed to generate token", "An error occurred while generating the JWT token", map[string]any{
			"error": err,
		})
		httpx.SendResponse(w, ld, response)
		return
	}

	logAction.AppendResult("token_generated", true)

	SetSessionCookie(w, r, signedToken)

	// The token is still returned in the body for API clients that authenticate with a
	// bearer header; the browser UI relies on the cookie set above.
	response.Token = signedToken
	httpx.SendResponse(w, ld, response)
}
