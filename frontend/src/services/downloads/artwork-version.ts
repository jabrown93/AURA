export function updateArtworkVersion<T extends { updated_at: number }>(item: T, updatedAt?: number): T {
  if (updatedAt !== undefined && updatedAt > item.updated_at) item.updated_at = updatedAt;
  return item;
}
