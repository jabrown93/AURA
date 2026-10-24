import { markOpenMediaItemSetSaved } from "@/helper/update-open-media-item";
import apiClient from "@/services/api-client";
import { ReturnErrorMessage } from "@/services/api-error-return";

import { log } from "@/lib/logger";

import type { APIResponse } from "@/types/api/api-response";
import type { DBSavedItem } from "@/types/database/db-poster-set";

export interface UpdateItem_Request {
  complete: boolean;
  update_item: DBSavedItem;
}

export interface UpdateItem_Response {
  result: string;
}

export const UpdateItemInDB = async (saveItem: DBSavedItem): Promise<APIResponse<UpdateItem_Response>> => {
  log(
    "INFO",
    "API - DB",
    "Update",
    `Patching ${saveItem.media_item.title} (${saveItem.media_item.tmdb_id} | ${saveItem.media_item.library_title}) in DB`,
    saveItem
  );
  for (const set of saveItem.poster_sets) {
    set.last_downloaded = new Date().toISOString();
  }

  try {
    const req: UpdateItem_Request = {
      update_item: saveItem,
      complete: true,
    };
    const response = await apiClient.patch<APIResponse<UpdateItem_Response>>(`/db`, req);
    if (response.data.status === "error") {
      throw new Error(response.data.error?.message || "Unknown error patching item in DB");
    } else {
      log(
        "INFO",
        "API - DB",
        "Update",
        `Patched ${saveItem.media_item.title} (${saveItem.media_item.tmdb_id} | ${saveItem.media_item.library_title}) in DB`,
        response.data
      );
    }
    for (const savedSet of saveItem.poster_sets) {
      markOpenMediaItemSetSaved({
        tmdbID: saveItem.media_item.tmdb_id,
        libraryTitle: saveItem.media_item.library_title,
        edition: saveItem.media_item.edition,
        setID: savedSet.id,
        setUser: savedSet.user_created,
        selectedTypes: savedSet.selected_types,
        toDelete: savedSet.to_delete,
      });
    }

    return response.data;
  } catch (error) {
    log(
      "ERROR",
      "API - DB",
      "Update",
      `Failed to patch ${saveItem.media_item.title} (${saveItem.media_item.tmdb_id} | ${saveItem.media_item.library_title}) in DB: ${
        error instanceof Error ? error.message : "Unknown error"
      }`,
      error
    );
    return ReturnErrorMessage<UpdateItem_Response>(error);
  }
};
