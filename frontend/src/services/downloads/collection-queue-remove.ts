import apiClient from "@/services/api-client";
import { ReturnErrorMessage } from "@/services/api-error-return";

import { log } from "@/lib/logger";

import type { APIResponse } from "@/types/api/api-response";
import type { CollectionQueueItem } from "@/types/database/db-collection-queue";

export interface RemoveCollectionFromQueue_Request {
  item: CollectionQueueItem;
}
export interface RemoveCollectionFromQueue_Response {
  result: string;
}

export const RemoveCollectionFromQueue = async (
  item: CollectionQueueItem
): Promise<APIResponse<RemoveCollectionFromQueue_Response>> => {
  log(
    "INFO",
    "API - Download Queue",
    "Delete Collection from Queue",
    `Deleting collection '${item.collection_item?.title ?? "(unknown)"}' [${
      item.collection_item?.rating_key ?? "(unknown)"
    }] from the download queue`
  );
  try {
    const req: RemoveCollectionFromQueue_Request = { item };
    const response = await apiClient.delete<APIResponse<RemoveCollectionFromQueue_Response>>(
      `/download/queue/collection`,
      { data: req }
    );
    if (response.data.status === "error") {
      throw new Error(response.data.error?.message || "Unknown error while deleting collection from download queue");
    }
    return response.data;
  } catch (error) {
    log(
      "ERROR",
      "API - Download Queue",
      "Delete Collection from Queue",
      `Failed to delete collection from the download queue: ${error instanceof Error ? error.message : "Unknown error"}`,
      error
    );
    return ReturnErrorMessage<RemoveCollectionFromQueue_Response>(error);
  }
};
