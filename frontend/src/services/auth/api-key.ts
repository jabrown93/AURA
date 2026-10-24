import apiClient from "@/services/api-client";
import { ReturnErrorMessage } from "@/services/api-error-return";

import { log } from "@/lib/logger";

import type { APIResponse } from "@/types/api/api-response";

export interface GenerateAPIKey_Response {
  // Plaintext key, returned exactly once. Not stored anywhere after this - copy it immediately.
  api_key: string;
}

export const GenerateAPIKey = async (): Promise<APIResponse<GenerateAPIKey_Response>> => {
  try {
    const resp = await apiClient.post<APIResponse<GenerateAPIKey_Response>>(`/config/auth/api-key`);
    log("INFO", "Auth", "GenerateAPIKey", "API key generated");
    return resp.data;
  } catch (error) {
    log(
      "ERROR",
      "Auth",
      "GenerateAPIKey",
      `Failed to generate API key: ${error instanceof Error ? error.message : "Unknown error"}`,
      error
    );
    return ReturnErrorMessage<GenerateAPIKey_Response>(error);
  }
};
