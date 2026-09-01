package routes_images

import (
	"aura/cache"
	"aura/config"
	"aura/logging"
	"aura/mediaserver"
	"aura/utils/httpx"
	"net/http"
	"strconv"
)

const (
	versionedImageCacheControl   = "private, max-age=86400"
	unversionedImageCacheControl = "private, no-store"
)

func setMediaImageCacheControl(w http.ResponseWriter, r *http.Request, artworkVersion int64) {
	if config.Current.MediaServer.Type == "Plex" && artworkVersion > 0 && r.URL.Query().Get("v") == strconv.FormatInt(artworkVersion, 10) {
		w.Header().Set("Cache-Control", versionedImageCacheControl)
		return
	}
	w.Header().Set("Cache-Control", unversionedImageCacheControl)
}

// GetMediaItemImage godoc
// @Summary      Get Media Item Image
// @Description  Get a media item image (poster, backdrop or thumb) from the media server by rating key and image type
// @Tags         Images
// @Produce      image/jpeg
// @Param        rating_key   query     string  true  "Rating Key of the media item"
// @Param        image_rating_key   query     string  false  "Rating Key of the specific image to fetch (if different from the media item rating key)"
// @Param        image_type   query     string  true  "Type of image to fetch (poster, backdrop or thumb)"
// @Success      200  {string}  string "Image data in JPEG format"
// @Failure      500           {object}  httpx.JSONResponse "Internal Server Error"
// @Router       /api/images/media/item [get]
func GetMediaItemImage(w http.ResponseWriter, r *http.Request) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Get Media Item Image From Media Server", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, logAction)

	actionGetQueryParams := logAction.AddSubAction("Get Query Params", logging.LevelDebug)
	ratingKey := r.URL.Query().Get("rating_key")
	imageRatingKey := r.URL.Query().Get("image_rating_key")
	imageType := r.URL.Query().Get("image_type")
	if ratingKey == "" || imageType == "" {
		actionGetQueryParams.SetError("Missing Query Parameters", "One or more required query parameters are missing",
			map[string]any{
				"rating_key":       ratingKey,
				"image_rating_key": imageRatingKey,
				"image_type":       imageType,
			})
		httpx.SendResponse(w, ld, nil)
		return
	} else if imageType != "poster" && imageType != "backdrop" && imageType != "thumb" {
		actionGetQueryParams.SetError("Invalid Query Parameters", "Image type must be either 'poster', 'backdrop', or 'thumb'",
			map[string]any{
				"rating_key":       ratingKey,
				"image_rating_key": imageRatingKey,
				"image_type":       imageType,
			})
		httpx.SendResponse(w, ld, nil)
		return
	}
	if imageRatingKey == "" {
		imageRatingKey = ratingKey
	}
	actionGetQueryParams.Complete()

	// Get the matching media item from the cache
	item, found := cache.LibraryStore.GetMediaItemByRatingKey(ratingKey)
	if !found {
		logAction.SetError("Media Item Not Found", "No media item found matching the provided rating key",
			map[string]any{
				"rating_key": ratingKey,
			})
		httpx.SendResponse(w, ld, nil)
		return
	}

	// If the image does not exist, then get it from the media server
	imageData, Err := mediaserver.GetMediaItemImage(ctx, item, imageRatingKey, imageType)
	if Err.Message != "" {
		httpx.SendResponse(w, ld, nil)
		return
	}

	// Set the content type for the response
	w.Header().Set("Content-Type", "image/jpeg")
	setMediaImageCacheControl(w, r, item.ArtworkVersion)
	// Write the image data to the response
	w.WriteHeader(http.StatusOK)
	w.Write(imageData)
}

// GetCollectionItemImage godoc
// @Summary      Get Collection Item Image
// @Description  Get a collection item image (poster or backdrop) from the media server by rating key and image type
// @Tags         Images
// @Produce      image/jpeg
// @Param        rating_key   query     string  true  "Rating Key of the collection item"
// @Param        image_type   query     string  true  "Type of image to fetch (poster or backdrop)"
// @Success      200  {string}  string "Image data in JPEG format"
// @Failure      500           {object}  httpx.JSONResponse "Internal Server Error"
// @Router       /api/images/media/collection [get]
func GetCollectionItemImage(w http.ResponseWriter, r *http.Request) {
	ctx, ld := logging.CreateLoggingContext(r.Context(), r.URL.Path)
	logAction := ld.AddAction("Get Collection Item Image From Media Server", logging.LevelInfo)
	ctx = logging.WithCurrentAction(ctx, logAction)

	actionGetQueryParams := logAction.AddSubAction("Get Query Params", logging.LevelDebug)
	ratingKey := r.URL.Query().Get("rating_key")
	imageType := r.URL.Query().Get("image_type")
	if ratingKey == "" || imageType == "" {
		actionGetQueryParams.SetError("Missing Query Parameters", "One or more required query parameters are missing",
			map[string]any{
				"rating_key": ratingKey,
				"image_type": imageType,
			})
		httpx.SendResponse(w, ld, nil)
		return
	} else if imageType != "poster" && imageType != "backdrop" {
		actionGetQueryParams.SetError("Invalid Query Parameters", "Image type must be either 'poster' or 'backdrop'",
			map[string]any{
				"rating_key": ratingKey,
				"image_type": imageType,
			})
		httpx.SendResponse(w, ld, nil)
		return
	}
	actionGetQueryParams.Complete()

	// Get the matching collection item from the cache
	item, found := cache.CollectionsStore.GetCollectionByRatingKey(ratingKey)
	if !found {
		logAction.SetError("Collection Item Not Found", "No collection item found matching the provided rating key in the cache",
			map[string]any{
				"rating_key": ratingKey,
			})
		httpx.SendResponse(w, ld, nil)
		return
	}

	imageData, Err := mediaserver.GetCollectionItemImage(ctx, item, imageType)
	if Err.Message != "" {
		httpx.SendResponse(w, ld, nil)
		return
	}

	// Set the content type for the response
	w.Header().Set("Content-Type", "image/jpeg")
	setMediaImageCacheControl(w, r, item.ArtworkVersion)
	// Write the image data to the response
	w.WriteHeader(http.StatusOK)
	w.Write(imageData)
}
