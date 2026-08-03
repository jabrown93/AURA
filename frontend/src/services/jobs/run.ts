import apiClient from "@/services/api-client";
import { ReturnErrorMessage } from "@/services/api-error-return";

import { log } from "@/lib/logger";

import type { APIResponse } from "@/types/api/api-response";

export interface RunJob_Response {
  message: string;
}

export const RunJob = async (jobName: string, jobId: string): Promise<APIResponse<RunJob_Response>> => {
  log("INFO", "API - Jobs", "Trigger", `Triggering job: ${jobName} (ID: ${jobId})`);

  try {
    const params = { job_name: jobName, job_id: jobId };
    const response = await apiClient.post<APIResponse<RunJob_Response>>(`/jobs/`, null, { params });

    if (response.data.status === "error") {
      throw new Error(response.data.error?.message || "Unknown error triggering job");
    }

    log("INFO", "API - Jobs", "Trigger", `Job triggered successfully: ${jobName} (ID: ${jobId})`);
    return response.data;
  } catch (error) {
    log(
      "ERROR",
      "API - Jobs",
      "Trigger",
      `Triggering job failed: ${error instanceof Error ? error.message : "Unknown error"}`,
      error
    );
    return ReturnErrorMessage<RunJob_Response>(error);
  }
};
