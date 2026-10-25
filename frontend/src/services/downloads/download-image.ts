import apiClient from "@/services/api-client";
import { ReturnErrorMessage } from "@/services/api-error-return";

import { log } from "@/lib/logger";

import type { APIResponse } from "@/types/api/api-response";
import type { MediaItem } from "@/types/media-and-posters/media-item-and-library";
import type { ImageFile } from "@/types/media-and-posters/sets";

export interface DownloadImageFileForMediaItem_Request {
  media_item: MediaItem;
  image_file: ImageFile;
}

export interface DownloadImageFileForMediaItem_Response {
  message: string;
}

export const downloadImageFileForMediaItem = async (
  imageFile: ImageFile,
  mediaItem: MediaItem,
  fileName: string
): Promise<APIResponse<DownloadImageFileForMediaItem_Response>> => {
  try {
    const req: DownloadImageFileForMediaItem_Request = {
      image_file: imageFile,
      media_item: mediaItem,
    };
    const response = await apiClient.post<APIResponse<DownloadImageFileForMediaItem_Response>>(
      `/download/image/item`,
      req
    );
    if (response.data.status === "error") {
      throw new Error(
        response.data.error?.message || "Unknown error downloading poster file and updating media server"
      );
    }
    return response.data;
  } catch (error) {
    log(
      "ERROR",
      "API - Media Server",
      "Download and Update",
      `Failed to download file '${fileName}' and update media item '${mediaItem.title}' (TMDB ID: ${mediaItem.tmdb_id}): ${
        mediaItem.rating_key
      }: ${error instanceof Error ? error.message : "Unknown error"}`,
      error
    );
    return ReturnErrorMessage<DownloadImageFileForMediaItem_Response>(error);
  }
};
