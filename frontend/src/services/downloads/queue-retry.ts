import apiClient from "@/services/api-client";
import { ReturnErrorMessage } from "@/services/api-error-return";

import { log } from "@/lib/logger";

import type { APIResponse } from "@/types/api/api-response";
import type { DBSavedItem } from "@/types/database/db-poster-set";

export interface RetryItemInQueue_Request {
  item: DBSavedItem;
}
export interface RetryItemInQueue_Response {
  result: string;
}

/**
 * Retry a failed (error) download queue entry.
 *
 * The backend atomically re-queues the errored entry (strips its `error_`
 * prefix) so the processor reprocesses it on its next run. This is a single,
 * atomic operation — there is no window where a remove could succeed but a
 * re-add fail, leaving the item lost.
 */
export const RetryItemInQueue = async (dbItem: DBSavedItem): Promise<APIResponse<RetryItemInQueue_Response>> => {
  const safeEntry: DBSavedItem = {
    ...dbItem,
    poster_sets: Array.isArray(dbItem.poster_sets) ? dbItem.poster_sets : [],
  };

  log(
    "INFO",
    "API - Download Queue",
    "Retry",
    `Retrying '${safeEntry.media_item?.title ?? "(unknown)"}' (TMDB ID: ${safeEntry.media_item?.tmdb_id ?? "(unknown)"}) in the download queue`
  );

  try {
    const req: RetryItemInQueue_Request = { item: safeEntry };
    const response = await apiClient.post<APIResponse<RetryItemInQueue_Response>>(`/download/queue/item/retry`, req);
    if (response.data.status === "error") {
      throw new Error(response.data.error?.message || "Unknown error while retrying download queue item");
    }
    log(
      "INFO",
      "API - Download Queue",
      "Retry",
      `Retried '${safeEntry.media_item.title}' (TMDB ID: ${safeEntry.media_item.tmdb_id}) in the download queue`,
      response.data
    );
    return response.data;
  } catch (error) {
    log(
      "ERROR",
      "API - Download Queue",
      "Retry",
      `Failed to retry '${safeEntry.media_item?.title ?? "(unknown)"}' (TMDB ID: ${safeEntry.media_item?.tmdb_id ?? "(unknown)"}) in the download queue: ${
        error instanceof Error ? error.message : "Unknown error"
      }`,
      error
    );
    return ReturnErrorMessage<RetryItemInQueue_Response>(error);
  }
};
