package routes_images

import (
	"aura/cache"
	"aura/logging"
	"aura/mediux"
	"aura/utils"
	"aura/utils/httpx"
	"net/http"
)

// GetMediuxImage godoc
// @Summary      Get Mediux Image
// @Description  Get an image from Mediux by asset ID, modified date and quality
// @Tags         Images
// @Produce      image/jpeg
// @Param        asset_id   query     string  true  "Asset ID of the image to fetch"
// @Param        modified_date   query     string  true  "Modified date of the image in ISO format (e.g. 2024-01-01T12:00:00Z)"
// @Param        quality   query     string  false  "Quality of the image to fetch (thumb, optimized, original). Default is thumb."
// @Success      200  {string}  string "Image data in JPEG format"
// @Failure      500           {object}  httpx.JSONResponse "Internal Server Error"
// @Router       /api/images/mediux/item [get]
func GetMediuxImage(w http.ResponseWriter, r *http.Request) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Get Mediux Image", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, logAction)

	assetID := r.URL.Query().Get("asset_id")
	modifiedDate := r.URL.Query().Get("modified_date")
	quality := r.URL.Query().Get("quality") // thumb, optimized, original

	if quality == "" {
		quality = "thumb"
	}
	if quality != "thumb" && quality != "optimized" && quality != "original" {
		logAction.SetError("Invalid quality parameter", "quality must be one of: thumb, optimized, original", map[string]any{
			"quality": quality,
		})
		httpx.SendResponse(w, ld, nil)
		return
	}

	formatDate := utils.ConvertDateStringToTime(modifiedDate).Format("20060102150405")

	var imageQuality mediux.ImageQuality
	switch quality {
	case "thumb":
		imageQuality = mediux.ImageQualityThumb
	case "optimized":
		imageQuality = mediux.ImageQualityOptimized
	case "original":
		imageQuality = mediux.ImageQualityOriginal
	}

	imageData, imageType, Err := mediux.GetImage(ctx, assetID, formatDate, imageQuality)
	if Err.Message != "" {
		httpx.SendResponse(w, ld, nil)
		return
	} else if len(imageData) == 0 {
		logAction.SetError("Image not found", "No image found for the provided asset ID and modified date",
			map[string]any{
				"asset_id":      assetID,
				"modified_date": modifiedDate,
				"quality":       quality,
			})
		httpx.SendResponse(w, ld, nil)
		return
	}

	w.Header().Set("Content-Type", imageType)
	w.WriteHeader(http.StatusOK)
	w.Write(imageData)
}

// GetMediuxAvatarImage godoc
// @Summary      Get Mediux Avatar Image
// @Description  Get a user avatar image from Mediux by avatar ID or username
// @Tags         Images
// @Produce      image/jpeg
// @Param        avatar_id   query     string  false  "Avatar ID of the user to fetch the avatar for"
// @Param        username    query     string  false  "Username of the user to fetch the avatar for (only used if avatar_id is not provided)"
// @Success      200  {string}  string "Avatar image data in JPEG format"
// @Failure      500           {object}  httpx.JSONResponse "Internal Server Error"
// @Router       /api/images/mediux/avatar [get]
func GetMediuxAvatarImage(w http.ResponseWriter, r *http.Request) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Get Mediux Avatar Image", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, logAction)

	avatarID := r.URL.Query().Get("avatar_id")
	username := r.URL.Query().Get("username")

	if avatarID == "" && username == "" {
		logAction.SetError("Missing parameters", "Either avatar_id or username must be provided",
			map[string]any{
				"avatar_id": avatarID,
				"username":  username,
			})
		httpx.SendResponse(w, ld, nil)
		return
	} else if avatarID == "" && username != "" {
		// Lookup avatar ID from username
		cachedUser, found := cache.MediuxUsers.GetMediuxUserByUsername(username)
		if found && cachedUser.Avatar != "" {
			avatarID = cachedUser.Avatar
		} else {
			logAction.SetError("User not found", "No user found with the provided username",
				map[string]any{
					"username": username,
				})
			httpx.SendResponse(w, ld, nil)
			return
		}
	}

	imageData, imageType, Err := mediux.GetAvatarImage(ctx, avatarID)
	if Err.Message != "" {
		httpx.SendResponse(w, ld, nil)
		return
	} else if len(imageData) == 0 {
		logAction.SetError("Avatar image not found", "No avatar image found for the provided avatar ID",
			map[string]any{
				"avatar_id": avatarID,
			})
		httpx.SendResponse(w, ld, nil)
		return
	}

	w.Header().Set("Content-Type", imageType)
	w.WriteHeader(http.StatusOK)
	w.Write(imageData)

}
