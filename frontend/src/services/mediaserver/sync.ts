import apiClient from "@/services/api-client";
import { ReturnErrorMessage } from "@/services/api-error-return";

import { log } from "@/lib/logger";

import type { APIResponse } from "@/types/api/api-response";

// The progress of applying the saved sets to a media server
export interface MediaServerSync {
  server_id: string;
  server_name: string;
  running: boolean;
  total: number; // Saved items
  done: number; // Saved items checked
  applied: number; // Images applied
  not_found: number; // Saved items that aren't on the media server
  failed: number; // Images or items that couldn't be applied
  failures: string[]; // The first few things that couldn't be applied
  current?: string; // The item being applied
  error?: string; // Why the sync stopped before the end
  stopped: boolean; // Whether it was stopped from Settings
  started_at: string;
  finished_at?: string;
}

export interface MediaServerSync_Response {
  sync: MediaServerSync | null; // The latest sync, null if there hasn't been one
}

const errorMessage = (error: unknown) => (error instanceof Error ? error.message : "Unknown error");

export const StartMediaServerSync = async (serverId: string): Promise<APIResponse<MediaServerSync_Response>> => {
  log("INFO", "API - Media Server", "Sync", `Applying the saved sets to media server ${serverId}`);
  try {
    const response = await apiClient.post<APIResponse<MediaServerSync_Response>>(`/mediaserver/sync`, {
      server_id: serverId,
    });
    if (response.data.status === "error") {
      throw new Error(response.data.error?.message || "Unknown error applying the saved sets");
    }
    return response.data;
  } catch (error) {
    log("ERROR", "API - Media Server", "Sync", `Applying the saved sets failed: ${errorMessage(error)}`, error);
    return ReturnErrorMessage<MediaServerSync_Response>(error);
  }
};

export const GetMediaServerSync = async (): Promise<APIResponse<MediaServerSync_Response>> => {
  try {
    const response = await apiClient.get<APIResponse<MediaServerSync_Response>>(`/mediaserver/sync`);
    if (response.data.status === "error") {
      throw new Error(response.data.error?.message || "Unknown error getting the sync progress");
    }
    return response.data;
  } catch (error) {
    log("ERROR", "API - Media Server", "Sync", `Getting the sync progress failed: ${errorMessage(error)}`, error);
    return ReturnErrorMessage<MediaServerSync_Response>(error);
  }
};

export const StopMediaServerSync = async (): Promise<APIResponse<MediaServerSync_Response>> => {
  log("INFO", "API - Media Server", "Sync", "Stopping applying the saved sets");
  try {
    const response = await apiClient.delete<APIResponse<MediaServerSync_Response>>(`/mediaserver/sync`);
    if (response.data.status === "error") {
      throw new Error(response.data.error?.message || "Unknown error stopping the sync");
    }
    return response.data;
  } catch (error) {
    log("ERROR", "API - Media Server", "Sync", `Stopping the sync failed: ${errorMessage(error)}`, error);
    return ReturnErrorMessage<MediaServerSync_Response>(error);
  }
};
