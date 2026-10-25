import apiClient from "@/services/api-client";
import { ReturnErrorMessage } from "@/services/api-error-return";

import { log } from "@/lib/logger";

import type { APIResponse } from "@/types/api/api-response";

export interface RemoveDownloadHistoryEntry_Request {
  id: number;
}
export interface RemoveDownloadHistoryEntry_Response {
  result: string;
}

export const RemoveDownloadHistoryEntry = async (
  id: number
): Promise<APIResponse<RemoveDownloadHistoryEntry_Response>> => {
  log("INFO", "API - Download History", "Delete Entry", `Deleting history entry ${id}`);
  try {
    const req: RemoveDownloadHistoryEntry_Request = { id };
    const response = await apiClient.delete<APIResponse<RemoveDownloadHistoryEntry_Response>>(
      `/download/history/item`,
      { data: req }
    );
    if (response.data.status === "error") {
      throw new Error(response.data.error?.message || "Unknown error while deleting history entry");
    }
    return response.data;
  } catch (error) {
    log("ERROR", "API - Download History", "Delete Entry", `Failed to delete history entry ${id}`, { error });
    return ReturnErrorMessage<RemoveDownloadHistoryEntry_Response>(error);
  }
};

export interface RemoveAllDownloadHistory_Response {
  result: string;
}

export const RemoveAllDownloadHistory = async (): Promise<APIResponse<RemoveAllDownloadHistory_Response>> => {
  log("INFO", "API - Download History", "Clear History", "Clearing all download history");
  try {
    const response = await apiClient.delete<APIResponse<RemoveAllDownloadHistory_Response>>(`/download/history`);
    if (response.data.status === "error") {
      throw new Error(response.data.error?.message || "Unknown error while clearing download history");
    }
    return response.data;
  } catch (error) {
    log("ERROR", "API - Download History", "Clear History", "Failed to clear download history", { error });
    return ReturnErrorMessage<RemoveAllDownloadHistory_Response>(error);
  }
};
