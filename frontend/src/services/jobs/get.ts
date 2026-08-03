import apiClient from "@/services/api-client";
import { ReturnErrorMessage } from "@/services/api-error-return";

import { log } from "@/lib/logger";

import type { APIResponse } from "@/types/api/api-response";

export type JobInfo = {
  id: string;
  spec: string;
  next_run: string;
  prev_run: string;
  job_name: string;
};

export interface GetAllJobs_Response {
  jobs: JobInfo[];
}

export const GetAllJobs = async (): Promise<APIResponse<GetAllJobs_Response>> => {
  log("INFO", "API - Jobs", "Fetch", "Fetching scheduled jobs");

  try {
    const response = await apiClient.get<APIResponse<GetAllJobs_Response>>(`/jobs`);
    if (response.data.status === "error") {
      throw new Error(response.data.error?.message || "Unknown error fetching jobs");
    }
    log("INFO", "API - Jobs", "Fetch", `Fetched scheduled jobs successfully`);
    return response.data;
  } catch (error) {
    log(
      "ERROR",
      "API - Jobs",
      "Fetch",
      `Fetching jobs failed: ${error instanceof Error ? error.message : "Unknown error"}`,
      error
    );
    return ReturnErrorMessage<GetAllJobs_Response>(error);
  }
};
