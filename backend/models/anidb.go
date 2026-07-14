package models

// AnidbMapping is a cross-reference for a single AniDB entry, derived from the
// Fribb anime-lists dataset. It lets AURA resolve anime items that Plex matched
// with the HAMA agent (which yields only an AniDB ID) to a TMDB ID. IDs are
// stored as strings and are empty when the dataset has no value for them.
type AnidbMapping struct {
	AnidbID     string `json:"anidb_id"`
	Type        string `json:"type"`          // Fribb type, e.g. "TV", "MOVIE", "ONA"
	TMDBTvID    string `json:"tmdb_tv_id"`    // TMDB TV id (for series)
	TMDBMovieID string `json:"tmdb_movie_id"` // TMDB movie id (for films)
	TVDBID      string `json:"tvdb_id"`       // TheTVDB id, used as a fallback via MediUX
	IMDBID      string `json:"imdb_id"`
}
