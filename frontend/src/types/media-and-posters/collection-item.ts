import type { MediaItem } from "@/types/media-and-posters/media-item-and-library";

// A native media-server Collection (e.g. "James Bond Collection") — a library
// grouping of media items with its own artwork. Mirrors the backend
// models.CollectionItem.
//
// This was previously defined in @/app/collections/page; it lives here so shared
// code (services, stores, types, components) can import it without depending on a
// client page module.
export interface CollectionItem {
  rating_key: string;
  index: string;
  title: string;
  summary?: string;
  child_count: number;
  media_items: MediaItem[];
  library_title?: string;
}
