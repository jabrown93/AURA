import apiClient from "@/services/api-client";
import { ReturnErrorMessage } from "@/services/api-error-return";

import { log } from "@/lib/logger";

import type { APIResponse } from "@/types/api/api-response";
import type { AppConfigImageNameTemplates, NotificationTemplateVariablesCatalog } from "@/types/config/config";

export interface NotificationTemplateVariablesResponse {
  variables: NotificationTemplateVariablesCatalog;
  image_name_variables: string[]; // Variables the saved image file names can use
  image_name_defaults: AppConfigImageNameTemplates; // The standard file names
}

export const GetNotificationTemplateVariables = async (): Promise<
  APIResponse<NotificationTemplateVariablesResponse>
> => {
  try {
    const response =
      await apiClient.get<APIResponse<NotificationTemplateVariablesResponse>>("/config/template-variables");
    if (response.data.status === "error") {
      throw new Error(response.data.error?.message || "Unknown error fetching notification template variables");
    }
    return response.data;
  } catch (error) {
    log(
      "ERROR",
      "API - Config",
      "Get Notification Template Variables",
      `Failed to fetch notification template variables: ${error instanceof Error ? error.message : "Unknown error"}`,
      error
    );
    return ReturnErrorMessage<NotificationTemplateVariablesResponse>(error);
  }
};
