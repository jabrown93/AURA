import apiClient from "@/services/api-client";
import { ReturnErrorMessage } from "@/services/api-error-return";

import { log } from "@/lib/logger";

import type { APIResponse } from "@/types/api/api-response";
import type { CollectionQueueItem } from "@/types/database/db-collection-queue";

export interface GetAllCollectionQueueItems_Response {
  in_progress_entries: CollectionQueueItem[];
  warning_entries: CollectionQueueItem[];
  error_entries: CollectionQueueItem[];
}

export const GetAllCollectionQueueItems = async (): Promise<APIResponse<GetAllCollectionQueueItems_Response>> => {
  try {
    log("INFO", "API - Download Queue", "Fetch", "Fetching collection download queue entries");
    const response =
      await apiClient.get<APIResponse<GetAllCollectionQueueItems_Response>>(`/download/queue/collection`);
    if (response.data.status === "error") {
      throw new Error(response.data.error?.message || "Unknown error fetching collection download queue entries");
    }
    return response.data;
  } catch (error) {
    log("ERROR", "API - Download Queue", "Fetch", "Error fetching collection download queue entries", { error });
    return ReturnErrorMessage<GetAllCollectionQueueItems_Response>(error);
  }
};
