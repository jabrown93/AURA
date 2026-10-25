package models

// ImageDownloadResult captures the outcome of downloading a single image
// during download queue processing, for persistence into download history.
type ImageDownloadResult struct {
	ImageType     string `json:"image_type"`
	SeasonNumber  *int   `json:"season_number,omitempty"`
	EpisodeNumber *int   `json:"episode_number,omitempty"`
	Success       bool   `json:"success"`
	FailureReason string `json:"failure_reason,omitempty"`
}
