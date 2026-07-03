// Identifiers for Kometa-imported saved sets and locally-imported asset images.
// These mirror the backend prefixes in backend/kometa/kometa.go.

const KOMETA_SET_ID_PREFIX = "kometa-";
const KOMETA_IMAGE_ID_PREFIX = "kometa|";

// isKometaSetId reports whether a poster set ID belongs to a Kometa asset import.
export function isKometaSetId(id: string | undefined | null): boolean {
  return !!id && id.startsWith(KOMETA_SET_ID_PREFIX);
}

// isKometaImageId reports whether an image ID refers to a locally-imported Kometa asset.
export function isKometaImageId(id: string | undefined | null): boolean {
  return !!id && id.startsWith(KOMETA_IMAGE_ID_PREFIX);
}

// kometaImageSrc builds the URL that serves a locally-imported Kometa asset image.
export function kometaImageSrc(id: string): string {
  return `/api/images/kometa/item?asset_id=${encodeURIComponent(id)}`;
}
