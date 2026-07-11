import apiClient from "@/services/api-client";
import { ReturnErrorMessage } from "@/services/api-error-return";

import { log } from "@/lib/logger";

import type { APIResponse } from "@/types/api/api-response";
import type { CollectionItem } from "@/types/media-and-posters/collection-item";

export interface GetCollectionItems_Response {
  collections: CollectionItem[];
}

export const GetMovieCollections = async (): Promise<APIResponse<GetCollectionItems_Response>> => {
  log("INFO", "API - Media Server", "Fetch Collection Items", "Fetching all collection items");
  try {
    const response = await apiClient.get<APIResponse<GetCollectionItems_Response>>(`/mediaserver/collections`);
    if (response.data.status === "error") {
      throw new Error(response.data.error?.message || "Unknown error fetching all collection items");
    } else {
      log("INFO", "API - Media Server", "Fetch Collection Items", `Fetched all collection items successfully`);
    }
    return response.data;
  } catch (error) {
    log(
      "ERROR",
      "API - Media Server",
      "Fetch Collection Items",
      `Failed to fetch all collection items: ${error instanceof Error ? error.message : "Unknown error"}`,
      error
    );
    return ReturnErrorMessage<GetCollectionItems_Response>(error);
  }
};
