import apiClient from "@/services/api-client";
import { ReturnErrorMessage } from "@/services/api-error-return";

import { log } from "@/lib/logger";

import type { APIResponse } from "@/types/api/api-response";
import type { AppStatusResponse } from "@/types/config/response-status";

export interface SwitchMainItem {
  title: string;
  year: number;
  tmdb_id: string;
  type: string;
  library_title: string;
  edition: string;
  new_library_title: string; // Empty if no library on the new main media server matches
  new_edition: string;
  found: boolean; // Whether the item is on the new main media server
  merged: boolean; // Whether another saved item becomes the same item, e.g. two editions
}

export interface SwitchMainPlan {
  media_server_name: string;
  library_changes: Record<string, string>; // Library on the new main media server for each library on the current one
  items: SwitchMainItem[] | null;
  moved: number;
  not_found: number;
  merged: number;
  blocked?: string; // Why the switch can't happen right now
}

export interface MainMediaServer_Response {
  plan: SwitchMainPlan;
  status?: AppStatusResponse; // Set once the main media server has changed
}

// Show what happens to the saved items if another media server becomes the main one. Nothing is changed.
export const PreviewMainMediaServerSwitch = async (id: string): Promise<APIResponse<MainMediaServer_Response>> => {
  try {
    const response = await apiClient.post<APIResponse<MainMediaServer_Response>>(`/config/main-media-server/preview`, {
      id,
    });
    if (response.data.status === "error") {
      throw new Error(response.data.error?.message || "Unknown error checking the media server");
    }
    return response.data;
  } catch (error) {
    log(
      "ERROR",
      "API - Config",
      "Preview Main Media Server",
      `Failed to preview the main media server switch: ${error instanceof Error ? error.message : "Unknown error"}`
    );
    return ReturnErrorMessage<MainMediaServer_Response>(error);
  }
};

// Make another media server the main one. The saved items move to its items.
export const SwitchMainMediaServer = async (id: string): Promise<APIResponse<MainMediaServer_Response>> => {
  try {
    const response = await apiClient.post<APIResponse<MainMediaServer_Response>>(`/config/main-media-server`, { id });
    if (response.data.status === "error") {
      throw new Error(response.data.error?.message || "Unknown error changing the main media server");
    }
    return response.data;
  } catch (error) {
    log(
      "ERROR",
      "API - Config",
      "Switch Main Media Server",
      `Failed to change the main media server: ${error instanceof Error ? error.message : "Unknown error"}`
    );
    return ReturnErrorMessage<MainMediaServer_Response>(error);
  }
};
