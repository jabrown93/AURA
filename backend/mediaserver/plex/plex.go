package plex

type PlexConnectionInfoWrapper struct {
	MediaContainer PlexConnectionInfo `json:"MediaContainer"`
}

type PlexConnectionInfo struct {
	Size              int    `json:"size"`
	APIVersion        string `json:"apiVersion"`
	Claimed           bool   `json:"claimed"`
	MachineIdentifier string `json:"machineIdentifier"`
	Version           string `json:"version"`
	FriendlyName      string `json:"friendlyName"`
}

////////////////////////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////////////////////

type PlexLibrarySectionsWrapper struct {
	MediaContainer PlexLibrarySections `json:"MediaContainer"`
}

type PlexLibrarySections struct {
	Size      int                            `json:"size"`
	Directory []PlexLibrarySectionsDirectory `json:"Directory"`
}

type PlexLibrarySectionsDirectory struct {
	Key      string         `json:"key"`
	Type     string         `json:"type"`
	Title    string         `json:"title"`
	Location []PlexLocation `json:"Location"`
}

////////////////////////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////////////////////

type PlexLibraryItemsWrapper struct {
	MediaContainer PlexLibraryItems `json:"MediaContainer"`
}

type PlexLibraryItems struct {
	LibrarySectionID    int                        `json:"librarySectionID"`
	LibrarySectionTitle string                     `json:"title1"`
	Metadata            []PlexLibraryItemsMetadata `json:"Metadata"`
	Offset              int                        `json:"offset"`
	Size                int                        `json:"size"`
	TotalSize           int                        `json:"totalSize"`
	ViewGroup           string                     `json:"viewGroup"`
}

type PlexLibraryItemsMetadata struct {
	// Shared Fields
	RatingKey             string            `json:"ratingKey"`
	Key                   string            `json:"key"`
	Slug                  string            `json:"slug"`
	Studio                string            `json:"studio"`
	Type                  string            `json:"type"` // "movie", "show", "season", "episode"
	Title                 string            `json:"title"`
	OriginalTitle         string            `json:"originalTitle"`
	Guid                  string            `json:"guid"`
	ContentRating         string            `json:"contentRating"` // "PG-13", "R", etc
	ContentRatingAge      int               `json:"contentRatingAge"`
	Summary               string            `json:"summary"`
	AudienceRating        float64           `json:"audienceRating"` // TMDB Rating
	ViewCount             int               `json:"viewCount"`
	LastViewedAt          int64             `json:"lastViewedAt"`
	Year                  int               `json:"year"`
	Tagline               string            `json:"tagline"`
	Thumb                 string            `json:"thumb"`
	Art                   string            `json:"art"`
	Duration              int64             `json:"duration"`
	OriginallyAvailableAt string            `json:"originallyAvailableAt"`
	AddedAt               int64             `json:"addedAt"`
	UpdatedAt             int64             `json:"updatedAt"`
	Images                []PlexImage       `json:"Image,omitempty"`
	UltraBlurColors       *UltraBlurColors  `json:"UltraBlurColors,omitempty"`
	Guids                 []PlexTagField    `json:"Guid"`
	Genres                []PlexTagFieldInt `json:"Genre,omitempty"`
	Countries             []PlexTagFieldInt `json:"Country,omitempty"`
	Roles                 []PlexRoleField   `json:"Role,omitempty"`
	Edition               string            `json:"editionTitle,omitempty"`

	// Series Specific
	Index           int `json:"index,omitempty"`
	LeafCount       int `json:"leafCount,omitempty"` // Number of Episodes
	ViewedLeafCount int `json:"viewedLeafCount"`
	ChildCount      int `json:"childCount"` // Number of Seasons

	// Media Specific (Movies & Episodes)
	TitleSort string               `json:"titleSort,omitempty"`
	SkipCount int                  `json:"skipCount,omitempty"`
	Directors []PlexTagFieldInt    `json:"Director,omitempty"`
	Writers   []PlexTagFieldInt    `json:"Writer,omitempty"`
	Media     []PlexVideoMediaItem `json:"Media,omitempty"`

	// MediaItem Specific
	LibrarySectionID    int    `json:"librarySectionID"`
	LibrarySectionTitle string `json:"librarySectionTitle"`
	LibrarySectionKey   string `json:"librarySectionKey"`

	// Series Item Specific
	Rating      float64           `json:"rating,omitempty"` // IMDB Rating
	UserRating  float64           `json:"userRating,omitempty"`
	LastRatedAt int64             `json:"lastRatedAt,omitempty"`
	Theme       string            `json:"theme,omitempty"`
	Ratings     []PlexRatings     `json:"Rating,omitempty"`
	Collections []PlexTagFieldInt `json:"Collection,omitempty"`
	Labels      []PlexTagFieldInt `json:"Label,omitempty"`
	Location    []PlexLocation    `json:"Location,omitempty"`

	// Season & Episode Specific
	ParentRatingKey string `json:"parentRatingKey,omitempty"`

	// Episode Specific
	GrandParentRatingKey string `json:"grandparentRatingKey,omitempty"`
	ParentTitle          string `json:"parentTitle,omitempty"`
	ParentIndex          int    `json:"parentIndex,omitempty"`
}

