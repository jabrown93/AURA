package mediux

import (
	"aura/config"
	"aura/models"
	"aura/utils"
	"path"
	"time"
)

type ImageQuality string

const (
	ImageQualityOriginal  ImageQuality = "original"
	ImageQualityOptimized ImageQuality = "optimized"
	ImageQualityThumb     ImageQuality = "thumb"
)

var ThumbsTempImageFolder string
var FullTempImageFolder string

func init() {
	ThumbsTempImageFolder = path.Join(config.ConfigPath, "temp-images", "mediux", "thumbs")
	FullTempImageFolder = path.Join(config.ConfigPath, "temp-images", "mediux", "full")
}

type ErrorResponse struct {
	Message    string          `json:"message"`
	Paths      []string        `json:"path,omitempty"`
	Extensions map[string]any  `json:"extensions,omitempty"`
	Locations  []ErrorLocation `json:"locations,omitempty"`
}

type ErrorLocation struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}

type ImageAsset struct {
	ID         string    `json:"id"`
	ModifiedOn time.Time `json:"modified_on"`
	Filesize   string    `json:"filesize"`
	Src        string    `json:"src"`
	Blurhash   *string   `json:"blurhash"`
	Language   Language  `json:"language"`
	Season     *Season   `json:"season"`
	Episode    *Episode  `json:"episode"`
}

type Season struct {
	SeasonNumber int `json:"season_number"`
}

type Episode struct {
	EpisodeTitle  string `json:"episode_title"`
	EpisodeNumber int    `json:"episode_number"`
	Season        Season `json:"season_id"`
}

type Language struct {
	DisplayName string `json:"display_name"`
	Iso639_1    string `json:"iso639_1"`
	Iso639_2    string `json:"iso639_2"`
	Iso639_3    string `json:"iso639_3"`
}

type SetUser struct {
	Username string `json:"username"`
}

type BaseItemInfo struct {
	ID           string    `json:"id"`
	DateUpdated  time.Time `json:"date_updated"`
	Status       string    `json:"status"`
	Title        string    `json:"title"`
	Tagline      string    `json:"tagline"`
	TvdbID       string    `json:"tvdb_id"`
	ImdbID       string    `json:"imdb_id"`
	TraktID      string    `json:"trakt_id"`
	Slug         string    `json:"slug"`
	PosterPath   string    `json:"poster_path"`
	BackdropPath string    `json:"backdrop_path"`
}

type BaseSetInfo struct {
	ID               string    `json:"id"`
	SetTitle         string    `json:"set_title"`
	BoxsetTitle      string    `json:"boxset_title,omitempty"`
	UserCreated      SetUser   `json:"user_created"`
	DateCreated      time.Time `json:"date_created"`
	DateUpdated      time.Time `json:"date_updated"`
	Popularity       int       `json:"popularity"`
	PopularityGlobal int       `json:"popularity_global"`
}

type BaseShowInfo struct {
	BaseItemInfo
	FirstAirDate string `json:"first_air_date"`
}

type BaseMovieInfo struct {
	BaseItemInfo
	ReleaseDate string `json:"release_date"`
}

/////////////////////////////////////////////////////////////////////////////////

// Convert MediUX BaseItemInfo to Set Response BaseItemInfo
func convertMediuxBaseItemToResponseBaseItem(mediuxItem BaseItemInfo, itemType string) models.BaseMediuxItemInfo {
	return models.BaseMediuxItemInfo{
		TMDB_ID:           mediuxItem.ID,
		Type:              itemType,
		DateUpdated:       mediuxItem.DateUpdated,
		Status:            mediuxItem.Status,
		Title:             mediuxItem.Title,
		Tagline:           mediuxItem.Tagline,
		TvdbID:            mediuxItem.TvdbID,
		ImdbID:            mediuxItem.ImdbID,
		TraktID:           mediuxItem.TraktID,
		Slug:              mediuxItem.Slug,
		TMDB_PosterPath:   mediuxItem.PosterPath,
		TMDB_BackdropPath: mediuxItem.BackdropPath,
	}
}

