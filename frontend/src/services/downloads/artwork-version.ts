export function updateArtworkVersion<T extends { artwork_version: number }>(
  item: T,
  artworkVersion?: number,
  onArtworkApplied?: (item: T) => void
): T {
  if (artworkVersion !== undefined && artworkVersion > item.artwork_version) item.artwork_version = artworkVersion;
  onArtworkApplied?.(item);
  return item;
}
