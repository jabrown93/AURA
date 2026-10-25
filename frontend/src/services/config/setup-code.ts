import apiClient, { SETUP_CODE_STORAGE_KEY } from "@/services/api-client";
import { ReturnErrorMessage } from "@/services/api-error-return";

import type { APIResponse } from "@/types/api/api-response";

// Check the setup code. If it is correct, it is stored for this browser session and sent with every request.
export const VerifySetupCode = async (code: string): Promise<APIResponse<{ valid: boolean }>> => {
  try {
    const response = await apiClient.post<APIResponse<{ valid: boolean }>>(`/setup/verify`, { code });
    if (response.data.status === "error" || !response.data.data?.valid) {
      throw new Error(response.data.error?.message || "Incorrect setup code");
    }
    try {
      sessionStorage.setItem(SETUP_CODE_STORAGE_KEY, code.trim().toUpperCase());
    } catch {
      // Without sessionStorage, the code is asked for again on the next page load
    }
    return response.data;
  } catch (error) {
    // The code itself is not logged
    return ReturnErrorMessage<{ valid: boolean }>(error);
  }
};

// Forget the setup code once setup is finished
export const ClearSetupCode = () => {
  try {
    sessionStorage.removeItem(SETUP_CODE_STORAGE_KEY);
  } catch {
    // Nothing stored
  }
};
