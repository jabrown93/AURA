import apiClient from "@/services/api-client";
import { ReturnErrorMessage } from "@/services/api-error-return";

import { log } from "@/lib/logger";

import type { APIResponse } from "@/types/api/api-response";

export interface DatabaseBackup {
  name: string;
  size: number; // Bytes
  created_at: string;
}

export interface DatabaseBackups_Response {
  backups: DatabaseBackup[]; // Newest first
  folder: string; // Where the backups are saved
  keep: number; // How many backups are kept
}

export interface CreateDatabaseBackup_Response extends DatabaseBackups_Response {
  backup: DatabaseBackup; // The new backup
}

export const GetDatabaseBackups = async (): Promise<APIResponse<DatabaseBackups_Response>> => {
  try {
    const response = await apiClient.get<APIResponse<DatabaseBackups_Response>>(`/db/backups`);
    if (response.data.status === "error") {
      throw new Error(response.data.error?.message || "Unknown error getting the database backups");
    }
    return response.data;
  } catch (error) {
    log(
      "ERROR",
      "API - Database",
      "Get Backups",
      `Getting the database backups failed: ${error instanceof Error ? error.message : "Unknown error"}`,
      error
    );
    return ReturnErrorMessage<DatabaseBackups_Response>(error);
  }
};

export const CreateDatabaseBackup = async (): Promise<APIResponse<CreateDatabaseBackup_Response>> => {
  log("INFO", "API - Database", "Create Backup", "Backing up the database");
  try {
    const response = await apiClient.post<APIResponse<CreateDatabaseBackup_Response>>(`/db/backups`);
    if (response.data.status === "error") {
      throw new Error(response.data.error?.message || "Unknown error backing up the database");
    }
    log("INFO", "API - Database", "Create Backup", `Database backed up to ${response.data.data?.backup.name}`);
    return response.data;
  } catch (error) {
    log(
      "ERROR",
      "API - Database",
      "Create Backup",
      `Backing up the database failed: ${error instanceof Error ? error.message : "Unknown error"}`,
      error
    );
    return ReturnErrorMessage<CreateDatabaseBackup_Response>(error);
  }
};
