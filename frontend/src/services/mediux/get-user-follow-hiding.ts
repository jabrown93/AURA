import apiClient from "@/services/api-client";
import { ReturnErrorMessage } from "@/services/api-error-return";

import { log } from "@/lib/logger";

import type { APIResponse } from "@/types/api/api-response";
import type { MediuxUserInfo } from "@/types/mediux/mediux-user-follow-hide";

export interface GetUserFollowingAndHiding_Response {
  users: MediuxUserInfo[];
}

export const GetUserFollowingAndHiding = async (): Promise<APIResponse<GetUserFollowingAndHiding_Response>> => {
  log("INFO", "API - MediUX", "Fetch User Follow/Hides", "Fetching user follow/hide data");
  try {
    const response = await apiClient.get<APIResponse<GetUserFollowingAndHiding_Response>>(`/mediux/user`);
    if (response.data.status === "error") {
      throw new Error(response.data.error?.message || "Unknown error fetching user follow/hide data");
    } else {
      log(
        "INFO",
        "API - MediUX",
        "Fetch User Follow/Hides",
        "Fetched user follow/hide data successfully",
        response.data
      );
    }
    return response.data;
  } catch (error) {
    log(
      "ERROR",
      "API - MediUX",
      "Fetch User Follow/Hides",
      `Failed to fetch user follow/hide data: ${error instanceof Error ? error.message : "Unknown error"}`,
      error
    );
    return ReturnErrorMessage<GetUserFollowingAndHiding_Response>(error);
  }
};
