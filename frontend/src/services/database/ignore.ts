import apiClient from "@/services/api-client";
import { ReturnErrorMessage } from "@/services/api-error-return";

import { log } from "@/lib/logger";

import type { APIResponse } from "@/types/api/api-response";

export interface IgnoreItem_Response {
  ignored: boolean;
  tmdb_id: string;
  library_title: string;
  mode?: string;
}

export const StopIgnoringItemInDB = async (
  tmdbID: string,
  libraryTitle: string,
  edition: string = ""
): Promise<APIResponse<IgnoreItem_Response>> => {
  log(
    "INFO",
    "API - DB",
    "Stop Ignoring Item",
    `Stopping ignoring item TMDB_ID: ${tmdbID} in library: ${libraryTitle}`
  );
  try {
    const params = {
      tmdb_id: tmdbID,
      library_title: libraryTitle,
      edition,
    };
    const response = await apiClient.patch<APIResponse<IgnoreItem_Response>>(`/db/ignore/stop`, null, {
      params: params,
    });
    if (response.data.status === "error") {
      throw new Error(response.data.error?.message || "Unknown error stopping ignoring item in DB");
    } else {
      log(
        "INFO",
        "API - DB",
        "Stop Ignoring Item",
        `Stopped ignoring item TMDB_ID: ${tmdbID} in library: ${libraryTitle} successfully`,
        response.data
      );
    }
    return response.data;
  } catch (error) {
    log(
      "ERROR",
      "API - DB",
      "Stop Ignoring",
      `Failed to stop ignoring item in DB: ${error instanceof Error ? error.message : "Unknown error"}`,
      error
    );
    return ReturnErrorMessage<IgnoreItem_Response>(error);
  }
};

export const IgnoreItemInDB = async (
  tmdbID: string,
  libraryTitle: string,
  ignoreMode: string,
  currentSetsAvailable: string[],
  edition: string = ""
): Promise<APIResponse<IgnoreItem_Response>> => {
  log(
    "INFO",
    "API - DB",
    "Ignore Item",
    `Ignoring item TMDB_ID: ${tmdbID} in library: ${libraryTitle} with mode: ${ignoreMode} and current sets available: ${currentSetsAvailable.join(", ")}`
  );
  try {
    const params = {
      tmdb_id: tmdbID,
      library_title: libraryTitle,
      edition,
      mode: ignoreMode,
      current_sets: currentSetsAvailable.join(",") || "",
    };
    const response = await apiClient.patch<APIResponse<IgnoreItem_Response>>(`/db/ignore`, null, { params: params });
    if (response.data.status === "error") {
      throw new Error(response.data.error?.message || "Unknown error ignoring item in DB");
    } else {
      log(
        "INFO",
        "API - DB",
        "Ignore Item",
        `Ignored item TMDB_ID: ${tmdbID} in library: ${libraryTitle} with mode: ${ignoreMode} successfully`,
        response.data
      );
    }
    return response.data;
  } catch (error) {
    log(
      "ERROR",
      "API - DB",
      "Add",
      `Failed to add item to DB: ${error instanceof Error ? error.message : "Unknown error"}`,
      error
    );
    return ReturnErrorMessage<IgnoreItem_Response>(error);
  }
};
