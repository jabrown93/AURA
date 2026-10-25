import apiClient from "@/services/api-client";
import { ReturnErrorMessage } from "@/services/api-error-return";

import { log } from "@/lib/logger";

import type { APIResponse } from "@/types/api/api-response";

export interface Logout_Response {
  logged_out: boolean;
}

// Logout clears the server-side HttpOnly session cookie. This must go through the API - the
// cookie is HttpOnly, so client JS can't clear it on its own.
export const Logout = async (): Promise<APIResponse<Logout_Response>> => {
  try {
    const resp = await apiClient.post<APIResponse<Logout_Response>>(`/logout`);
    log("INFO", "Auth", "Logout", "Logout successful");
    return resp.data;
  } catch (error) {
    log(
      "ERROR",
      "Auth",
      "Logout",
      `Failed to logout: ${error instanceof Error ? error.message : "Unknown error"}`,
      error
    );
    return ReturnErrorMessage<Logout_Response>(error);
  }
};
