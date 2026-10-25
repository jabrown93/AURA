import apiClient from "@/services/api-client";
import { ReturnErrorMessage } from "@/services/api-error-return";

import { log } from "@/lib/logger";

import type { APIResponse } from "@/types/api/api-response";

export interface FavoriteCreatorsResult {
  checked: number; // Items without a saved set
  queued: number; // Items a set was added to the Download Queue for
  no_set: number; // Items none of the favorite creators have a set for
}

// For every item without a saved set, adds the best set from the favorite creators to the Download Queue
export const DownloadFromFavoriteCreators = async (): Promise<APIResponse<FavoriteCreatorsResult>> => {
  try {
    const response = await apiClient.post<APIResponse<FavoriteCreatorsResult>>(`/download/favorite-creators`);
    if (response.data.status === "error") {
      throw new Error(response.data.error?.message || "Unknown error downloading from the favorite creators");
    }
    return response.data;
  } catch (error) {
    log(
      "ERROR",
      "API - Download",
      "Favorite Creators",
      `Downloading from the favorite creators failed: ${error instanceof Error ? error.message : "Unknown error"}`,
      error
    );
    return ReturnErrorMessage<FavoriteCreatorsResult>(error);
  }
};

export interface UpgradeSetsResult {
  checked: number; // Saved items that can be upgraded
  upgraded: number; // Items a better set was added to the Download Queue for
}

export interface UpgradeSetsItem {
  tmdb_id: string;
  library_title: string;
  edition: string;
}

// Replaces saved sets with a set from a higher ranked favorite creator, for the items given or every saved item
export const UpgradeSavedSets = async (items?: UpgradeSetsItem[]): Promise<APIResponse<UpgradeSetsResult>> => {
  try {
    const response = await apiClient.post<APIResponse<UpgradeSetsResult>>(`/download/upgrade-sets`, {
      items: items ?? [],
    });
    if (response.data.status === "error") {
      throw new Error(response.data.error?.message || "Unknown error upgrading the saved sets");
    }
    return response.data;
  } catch (error) {
    log(
      "ERROR",
      "API - Download",
      "Upgrade Sets",
      `Upgrading the saved sets failed: ${error instanceof Error ? error.message : "Unknown error"}`,
      error
    );
    return ReturnErrorMessage<UpgradeSetsResult>(error);
  }
};

// upgradeSetsMessage describes what an upgrade run did
export const upgradeSetsMessage = ({ checked, upgraded }: UpgradeSetsResult) =>
  `Checked ${checked} saved ${checked === 1 ? "item" : "items"}. ${upgraded} ${upgraded === 1 ? "was" : "were"} upgraded and added to the Download Queue.`;
