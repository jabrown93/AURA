import apiClient from "@/services/api-client";

import { log } from "@/lib/logger";

// Download a zip with the versions, the redacted config and the recent logs, to share when reporting a problem
export const DownloadDebugBundle = async (): Promise<{ ok: true } | { ok: false; error: string }> => {
  try {
    const response = await apiClient.get<Blob>(`/logs/debug-bundle`, { responseType: "blob" });
    const disposition = String(response.headers["content-disposition"] ?? "");
    const filename = /filename="([^"]+)"/.exec(disposition)?.[1] ?? "aura-debug.zip";

    const url = URL.createObjectURL(response.data);
    const link = document.createElement("a");
    link.href = url;
    link.download = filename;
    link.click();
    URL.revokeObjectURL(url);
    return { ok: true };
  } catch (error) {
    const message = error instanceof Error ? error.message : "Unknown error";
    log("ERROR", "API - Logs", "Debug Bundle", `Failed to download the debug bundle: ${message}`);
    return { ok: false, error: message };
  }
};
