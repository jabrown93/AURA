export interface LibrarySectionBase {
  id: string;
  type: string; // "movie" or "show"
  title: string;
  paths?: string[];
}

export interface LibrarySection extends LibrarySectionBase {
  total_size: number;
  media_items: MediaItem[];
}

export interface MediaItem {
  tmdb_id: string;
  library_title: string;
  edition: string;
  rating_key: string;
  type: "show" | "movie";
  title: string;
  year: number;
  movie?: MediaItemMovie;
  series?: MediaItemSeries;

  db_saved_sets: DBSavedSet[];
  ignored_in_db: boolean;
  ignored_mode: string;

  has_mediux_sets: boolean;
  updated_at: number;
  added_at: number;
  released_at: number;
  latest_episode_added_at: number;

  guids: Guid[];

  content_rating: string;
  summary: string;
}

export interface Guid {
  provider?: string;
  id?: string;
  rating?: string;
}

export interface MediaItemMovie {
  file: MediaItemFile;
}

export interface MediaItemSeries {
  seasons: MediaItemSeason[];
  season_count: number;
  episode_count: number;
  location: string;
}

export interface MediaItemSeason {
  rating_key: string;
  season_number: number;
  title: string;
  episodes: MediaItemEpisode[];
}

export interface MediaItemEpisode {
  rating_key: string;
  title: string;
  season_number: number;
  episode_number: number;
  added_at: number;
  file: MediaItemFile;
}

export interface MediaItemFile {
  path: string;
  size: number;
  duration: number;
}

export interface DBSavedSet {
  id: string;
  user_created: string;
  selected_types: SelectedTypes;
}

export interface SelectedTypes {
  poster: boolean;
  backdrop: boolean;
  season_poster: boolean;
  special_season_poster: boolean;
  titlecard: boolean;
}
