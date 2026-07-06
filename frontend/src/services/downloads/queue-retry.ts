import { ReturnErrorMessage } from "@/services/api-error-return";
import { AddItemToDownloadQueue } from "@/services/downloads/queue-add";
import { RemoveItemFromQueue } from "@/services/downloads/queue-remove";

import { log } from "@/lib/logger";

import type { APIResponse } from "@/types/api/api-response";
import type { DBSavedItem } from "@/types/database/db-poster-set";

export interface RetryItemInQueue_Response {
  result: string;
}

/**
 * Retry a failed (error) download queue entry.
 *
 * The queue processor permanently skips files prefixed with `error_`, so a retry
 * must first remove the error entry and then re-add the item as a fresh
 * in-progress entry that the processor will pick up on its next run.
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
    // 1. Clear the existing error_ entry so it stops being skipped.
    const removeResp = await RemoveItemFromQueue(safeEntry);
    if (removeResp.status === "error") {
      throw new Error(removeResp.error?.message || "Failed to clear the errored queue entry before retrying");
    }

    // 2. Re-add as a fresh in-progress entry for reprocessing.
    const addResp = await AddItemToDownloadQueue(safeEntry);
    if (addResp.status === "error") {
      throw new Error(addResp.error?.message || "Failed to re-add the item to the download queue");
    }

    return {
      status: "success",
      data: { result: addResp.data?.result || "Item re-added to download queue" },
    };
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