type PlexLocation struct {
	ID   int    `json:"id"`
	Path string `json:"path"`
}

type PlexVideoMediaItem struct {
	ID               int                 `json:"id"`
	Duration         int64               `json:"duration"`
	Bitrate          int64               `json:"bitrate"`
	Width            int64               `json:"width"`
	Height           int64               `json:"height"`
	AspectRatio      float64             `json:"aspectRatio"`
	AudioChannels    int64               `json:"audioChannels"`
	AudioCodec       string              `json:"audioCodec"`
	VideoCodec       string              `json:"videoCodec"`
	Container        string              `json:"container"`
	VideoFrameRate   string              `json:"videoFrameRate"`
	VideoProfile     string              `json:"videoProfile"`
	HasVoiceActivity bool                `json:"hasVoiceActivity"`
	Part             []PlexVideoPartItem `json:"Part"`
}

type PlexVideoPartItem struct {
	ID           int    `json:"id"`
	Key          string `json:"key"`
	Duration     int64  `json:"duration"`
	File         string `json:"file"`
	Size         int64  `json:"size"`
	Container    string `json:"container"`
	VideoProfile string `json:"videoProfile"`
}

type PlexTagFieldInt struct {
	ID     int    `json:"id,omitempty"`
	Filter string `json:"filter,omitempty"`
	Tag    string `json:"tag"`
}

type PlexTagField struct {
	ID     string `json:"id,omitempty"`
	Filter string `json:"filter,omitempty"`
	Tag    string `json:"tag"`
}

type PlexRoleField struct {
	PlexTagFieldInt
	Role  string `json:"role"`
	Thumb string `json:"thumb,omitempty"`
}

type PlexImage struct {
	Alt  string `json:"alt"`
	Type string `json:"type"`
	URL  string `json:"url"`
}

type UltraBlurColors struct {
	TopLeft     string `json:"topLeft"`
	TopRight    string `json:"topRight"`
	BottomRight string `json:"bottomRight"`
	BottomLeft  string `json:"bottomLeft"`
}

type PlexRatings struct {
	Image string  `json:"image"` // Use this to get the provider name as well
	Value float64 `json:"value"`
	Type  string  `json:"type"`
}

// //////////////////////////////////////////////////////////////////////////////////////////
// //////////////////////////////////////////////////////////////////////////////////////////
type PlexGetAllImagesWrapper struct {
	MediaContainer PlexGetAllImages `json:"MediaContainer"`
}

type PlexGetAllImages struct {
	Size            int                        `json:"size"`
	Identifier      string                     `json:"identifier"`
	MediaTagPrefix  string                     `json:"mediaTagPrefix"`
	MediaTagVersion int64                      `json:"mediaTagVersion"`
	Metadata        []PlexGetAllImagesMetadata `json:"Metadata"`
}

type PlexGetAllImagesMetadata struct {
	Key       string `json:"key"`
	RatingKey string `json:"ratingKey"`
	Thumb     string `json:"thumb"`
	Selected  bool   `json:"selected"`
	Provider  string `json:"provider"`
}

// //////////////////////////////////////////////////////////////////////////////////////////
// //////////////////////////////////////////////////////////////////////////////////////////

type PlexSearchResponseWrapper struct {
	MediaContainer PlexSearchResponse `json:"MediaContainer"`
}

type PlexSearchResponse struct {
	Size          int                        `json:"size"`
	SearchResults []PlexSearchResultResponse `json:"SearchResult"`
}

type PlexSearchResultResponse struct {
	Metadata []PlexLibraryItemsMetadata `json:"Metadata"`
}

////////////////////////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////////////////////

type PlexServersResponse struct {
	Name        string                  `json:"name"`
	Owned       bool                    `json:"owned"`
	Connections []PlexServerConnections `json:"connections"`
}

type PlexServerConnections struct {
	Protocol string `json:"protocol"`
	Address  string `json:"address"`
	Port     int    `json:"port"`
	Uri      string `json:"uri"`
	Local    bool   `json:"local"`
	Relay    bool   `json:"relay"`
	IPv6     bool   `json:"ipv6"`
}
