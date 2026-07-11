import apiClient from "@/services/api-client";
import { ReturnErrorMessage } from "@/services/api-error-return";

import { log } from "@/lib/logger";

import type { APIResponse } from "@/types/api/api-response";
import type { CollectionQueueItem } from "@/types/database/db-collection-queue";

export interface RetryCollectionInQueue_Request {
  item: CollectionQueueItem;
}
export interface RetryCollectionInQueue_Response {
  result: string;
}

/**
 * Retry a failed (error) collection download queue entry. The backend atomically
 * re-queues the errored entry (strips its `error_` prefix) so the processor
 * reprocesses it on its next run.
 */
export const RetryCollectionInQueue = async (
  item: CollectionQueueItem
): Promise<APIResponse<RetryCollectionInQueue_Response>> => {
  log(
    "INFO",
    "API - Download Queue",
    "Retry Collection",
    `Retrying collection '${item.collection_item?.title ?? "(unknown)"}' [${
      item.collection_item?.rating_key ?? "(unknown)"
    }] in the download queue`
  );
  try {
    const req: RetryCollectionInQueue_Request = { item };
    const response = await apiClient.post<APIResponse<RetryCollectionInQueue_Response>>(
      `/download/queue/collection/retry`,
      req
    );
    if (response.data.status === "error") {
      throw new Error(response.data.error?.message || "Unknown error while retrying collection in download queue");
    }
    return response.data;
  } catch (error) {
    log(
      "ERROR",
      "API - Download Queue",
      "Retry Collection",
      `Failed to retry collection in the download queue: ${error instanceof Error ? error.message : "Unknown error"}`,
      error
    );
    return ReturnErrorMessage<RetryCollectionInQueue_Response>(error);
  }
};