// Convert MediUX BaseSetInfo to Set Response BaseSetInfo
func convertMediuxBaseSetInfoToResponseBaseSetInfo(mediuxSet BaseSetInfo, setType string) models.BaseSetInfo {
	return models.BaseSetInfo{
		ID:               mediuxSet.ID,
		Title:            mediuxSet.SetTitle,
		Type:             setType,
		UserCreated:      mediuxSet.UserCreated.Username,
		DateCreated:      mediuxSet.DateCreated,
		DateUpdated:      mediuxSet.DateUpdated,
		Popularity:       mediuxSet.Popularity,
		PopularityGlobal: mediuxSet.PopularityGlobal,
	}
}

// Convert MediUX image assets to Response ImageFile
func convertMediuxImageAssetToImageFile(a *ImageAsset, imageType string) *models.ImageFile {
	if a == nil || a.ID == "" {
		return nil
	}

	imageFile := &models.ImageFile{
		ID:       a.ID,
		Type:     imageType,
		Modified: a.ModifiedOn,
		FileSize: utils.ParseFileSize(a.Filesize),
		Src:      a.Src,
	}

	if a.Blurhash != nil {
		imageFile.Blurhash = *a.Blurhash
	}

	// Handle Season Poster Season Number
	if a.Season != nil {
		imageFile.SeasonNumber = &a.Season.SeasonNumber
	}

	// Handle Title Card Season/Episode Number
	if a.Episode != nil {
		imageFile.Title = a.Episode.EpisodeTitle
		imageFile.EpisodeNumber = &a.Episode.EpisodeNumber
		imageFile.SeasonNumber = &a.Episode.Season.SeasonNumber
	}

	// Handle Language
	if a.Language != (Language{}) {
		imageFile.Language = a.Language.DisplayName
	} else {
		imageFile.Language = "English" // Default to English if no language provided
	}

	return imageFile
}

// Convert MediUX ShowSet to Set Response ShowSet
func convertMediuxShowImagesToImageFiles(set BaseMediuxShowSet, showTMDBID string) []models.ImageFile {
	images := []models.ImageFile{}

	// Poster(s)
	if len(set.ShowPoster) > 0 && set.ShowPoster[0].ID != "" {
		img := convertMediuxImageAssetToImageFile(&set.ShowPoster[0], "poster")
		if img != nil {
			img.ItemTMDB_ID = showTMDBID
			images = append(images, *img)
		}
	}

	// Backdrop(s)
	if len(set.ShowBackdrop) > 0 && set.ShowBackdrop[0].ID != "" {
		img := convertMediuxImageAssetToImageFile(&set.ShowBackdrop[0], "backdrop")
		if img != nil {
			img.ItemTMDB_ID = showTMDBID
			images = append(images, *img)
		}
	}

	// Season Posters
	for _, sp := range set.SeasonPosters {
		img := convertMediuxImageAssetToImageFile(&sp, "season_poster")
		if img != nil {
			img.ItemTMDB_ID = showTMDBID
			images = append(images, *img)
		}
	}

	// Titlecards
	for _, tc := range set.Titlecards {
		img := convertMediuxImageAssetToImageFile(&tc, "titlecard")
		if img != nil {
			img.ItemTMDB_ID = showTMDBID
			images = append(images, *img)
		}
	}

	return images
}

func convertMediuxMovieImagesToImageFiles(set BaseMediuxMovieSet, movieTMDBID string) []models.ImageFile {
	images := []models.ImageFile{}

	// Poster(s)
	if len(set.MoviePoster) > 0 && set.MoviePoster[0].ID != "" {
		img := convertMediuxImageAssetToImageFile(&set.MoviePoster[0], "poster")
		if img != nil {
			img.ItemTMDB_ID = movieTMDBID
			images = append(images, *img)
		}
	}

	// Backdrop(s)
	if len(set.MovieBackdrop) > 0 && set.MovieBackdrop[0].ID != "" {
		img := convertMediuxImageAssetToImageFile(&set.MovieBackdrop[0], "backdrop")
		if img != nil {
			img.ItemTMDB_ID = movieTMDBID
			images = append(images, *img)
		}
	}

	return images
}
