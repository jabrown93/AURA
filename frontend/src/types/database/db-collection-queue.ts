import type { CollectionItem } from "@/types/media-and-posters/collection-item";
import type { CollectionItemImageFile } from "@/types/media-and-posters/sets";

// Mirror of the backend models.CollectionQueueItem. A queued request to download
// and apply one or more MediUX images (poster/backdrop only) to a native
// media-server Collection. Distinct from DBSavedItem, which represents a
// movie/show media item + MediUX poster sets.
export interface CollectionQueueItem {
  collection_item: CollectionItem;
  images: CollectionItemImageFile[];
  // Queue-only failure metadata, populated by the backend when an entry is moved
  // to the error/warning state. Absent on in-progress entries.
  queue_errors?: string[];
  queue_warnings?: string[];
  failed_at?: string;
}
