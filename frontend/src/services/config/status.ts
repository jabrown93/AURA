import apiClient from "@/services/api-client";
import { ReturnErrorMessage } from "@/services/api-error-return";

import { log } from "@/lib/logger";

import type { APIResponse } from "@/types/api/api-response";
import type { AppStatusResponse } from "@/types/config/response-status";

export interface AppConfigStatus_Response {
  status: AppStatusResponse;
}

export const GetAppConfigStatus = async (): Promise<APIResponse<AppConfigStatus_Response>> => {
  try {
    const response = await apiClient.get<APIResponse<AppConfigStatus_Response>>(`/config`);
    if (response.data.status === "error") {
      throw new Error(response.data.error?.message || "Unknown error fetching onboarding status");
    }
    return response.data;
  } catch (error) {
    log(
      "ERROR",
      "API - Config",
      "Get App Config Status",
      `Failed to fetch onboarding status: ${error instanceof Error ? error.message : "Unknown error"}`,
      error
    );
    return ReturnErrorMessage<AppConfigStatus_Response>(error);
  }
};
