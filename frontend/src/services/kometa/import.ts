import apiClient from "@/services/api-client";
import { ReturnErrorMessage } from "@/services/api-error-return";

import { log } from "@/lib/logger";

import type { APIResponse } from "@/types/api/api-response";

export interface KometaImportFolderOutcome {
  folder: string;
  outcome: string; // matched | unmatched | collection | error
  detail?: string;
  images_uploaded: number;
  images_failed: number;
  registered_in_db: boolean;
  managed_by_aura: boolean;
}

export interface KometaImportResult {
  started_at: string;
  finished_at: string;
  folders_scanned: number;
  matched: number;
  collections: number;
  unmatched_folders: number;
  images_uploaded: number;
  images_failed: number;
  items_registered: number;
  skipped_managed_by_aura: number;
  error?: string;
  folders?: KometaImportFolderOutcome[];
}

export interface KometaImport_Response {
  message: string;
  running: boolean;
  result?: KometaImportResult;
}

// TriggerKometaImport starts an asynchronous import of existing Kometa assets.
export const TriggerKometaImport = async (): Promise<APIResponse<KometaImport_Response>> => {
  log("INFO", "API - Settings", "Kometa Import", "Triggering Kometa asset import");
  try {
    const response = await apiClient.post<APIResponse<KometaImport_Response>>(`/kometa/import`);
    if (response.data.status === "error") {
      throw new Error(response.data.error?.message || "Unknown error triggering Kometa import");
    }
    return response.data;
  } catch (error) {
    return ReturnErrorMessage<KometaImport_Response>(error);
  }
};

// GetKometaImportStatus returns whether an import is running plus the last result.
export const GetKometaImportStatus = async (): Promise<APIResponse<KometaImport_Response>> => {
  try {
    const response = await apiClient.get<APIResponse<KometaImport_Response>>(`/kometa/import`);
    return response.data;
  } catch (error) {
    return ReturnErrorMessage<KometaImport_Response>(error);
  }
};
