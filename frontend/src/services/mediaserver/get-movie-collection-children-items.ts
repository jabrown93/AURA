import apiClient from "@/services/api-client";
import { ReturnErrorMessage } from "@/services/api-error-return";

import { log } from "@/lib/logger";

import type { APIResponse } from "@/types/api/api-response";
import type { CollectionItem } from "@/types/media-and-posters/collection-item";
import type { CollectionItemSetRef } from "@/types/media-and-posters/sets";
import type { MediuxUserInfo } from "@/types/mediux/mediux-user-follow-hide";

interface GetAllCollectionChildrenItems_Response {
  collection_item: CollectionItem;
  sets: CollectionItemSetRef[];
  user_follow_hide: MediuxUserInfo[];
}

export const GetAllCollectionChildrenItems = async (
  collection_item: CollectionItem
): Promise<APIResponse<GetAllCollectionChildrenItems_Response>> => {
  log(
    "INFO",
    "API - Media Server",
    "Fetch",
    `Fetching collection children for '${collection_item.title}' [${collection_item.rating_key}] from library '${collection_item.library_title}'`
  );
  try {
    const params = {
      rating_key: collection_item.rating_key,
    };
    const response = await apiClient.get<APIResponse<GetAllCollectionChildrenItems_Response>>(
      `/mediaserver/collections/item`,
      {
        params,
        // Important: do not throw on non-2xx for this endpoint
        validateStatus: () => true,
      }
    );

    const payload = response.data;

    // If backend says error but still sends partial data, return it.
    if (payload?.status === "error") {
      if (payload.data?.collection_item != null) {
        log(
          "WARN",
          "API - Media Server",
          "Fetch",
          `Partial collection children content returned for '${collection_item.title}' [${collection_item.rating_key}]`
        );
        return payload;
      }

      throw new Error(
        payload.error?.message ||
          `Unknown error fetching collection children for ratingKey ${collection_item.rating_key}`
      );
    }

    log(
      "INFO",
      "API - Media Server",
      "Fetch",
      `Fetched collection children successfully for '${collection_item.title}' [${collection_item.rating_key}]`
    );

    return payload;
  } catch (error) {
    log(
      "ERROR",
      "API - Media Server",
      "Fetch",
      `Failed to fetch collection children for '${collection_item.title}' [${collection_item.rating_key}]: ${
        error instanceof Error ? error.message : "Unknown error"
      }`,
      error
    );
    return ReturnErrorMessage<GetAllCollectionChildrenItems_Response>(error);
  }
};
