import type { MediaItem, SelectedTypes } from "@/types/media-and-posters/media-item-and-library";
import type { BaseSetInfo, ImageFile } from "@/types/media-and-posters/sets";
import type { TYPE_DB_SET_TYPE_OPTIONS } from "@/types/ui-options";

// What is used to save a record into the database
// This contains the MediaItem details, as well as an array of PosterSets that are associated with it
export interface DBSavedItem {
  //tmdb_id: string;
  //library_title: string;
  media_item: MediaItem;
  poster_sets: DBPosterSetDetail[];
  // Queue-only failure metadata, populated by the download-queue processor when an
  // entry is moved to the error/warning state. Absent on all other DBSavedItem uses.
  queue_errors?: string[];
  queue_warnings?: string[];
  failed_at?: string;
}

export interface DBPosterSetDetail extends PosterSet {
  last_downloaded: string;
  selected_types: SelectedTypes;
  auto_download: boolean;
  auto_add_new_collection_items: boolean;
  to_delete: boolean; // Flag to indicate if the poster set should be deleted (Not used in DB)
}

export interface PosterSet extends Omit<BaseSetInfo, "type"> {
  type: TYPE_DB_SET_TYPE_OPTIONS;
  images: ImageFile[];
}

export interface DBFilter {
  item_tmdb_id: string;
  item_library_title: string;
  item_year: number;
  item_title: string;
  set_id: number;
  library_titles: string[];
  image_types: string[];
  autodownload: string;
  multiset_only: boolean;
  usernames: string[];
  page_items: number;
  page_number: number;
  sort_option: string;
  sort_order: string;
}
