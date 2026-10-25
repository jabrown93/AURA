import apiClient from "@/services/api-client";
import { ReturnErrorMessage } from "@/services/api-error-return";

import { log } from "@/lib/logger";

import type { APIResponse } from "@/types/api/api-response";
import type { AppConfig } from "@/types/config/config";
import type { AppStatusResponse } from "@/types/config/response-status";

export type AppConfigSection = keyof AppConfig;

export interface UpdateAppConfigSection_Response {
  message: string;
  changed: boolean; // Whether any setting changed
  status: AppStatusResponse;
}

// Save some settings in one section of the config. Nested settings are merged, lists are replaced.
export const UpdateAppConfigSection = async (
  section: AppConfigSection,
  changes: Record<string, unknown>
): Promise<APIResponse<UpdateAppConfigSection_Response>> => {
  try {
    const response = await apiClient.patch<APIResponse<UpdateAppConfigSection_Response>>(`/config/${section}`, changes);
    if (response.data.status === "error") {
      throw new Error(response.data.error?.message || `Unknown error saving ${section} settings`);
    }
    return response.data;
  } catch (error) {
    log(
      "ERROR",
      "API - Config",
      "Update Config Section",
      `Failed to save ${section} settings: ${error instanceof Error ? error.message : "Unknown error"}`,
      error
    );
    return ReturnErrorMessage<UpdateAppConfigSection_Response>(error);
  }
};
