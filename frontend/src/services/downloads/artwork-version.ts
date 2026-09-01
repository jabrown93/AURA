export function updateArtworkVersion<T extends { updated_at: number }>(
  item: T,
  updatedAt?: number,
  onArtworkApplied?: (item: T) => void
): T {
  if (updatedAt !== undefined && updatedAt > item.updated_at) item.updated_at = updatedAt;
  onArtworkApplied?.(item);
  return item;
}
