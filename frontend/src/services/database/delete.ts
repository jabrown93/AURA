import { clearOpenMediaItemSavedSets } from "@/helper/update-open-media-item";
import apiClient from "@/services/api-client";
import { ReturnErrorMessage } from "@/services/api-error-return";

import { log } from "@/lib/logger";

import type { APIResponse } from "@/types/api/api-response";
import type { DBSavedItem } from "@/types/database/db-poster-set";

export interface DeleteItemFromDB_Response {
  message: string;
}

export const DeleteItemFromDB = async (deleteItem: DBSavedItem): Promise<APIResponse<DeleteItemFromDB_Response>> => {
  log(
    "INFO",
    "API - DB",
    "Delete",
    `Deleting ${deleteItem.media_item.title} (${deleteItem.media_item.tmdb_id} | ${deleteItem.media_item.library_title}) in DB`,
    deleteItem
  );
  try {
    const params = {
      tmdb_id: deleteItem.media_item.tmdb_id,
      library_title: deleteItem.media_item.library_title,
      edition: deleteItem.media_item.edition,
    };
    const response = await apiClient.delete<APIResponse<DeleteItemFromDB_Response>>(`/db`, { params });
    if (response.data.status === "error") {
      throw new Error(response.data.error?.message || "Unknown error deleting item in DB");
    } else {
      log(
        "INFO",
        "API - DB",
        "Delete",
        `Deleted ${deleteItem.media_item.title} (${deleteItem.media_item.tmdb_id} | ${deleteItem.media_item.library_title}) in DB`,
        response.data
      );
    }
    clearOpenMediaItemSavedSets({
      tmdbID: deleteItem.media_item.tmdb_id,
      libraryTitle: deleteItem.media_item.library_title,
      edition: deleteItem.media_item.edition,
    });
    return response.data;
  } catch (error) {
    log(
      "ERROR",
      "API - DB",
      "Delete",
      `Failed to delete ${deleteItem.media_item.title} (${deleteItem.media_item.tmdb_id} | ${deleteItem.media_item.library_title}) in DB: ${
        error instanceof Error ? error.message : "Unknown error"
      }`,
      error
    );
    return ReturnErrorMessage<DeleteItemFromDB_Response>(error);
  }
};
