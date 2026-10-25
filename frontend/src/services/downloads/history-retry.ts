import apiClient from "@/services/api-client";
import { ReturnErrorMessage } from "@/services/api-error-return";

import { log } from "@/lib/logger";

import type { APIResponse } from "@/types/api/api-response";

export interface RetryDownloadHistoryEntry_Request {
  id: number;
}
export interface RetryDownloadHistoryEntry_Response {
  result: string;
}

export const RetryDownloadHistoryEntry = async (
  id: number
): Promise<APIResponse<RetryDownloadHistoryEntry_Response>> => {
  log("INFO", "API - Download History", "Retry Entry", `Retrying history entry ${id}`);
  try {
    const req: RetryDownloadHistoryEntry_Request = { id };
    const response = await apiClient.post<APIResponse<RetryDownloadHistoryEntry_Response>>(
      `/download/history/retry`,
      req
    );
    if (response.data.status === "error") {
      throw new Error(response.data.error?.message || "Unknown error while retrying history entry");
    }
    return response.data;
  } catch (error) {
    log("ERROR", "API - Download History", "Retry Entry", `Failed to retry history entry ${id}`, { error });
    return ReturnErrorMessage<RetryDownloadHistoryEntry_Response>(error);
  }
};
