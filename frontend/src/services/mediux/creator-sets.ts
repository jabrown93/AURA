import apiClient from "@/services/api-client";
import { ReturnErrorMessage } from "@/services/api-error-return";

import { log } from "@/lib/logger";

import type { APIResponse } from "@/types/api/api-response";
import type { BoxsetRef, IncludedItem, SetRef } from "@/types/media-and-posters/sets";

export type CreatorTab = "showSets" | "movieSets" | "collectionSets" | "boxSets";

export interface CreatorLibrary {
  title: string;
  type: string;
  sets: number; // Sets with an item in this library
}

export interface CreatorTabCounts {
  show_sets: number;
  movie_sets: number;
  collection_sets: number;
  box_sets: number;
}

export interface CreatorBoxset extends BoxsetRef {
  sets: SetRef[]; // Only the sets with an item in the library
}

export interface CreatorSetsQuery {
  username: string;
  library: string;
  tab: CreatorTab;
  search: string;
  inDB: string;
  sort: string;
  order: string;
  page: number;
  perPage: number;
}

export interface CreatorSets_Response {
  libraries: CreatorLibrary[];
  totals: CreatorTabCounts; // Sets of each type the creator has
  counts: CreatorTabCounts; // Sets of each type matching the library and filters
  tab: CreatorTab;
  sets: SetRef[];
  boxsets: CreatorBoxset[];
  included_items: { [tmdb_id: string]: IncludedItem };
  total: number;
  page: number;
  per_page: number;
  total_pages: number;
}

const errorMessage = (error: unknown) => (error instanceof Error ? error.message : "Unknown error");

// The URL for one page of a creator's sets
const creatorSetsUrl = (query: CreatorSetsQuery, refresh = false) => {
  const params = new URLSearchParams();
  params.set("username", query.username);
  if (query.library) params.set("library", query.library);
  params.set("tab", query.tab);
  if (query.search) params.set("search", query.search);
  if (query.inDB) params.set("in_db", query.inDB);
  params.set("sort", query.sort);
  params.set("order", query.order);
  params.set("page", String(query.page));
  params.set("per_page", String(query.perPage));
  if (refresh) params.set("refresh", "true");
  return `/mediux/sets/user?${params.toString()}`;
};

// One page of a creator's sets for a library, matched, filtered and sorted by aura
export const GetCreatorSets = async (
  query: CreatorSetsQuery,
  refresh = false
): Promise<APIResponse<CreatorSets_Response>> => {
  try {
    const response = await apiClient.get<APIResponse<CreatorSets_Response>>(creatorSetsUrl(query, refresh));
    if (response.data.status === "error") {
      throw new Error(response.data.error?.message || `Unknown error getting the sets for ${query.username}`);
    }
    return response.data;
  } catch (error) {
    log(
      "ERROR",
      "API - MediUX",
      "Get Creator Sets",
      `Getting the sets for ${query.username} failed: ${errorMessage(error)}`,
      error
    );
    return ReturnErrorMessage<CreatorSets_Response>(error);
  }
};
