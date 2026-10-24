import apiClient from "@/services/api-client";
import { ReturnErrorMessage } from "@/services/api-error-return";

import { log } from "@/lib/logger";

import type { APIResponse } from "@/types/api/api-response";
import type { DownloadHistoryEntry } from "@/types/database/download-queue";

export interface GetDownloadHistory_Response {
  entries: DownloadHistoryEntry[];
  total: number;
  max_retries: number; // How many times a failed download is retried automatically
}

export const GetDownloadHistory = async (limit = -1, offset = 0): Promise<APIResponse<GetDownloadHistory_Response>> => {
  try {
    log("INFO", "API - Download History", "Fetch", "Fetching download history entries");
    const response = await apiClient.get<APIResponse<GetDownloadHistory_Response>>(`/download/history`, {
      params: { limit, offset },
    });
    if (response.data.status === "error") {
      throw new Error(response.data.error?.message || "Unknown error fetching download history");
    }
    return response.data;
  } catch (error) {
    log("ERROR", "API - Download History", "Fetch", "Error fetching download history", { error });
    return ReturnErrorMessage<GetDownloadHistory_Response>(error);
  }
};
