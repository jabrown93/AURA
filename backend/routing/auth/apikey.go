package routes_auth

import (
	"aura/config"
	"aura/logging"
	"aura/utils/httpx"
	"crypto/rand"
	"encoding/hex"
	"net/http"

	"aura/utils/argon2id"
)

type generateAPIKeyResponse struct {
	// APIKey is the plaintext key. It is only ever returned here, once - it is not stored and
	// cannot be retrieved again. Only its Argon2id hash is persisted.
	APIKey string `json:"api_key"`
}

// GenerateAPIKey godoc
// @Summary      Generate/Regenerate API Key
// @Description  Generates a new global API key for programmatic access (e.g. Sonarr/Radarr webhooks, scripts). The plaintext key is returned exactly once in this response and is never stored or retrievable again - copy it immediately. Regenerating replaces (revokes) any previous key immediately.
// @Tags         Auth
// @Produce      json
// @Security     SessionCookie
// @Security     ApiKeyAuth
// @Failure      401  {object}  httpx.UnauthorizedResponse "Unauthorized (only when Auth.Enabled=true)"
// @Success      200  {object}  httpx.JSONResponse{data=generateAPIKeyResponse}
// @Failure      500  {object}  httpx.JSONResponse "Internal Server Error"
// @Router       /api/config/auth/api-key [post]
func GenerateAPIKey(w http.ResponseWriter, r *http.Request) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Generate API Key", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, logAction)
	defer logAction.Complete()

	var response generateAPIKeyResponse

	rawKey, err := generateAPIKeySecret()
	if err != nil {
		logAction.SetError("Failed to generate API key", "An error occurred while generating a random secret", map[string]any{"error": err.Error()})
		httpx.SendResponse(w, ld, response)
		return
	}

	hash, err := argon2id.CreateHash(rawKey, argon2id.DefaultParams)
	if err != nil {
		logAction.SetError("Failed to hash API key", "An error occurred while hashing the generated key", map[string]any{"error": err.Error()})
		httpx.SendResponse(w, ld, response)
		return
	}

	// Only one config update at a time, so updates can't overwrite each other
	config.UpdateLock.Lock()
	defer config.UpdateLock.Unlock()

	newConfig := config.Current.Clone()
	newConfig.Auth.APIKeyHash = hash

	if saveErr := newConfig.Save(ctx); saveErr.Message != "" {
		logAction.SetError("Failed to save API key", saveErr.Message, saveErr.Detail)
		httpx.SendResponse(w, ld, response)
		return
	}
	config.Current = newConfig

	logAction.AppendResult("api_key_regenerated", true)

	response.APIKey = rawKey
	httpx.SendResponse(w, ld, response)
}

// generateAPIKeySecret creates a random, URL-safe API key with a recognizable prefix.
func generateAPIKeySecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "aura_" + hex.EncodeToString(b), nil
}

// VerifyAPIKey compares a plaintext API key against the configured hash. Always false if no
// key has been generated yet.
func VerifyAPIKey(key string) bool {
	if key == "" || config.Current.Auth.APIKeyHash == "" {
		return false
	}
	ok, err := argon2id.ComparePasswordAndHash(key, config.Current.Auth.APIKeyHash)
	if err != nil {
		return false
	}
	return ok
}
