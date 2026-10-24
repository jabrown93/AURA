import apiClient from "@/services/api-client";
import { ReturnErrorMessage } from "@/services/api-error-return";

import { log } from "@/lib/logger";

import type { APIResponse } from "@/types/api/api-response";

export interface CreatorSetCount {
  username: string;
  sets: number; // How many of their sets are saved
}

// The creators of the saved sets, the most saved sets first
export const GetSavedSetCreators = async (): Promise<APIResponse<{ creators: CreatorSetCount[] }>> => {
  try {
    const response = await apiClient.get<APIResponse<{ creators: CreatorSetCount[] }>>(`/db/creators`);
    if (response.data.status === "error") {
      throw new Error(response.data.error?.message || "Unknown error getting the saved set creators");
    }
    return response.data;
  } catch (error) {
    log(
      "ERROR",
      "API - DB",
      "Saved Set Creators",
      `Getting the saved set creators failed: ${error instanceof Error ? error.message : "Unknown error"}`,
      error
    );
    return ReturnErrorMessage<{ creators: CreatorSetCount[] }>(error);
  }
};
