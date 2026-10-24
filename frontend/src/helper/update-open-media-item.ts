import { removeSavedSet, upsertSavedSets } from "@/helper/media-item-update-saved-sets";

import { useMediaStore } from "@/lib/stores/global-store-media-store";

import type { MediaItem, SelectedTypes } from "@/types/media-and-posters/media-item-and-library";

interface ItemKey {
  tmdbID: string;
  libraryTitle: string;
  edition?: string;
}

// Changes the open media item, if it is this item
const updateOpenMediaItem = (
  { tmdbID, libraryTitle, edition = "" }: ItemKey,
  change: (item: MediaItem) => MediaItem
) => {
  const { mediaItem, setMediaItem } = useMediaStore.getState();
  if (
    mediaItem?.tmdb_id === tmdbID &&
    mediaItem.library_title === libraryTitle &&
    (mediaItem.edition || "") === edition
  ) {
    setMediaItem(change(mediaItem));
  }
};

// Shows a set as saved, or removed, on the open media item
export const markOpenMediaItemSetSaved = (
  args: ItemKey & { setID: string; setUser: string; selectedTypes: SelectedTypes; toDelete?: boolean }
) =>
  updateOpenMediaItem(args, (item) =>
    args.toDelete
      ? removeSavedSet(item, args.setID)
      : upsertSavedSets(item, args.setID, args.setUser, args.selectedTypes)
  );

// Shows the open media item as having no saved sets
export const clearOpenMediaItemSavedSets = (args: ItemKey) =>
  updateOpenMediaItem(args, (item) => ({ ...item, db_saved_sets: [] }));
