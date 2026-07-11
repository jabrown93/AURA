import apiClient from "@/services/api-client";
import { ReturnErrorMessage } from "@/services/api-error-return";

import { log } from "@/lib/logger";

import type { APIResponse } from "@/types/api/api-response";
import type { CollectionQueueItem } from "@/types/database/db-collection-queue";

export interface AddCollectionToQueue_Request {
  item: CollectionQueueItem;
}
export interface AddCollectionToQueue_Response {
  result: string;
}

export const AddCollectionToQueue = async (
  item: CollectionQueueItem
): Promise<APIResponse<AddCollectionToQueue_Response>> => {
  log(
    "INFO",
    "API - Download Queue",
    "Add Collection to Queue",
    `Adding collection '${item.collection_item?.title ?? "(unknown)"}' [${
      item.collection_item?.rating_key ?? "(unknown)"
    }] to the download queue`
  );
  try {
    const req: AddCollectionToQueue_Request = { item };
    const response = await apiClient.post<APIResponse<AddCollectionToQueue_Response>>(
      `/download/queue/collection`,
      req
    );
    if (response.data.status === "error") {
      throw new Error(response.data.error?.message || "Unknown error while adding collection to download queue");
    }
    log(
      "INFO",
      "API - Download Queue",
      "Add Collection to Queue",
      `Added collection '${item.collection_item.title}' to the download queue`,
      response.data
    );
    return response.data;
  } catch (error) {
    log(
      "ERROR",
      "API - Download Queue",
      "Add Collection to Queue",
      `Failed to add collection '${item.collection_item?.title ?? "(unknown)"}' to the download queue: ${
        error instanceof Error ? error.message : "Unknown error"
      }`,
      error
    );
    return ReturnErrorMessage<AddCollectionToQueue_Response>(error);
  }
};
