import type { CollectionItem } from "@/types/media-and-posters/collection-item";
import type { MediaItem } from "@/types/media-and-posters/media-item-and-library";

const titleYearSuffixRe = /\s*\((\d{4})\)\s*$/;

export function mediaItemInfo(item: MediaItem): string {
  let title = item.title;
  let year = item.year;

  // If title ends with "(YYYY)", pull that year and strip it from the title.
  const match = title.match(titleYearSuffixRe);
  if (match && match.length === 2) {
    const y = parseInt(match[1], 10);
    if (!isNaN(y)) {
      year = y;
      title = title.replace(titleYearSuffixRe, "").trim();
    }
  }

  return `'${title}' (${year}) | ${item.library_title} [TMDB: ${item.tmdb_id} | Key: ${item.rating_key}]`;
}

export function collectionItemInfo(item: CollectionItem): string {
  return `'${item.title}' | ${item.library_title} [Key: ${item.rating_key}]`;
}
