import { type ImageTypeLabelInput, formatImageTypeLabel } from "@/helper/format-image-type";

// The text shown while an image downloads, e.g. "Downloading Season 01 Poster for 'Title' (3/12)"
export const formatDownloadProgressText = (
  image: ImageTypeLabelInput,
  title: string,
  current: number,
  total: number
): string => `Downloading ${formatImageTypeLabel(image) || "Image"} for '${title}' (${current}/${total})`;

// The same text for narrow screens
export const formatDownloadProgressShortText = (image: ImageTypeLabelInput): string =>
  `Downloading ${formatImageTypeLabel(image) || "Image"}`;

// Where an image comes in the download order: each season's poster, then its titlecards by episode, specials last
const seasonImageOrder = (image: { type: string; season_number?: number; episode_number?: number }) => [
  image.season_number === undefined ? -1 : image.season_number === 0 ? Number.MAX_SAFE_INTEGER : image.season_number,
  image.type === "season_poster" ? 0 : 1,
  image.episode_number ?? 0,
];

// Sorts season posters and titlecards into the order they are downloaded, the same as the Download Queue
export const compareSeasonImages = (
  a: { type: string; season_number?: number; episode_number?: number },
  b: { type: string; season_number?: number; episode_number?: number }
): number => {
  const orderA = seasonImageOrder(a);
  const orderB = seasonImageOrder(b);
  for (let i = 0; i < orderA.length; i++) {
    if (orderA[i] !== orderB[i]) return orderA[i] - orderB[i];
  }
  return 0;
};
