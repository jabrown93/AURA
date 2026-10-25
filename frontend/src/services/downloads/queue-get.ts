import apiClient from "@/services/api-client";
import { ReturnErrorMessage } from "@/services/api-error-return";

import { log } from "@/lib/logger";

import type { APIResponse } from "@/types/api/api-response";
import type { DownloadQueueJob } from "@/types/database/download-queue";

export interface GetAllDownloadQueueItems_Response {
  jobs: DownloadQueueJob[];
}

export const GetAllDownloadQueueItems = async (): Promise<APIResponse<GetAllDownloadQueueItems_Response>> => {
  try {
    log("INFO", "API - Download Queue", "Fetch", "Fetching download queue entries");
    const response = await apiClient.get<APIResponse<GetAllDownloadQueueItems_Response>>(`/download/queue/item`);
    if (response.data.status === "error") {
      throw new Error(response.data.error?.message || "Unknown error fetching download queue entries");
    } else {
      log("INFO", "API - Download Queue", "Fetch", "Fetched download queue entries successfully", {
        jobs: response.data.data?.jobs,
      });
    }
    return response.data;
  } catch (error) {
    log("ERROR", "API - Download Queue", "Fetch", "Error fetching download queue entries", { error });
    return ReturnErrorMessage<GetAllDownloadQueueItems_Response>(error);
  }
};
