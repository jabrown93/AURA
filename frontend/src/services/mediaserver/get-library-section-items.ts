import apiClient from "@/services/api-client";
import { ReturnErrorMessage } from "@/services/api-error-return";
import {
  buildLibraryItemsParams,
  collectLibraryPages,
  type LibraryItemsQuery,
} from "@/services/mediaserver/collect-library-pages";

import { log } from "@/lib/logger";

import type { APIResponse } from "@/types/api/api-response";
import type { MediaItem } from "@/types/media-and-posters/media-item-and-library";

export interface GetLibrarySectionItems_Response {
  media_items: MediaItem[];
  total_items: number;
  has_updated_at: boolean;
  has_episode_added_at: boolean;
}

export const GetLibrarySectionItems = async (
  query: LibraryItemsQuery
): Promise<APIResponse<GetLibrarySectionItems_Response>> => {
  log("INFO", "API - Media Server", "Fetch Library Items", `Fetching library page ${query.pageNumber}`);
  try {
    const params = buildLibraryItemsParams(query);
    const response = await apiClient.get<APIResponse<GetLibrarySectionItems_Response>>(`/mediaserver/library/items`, {
      params,
    });
    if (response.data.status === "error") {
      throw new Error(response.data.error?.message || "Unknown error fetching library items");
    }
    log(
      "INFO",
      "API - Media Server",
      "Fetch Library Items",
      `Fetched ${response.data.data?.media_items.length ?? 0} library items`
    );
    return response.data;
  } catch (error) {
    log(
      "ERROR",
      "API - Media Server",
      "Fetch Library Items",
      `Failed to fetch library items: ${error instanceof Error ? error.message : "Unknown error"}`
    );
    return ReturnErrorMessage<GetLibrarySectionItems_Response>(error);
  }
};

export const GetCompleteLibrarySection = async (
  libraryTitle: string
): Promise<APIResponse<GetLibrarySectionItems_Response>> => {
  let hasUpdatedAt = false;
  let hasEpisodeAddedAt = false;
  let totalItems = 0;
  try {
    const mediaItems = await collectLibraryPages(async (pageNumber) => {
      const response = await GetLibrarySectionItems({
        libraryTitles: [libraryTitle],
        searchTitle: "",
        searchLibrary: "",
        searchID: "",
        searchYear: 0,
        filterInDB: "",
        filterIgnored: "",
        filterHasSets: "",
        sortOption: "title",
        sortOrder: "asc",
        pageNumber,
        itemsPerPage: 1000,
      });
      if (response.status === "error" || !response.data) {
        throw new Error(response.error?.message || `Failed to fetch complete library '${libraryTitle}'`);
      }
      totalItems = response.data.total_items;
      hasUpdatedAt ||= response.data.has_updated_at;
      hasEpisodeAddedAt ||= response.data.has_episode_added_at;
      return { items: response.data.media_items, totalItems };
    });
    return {
      status: "success",
      data: {
        media_items: mediaItems,
        total_items: totalItems,
        has_updated_at: hasUpdatedAt,
        has_episode_added_at: hasEpisodeAddedAt,
      },
    };
  } catch (error) {
    return ReturnErrorMessage<GetLibrarySectionItems_Response>(error);
  }
};
