import apiClient from "@/services/api-client";
import { ReturnErrorMessage } from "@/services/api-error-return";

import { log } from "@/lib/logger";

import type { APIResponse } from "@/types/api/api-response";

export interface RemoveItemFromQueue_Request {
  job_id: number;
}
export interface RemoveItemFromQueue_Response {
  result: string;
}

export const RemoveItemFromQueue = async (jobId: number): Promise<APIResponse<RemoveItemFromQueue_Response>> => {
  log("INFO", "API - Download Queue", "Delete from Queue", `Deleting job ${jobId} from the download queue`);
  try {
    const req: RemoveItemFromQueue_Request = { job_id: jobId };
    const response = await apiClient.delete<APIResponse<RemoveItemFromQueue_Response>>(`/download/queue/item`, {
      data: req,
    });
    if (response.data.status === "error") {
      throw new Error(response.data.error?.message || "Unknown error while deleting from download queue");
    } else {
      log(
        "INFO",
        "API - Download Queue",
        "Delete from Queue",
        `Deleted job ${jobId} from the download queue`,
        response.data
      );
    }
    return response.data;
  } catch (error) {
    log("ERROR", "API - Download Queue", "Delete from Queue", `Failed to delete job ${jobId} from the download queue`, {
      error,
    });
    return ReturnErrorMessage<RemoveItemFromQueue_Response>(error);
  }
};

export interface RemoveAllFromQueue_Response {
  result: string;
}

export const RemoveAllFromQueue = async (): Promise<APIResponse<RemoveAllFromQueue_Response>> => {
  log("INFO", "API - Download Queue", "Clear Queue", "Clearing the entire download queue");
  try {
    const response = await apiClient.delete<APIResponse<RemoveAllFromQueue_Response>>(`/download/queue`);
    if (response.data.status === "error") {
      throw new Error(response.data.error?.message || "Unknown error while clearing the download queue");
    }
    return response.data;
  } catch (error) {
    log("ERROR", "API - Download Queue", "Clear Queue", "Failed to clear the download queue", { error });
    return ReturnErrorMessage<RemoveAllFromQueue_Response>(error);
  }
};
