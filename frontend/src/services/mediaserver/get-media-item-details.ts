import apiClient from "@/services/api-client";
import { ReturnErrorMessage } from "@/services/api-error-return";

import { log } from "@/lib/logger";

import type { APIResponse } from "@/types/api/api-response";
import type { MediaItem } from "@/types/media-and-posters/media-item-and-library";
import type { PosterSetsResponse } from "@/types/media-and-posters/sets";
import type { MediuxUserInfo } from "@/types/mediux/mediux-user-follow-hide";

interface GetMediaItemDetails_Response {
  server_type: string;
  media_item: MediaItem;
  poster_sets: PosterSetsResponse;
  user_follow_hide: MediuxUserInfo[];
}

export const GetMediaItemDetails = async (
  itemTitle: string,
  ratingKey: string,
  libraryTitle: string,
  returnType: "full" | "item" = "full"
): Promise<APIResponse<GetMediaItemDetails_Response>> => {
  log(
    "INFO",
    "API - Media Server",
    "Fetch",
    `Fetching ${returnType} content for '${itemTitle}' [${ratingKey}] from library '${libraryTitle}'`
  );

  try {
    const params = {
      rating_key: ratingKey,
      return_type: returnType,
      library_title: libraryTitle,
    };

    const response = await apiClient.get<APIResponse<GetMediaItemDetails_Response>>(`/mediaserver/item`, {
      params,
      // Important: do not throw on non-2xx for this endpoint
      validateStatus: () => true,
    });

    const payload = response.data;

    // If backend says error but still sends partial data, return it.
    if (payload?.status === "error") {
      if (payload.data?.media_item != null) {
        log(
          "WARN",
          "API - Media Server",
          "Fetch",
          `Partial ${returnType} content returned for '${itemTitle}' [${ratingKey}]`
        );
        return payload;
      }

      throw new Error(payload.error?.message || `Unknown error fetching item content for ratingKey ${ratingKey}`);
    }

    log(
      "INFO",
      "API - Media Server",
      "Fetch",
      `Fetched ${returnType} content for '${itemTitle}' [${ratingKey}] from library '${libraryTitle}'`
    );

    return payload;
  } catch (error) {
    log(
      "ERROR",
      "API - Media Server",
      "Fetch",
      `Failed to fetch ${returnType} content for '${itemTitle}' [${ratingKey}]: ${
        error instanceof Error ? error.message : "Unknown error"
      }`,
      error
    );
    return ReturnErrorMessage<GetMediaItemDetails_Response>(error);
  }
};
