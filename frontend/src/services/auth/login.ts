import apiClient from "@/services/api-client";
import { ReturnErrorMessage } from "@/services/api-error-return";

import { log } from "@/lib/logger";

import type { APIResponse } from "@/types/api/api-response";

export interface Login_Request {
  password: string;
}

export interface Login_Response {
  authenticated: boolean;
}

// AttemptLogin posts the password to the backend, which - on success - sets an HttpOnly
// session cookie (aura_session) on the response. There is no token to store client-side:
// the browser handles the cookie automatically for subsequent requests (apiClient has
// withCredentials: true).
export const AttemptLogin = async (password: string): Promise<APIResponse<Login_Response>> => {
  try {
    const req: Login_Request = { password };
    const resp = await apiClient.post<APIResponse<Login_Response>>(`/login`, req);
    if (resp.data.status === "error" || !resp.data.data?.authenticated) {
      throw new Error(resp.data.error?.message || "Invalid Password");
    }
    log("INFO", "Auth", "Login", "Login successful");
    return resp.data;
  } catch (error) {
    log(
      "ERROR",
      "Auth",
      "Login",
      `Failed to login: ${error instanceof Error ? error.message : "Unknown error"}`,
      error
    );
    return ReturnErrorMessage<Login_Response>(error);
  }
};
