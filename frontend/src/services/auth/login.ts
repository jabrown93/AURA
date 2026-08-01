import apiClient from "@/services/api-client";
import { ReturnErrorMessage } from "@/services/api-error-return";

import { log } from "@/lib/logger";

import type { APIResponse } from "@/types/api/api-response";

export interface Login_Request {
  password: string;
}

export interface Login_Response {
  token: string;
}

export interface AuthMethods_Response {
  auth_enabled: boolean;
  password_enabled: boolean;
  oidc_enabled: boolean;
  oidc_button_label?: string;
}

/** Where the browser must navigate to start an OIDC sign-in. */
export const OIDC_LOGIN_PATH = "/api/auth/oidc/login";

export interface Session_Response {
  authenticated: boolean;
  subject?: string;
}

export interface Logout_Response {
  logged_out: boolean;
  /** Set when the provider session should be ended too; the UI navigates there. */
  end_session_url?: string;
}

/**
 * Authenticates with the shared password. On success the backend sets an HttpOnly session
 * cookie; the token in the response body only matters to non-browser clients.
 */
export const AttemptLogin = async (password: string): Promise<APIResponse<Login_Response>> => {
  try {
    const req: Login_Request = { password };
    const resp = await apiClient.post<APIResponse<Login_Response>>(`/login`, req);
    if (resp.data.status === "error" || !resp.data?.data?.token) {
      throw new Error(resp.data.error?.message || "Unknown error during login");
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

/** Reports which sign-in methods the backend offers. Safe to call unauthenticated. */
export const GetAuthMethods = async (): Promise<APIResponse<AuthMethods_Response>> => {
  try {
    const resp = await apiClient.get<APIResponse<AuthMethods_Response>>(`/auth/methods`);
    if (resp.data.status === "error") {
      throw new Error(resp.data.error?.message || "Unknown error fetching auth methods");
    }
    return resp.data;
  } catch (error) {
    log(
      "ERROR",
      "Auth",
      "Methods",
      `Failed to fetch auth methods: ${error instanceof Error ? error.message : "Unknown error"}`,
      error
    );
    return ReturnErrorMessage<AuthMethods_Response>(error);
  }
};

/**
 * Probes whether the current session cookie is valid. The cookie is HttpOnly, so this
 * endpoint is the only way for the UI to learn its own auth state.
 */
export const GetSession = async (): Promise<APIResponse<Session_Response>> => {
  try {
    const resp = await apiClient.get<APIResponse<Session_Response>>(`/auth/session`);
    if (resp.data.status === "error") {
      throw new Error(resp.data.error?.message || "Unknown error fetching session");
    }
    return resp.data;
  } catch (error) {
    log(
      "ERROR",
      "Auth",
      "Session",
      `Failed to fetch session: ${error instanceof Error ? error.message : "Unknown error"}`,
      error
    );
    return ReturnErrorMessage<Session_Response>(error);
  }
};

/** Clears the session cookie server-side and drops any legacy bearer token. */
export const Logout = async (): Promise<APIResponse<Logout_Response>> => {
  try {
    const resp = await apiClient.post<APIResponse<Logout_Response>>(`/logout`);
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
  } finally {
    if (typeof window !== "undefined") {
      localStorage.removeItem("aura-auth-token");
    }
  }
};
