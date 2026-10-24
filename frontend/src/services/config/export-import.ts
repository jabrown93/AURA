import apiClient from "@/services/api-client";
import { ReturnErrorMessage } from "@/services/api-error-return";

import { log } from "@/lib/logger";

import type { APIResponse } from "@/types/api/api-response";
import type { AppStatusResponse } from "@/types/config/response-status";

// Download the config as a YAML file. The full config includes secrets, a redacted one can be shared
export const ExportAppConfig = async (
  redacted = false
): Promise<{ ok: true; blob: Blob; filename: string } | { ok: false; error: string }> => {
  try {
    const response = await apiClient.get<Blob>(`/config/export`, {
      responseType: "blob",
      params: redacted ? { redacted: true } : undefined,
    });
    const disposition = String(response.headers["content-disposition"] ?? "");
    const filename = /filename="([^"]+)"/.exec(disposition)?.[1] ?? "aura-config.yaml";
    return { ok: true, blob: response.data, filename };
  } catch (error) {
    const message = error instanceof Error ? error.message : "Unknown error";
    log("ERROR", "API - Config", "Export Config", `Failed to export config: ${message}`);
    return { ok: false, error: message };
  }
};

export interface ImportAppConfig_Response {
  message: string;
  status: AppStatusResponse;
}

// Replace the config with a config file. The config is checked before it is saved.
export const ImportAppConfig = async (yamlText: string): Promise<APIResponse<ImportAppConfig_Response>> => {
  try {
    const response = await apiClient.post<APIResponse<ImportAppConfig_Response>>(`/config/import`, { yaml: yamlText });
    if (response.data.status === "error") {
      throw new Error(response.data.error?.message || "Unknown error importing config");
    }
    return response.data;
  } catch (error) {
    // The file contents are not logged, as they include secrets
    log(
      "ERROR",
      "API - Config",
      "Import Config",
      `Failed to import config: ${error instanceof Error ? error.message : "Unknown error"}`
    );
    return ReturnErrorMessage<ImportAppConfig_Response>(error);
  }
};
