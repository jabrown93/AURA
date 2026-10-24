import apiClient from "@/services/api-client";
import { ReturnErrorMessage } from "@/services/api-error-return";

import { log } from "@/lib/logger";

import type { APIResponse } from "@/types/api/api-response";
import type { IncludedItem, SetRef } from "@/types/media-and-posters/sets";
import type { TYPE_DB_SET_TYPE_OPTIONS } from "@/types/ui-options";

export interface GetSetByID_Response {
  set: SetRef;
  included_items: { [tmdb_id: string]: IncludedItem };
}

export const GetSetByID = async (
  itemLibraryTitle: string,
  tmdbID: string,
  setID: string,
  setType: TYPE_DB_SET_TYPE_OPTIONS,
  edition: string = ""
): Promise<APIResponse<GetSetByID_Response>> => {
  log(
    "INFO",
    "API - MediUX",
    "Fetch Set By ID",
    `Fetching set by ID: ${setID} for setType: ${setType}, itemLibraryTitle: ${itemLibraryTitle}, tmdbID: ${tmdbID}`
  );
  try {
    const params = {
      set_id: setID,
      set_type: setType,
      item_library_title: itemLibraryTitle,
      tmdb_id: tmdbID,
      edition,
    };

    const response = await apiClient.get<APIResponse<GetSetByID_Response>>(`/mediux/set`, { params });
    if (response.data.status === "error") {
      throw new Error(response.data.error?.message || `Unknown error fetching set by ID: ${setID}`);
    } else {
      log("INFO", "API - MediUX", "Fetch Set By ID", `Fetched set by ID: ${setID} successfully`, response.data);
    }
    return response.data;
  } catch (error) {
    log(
      "ERROR",
      "API - MediUX",
      "Fetch Set By ID",
      `Failed to fetch set by ID: ${setID}: ${error instanceof Error ? error.message : "Unknown error"}`,
      error
    );
    return ReturnErrorMessage<GetSetByID_Response>(error);
  }
};
