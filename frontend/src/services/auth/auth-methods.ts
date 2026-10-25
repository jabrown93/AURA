import apiClient from "@/services/api-client";
import { ReturnErrorMessage } from "@/services/api-error-return";

import { log } from "@/lib/logger";

import type { APIResponse } from "@/types/api/api-response";

export interface AuthMethods_Response {
  password_enabled: boolean;
  oidc_enabled: boolean;
}

// GetAuthMethods is public (no auth required) - the login page uses it to decide whether to
// show a password field, an "SSO" button, or both, before the user is authenticated.
export const GetAuthMethods = async (): Promise<APIResponse<AuthMethods_Response>> => {
  try {
    const resp = await apiClient.get<APIResponse<AuthMethods_Response>>(`/config/auth-methods`);
    return resp.data;
  } catch (error) {
    log(
      "ERROR",
      "Auth",
      "AuthMethods",
      `Failed to get auth methods: ${error instanceof Error ? error.message : "Unknown error"}`,
      error
    );
    return ReturnErrorMessage<AuthMethods_Response>(error);
  }
};
