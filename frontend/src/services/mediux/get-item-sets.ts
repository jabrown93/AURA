import apiClient from "@/services/api-client";
import { ReturnErrorMessage } from "@/services/api-error-return";

import { log } from "@/lib/logger";

import type { APIResponse } from "@/types/api/api-response";
import type { PosterSetsResponse } from "@/types/media-and-posters/sets";

export const GetItemSets = async (
  tmdbID: string,
  itemType: string,
  itemLibraryTitle: string,
  edition: string = ""
): Promise<APIResponse<PosterSetsResponse>> => {
  log(
    "INFO",
    "API - MediUX",
    "Fetch Item Sets",
    `Fetching sets for tmdbID: ${tmdbID}, itemType: ${itemType}, itemLibraryTitle: ${itemLibraryTitle}`
  );
  try {
    const params = {
      tmdb_id: tmdbID,
      item_type: itemType,
      item_library_title: itemLibraryTitle,
      edition,
    };

    const response = await apiClient.get<APIResponse<PosterSetsResponse>>(`/mediux/sets/item`, { params });
    if (response.data.status === "error") {
      throw new Error(response.data.error?.message || `Unknown error fetching sets for tmdbID: ${tmdbID}`);
    } else {
      log("INFO", "API - MediUX", "Fetch Item Sets", `Fetched sets for tmdbID: ${tmdbID} successfully`, response.data);
    }
    return response.data;
  } catch (error) {
    log(
      "ERROR",
      "API - MediUX",
      "Fetch Item Sets",
      `Failed to fetch sets for tmdbID: ${tmdbID}: ${error instanceof Error ? error.message : "Unknown error"}`,
      error
    );
    return ReturnErrorMessage<PosterSetsResponse>(error);
  }
};
