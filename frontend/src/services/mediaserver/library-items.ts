import apiClient from "@/services/api-client";
import { ReturnErrorMessage } from "@/services/api-error-return";

import { log } from "@/lib/logger";

import type { APIResponse } from "@/types/api/api-response";
import type { MediaItem } from "@/types/media-and-posters/media-item-and-library";

// A media item with only what the Home page shows, sorts and filters on
export type LibraryItem = Pick<
  MediaItem,
  | "rating_key"
  | "tmdb_id"
  | "type"
  | "title"
  | "year"
  | "library_title"
  | "edition"
  | "db_saved_sets"
  | "ignored_in_db"
  | "ignored_mode"
  | "has_mediux_sets"
  | "updated_at"
  | "added_at"
  | "released_at"
  | "latest_episode_added_at"
>;

export interface LibrarySummary {
  title: string;
  type: string;
  count: number;
}

export interface LibraryItemsQuery {
  libraries: string[];
  search: string;
  inDB: string;
  ignored: string;
  setsAvailable: string;
  sort: string;
  order: string;
  page: number;
  perPage: number;
}

// The Home page's filters, search, sort and page as a query
export const homeLibraryQuery = (
  home: {
    filteredLibraries: string[];
    filterInDB: string;
    filterIgnored: string;
    hasSetsAvailableFilter: string;
    sortOption: string;
    sortOrder: string;
    currentPage: number;
    itemsPerPage: number;
  },
  search: string
): LibraryItemsQuery => ({
  libraries: home.filteredLibraries,
  search,
  inDB: home.filterInDB,
  ignored: home.filterIgnored,
  setsAvailable: home.hasSetsAvailableFilter,
  sort: home.sortOption,
  order: home.sortOrder,
  page: home.currentPage,
  perPage: home.itemsPerPage,
});

export interface LibraryItems_Response {
  items: LibraryItem[];
  total: number; // Items on every page
  page: number;
  per_page: number;
  total_pages: number;
  libraries: LibrarySummary[];
  has_updated_at: boolean;
  has_episode_added_at: boolean;
  last_full_update: number;
}

const errorMessage = (error: unknown) => (error instanceof Error ? error.message : "Unknown error");

export interface RelatedLibraryItems_Response {
  other_libraries: MediaItem[]; // The same item in other libraries
  previous: LibraryItem | null; // The item before it on the Home page
  next: LibraryItem | null; // The item after it on the Home page
}

// The Home page's filters, search and sort as URL params
const libraryParams = (query: LibraryItemsQuery) => {
  const params = new URLSearchParams();
  for (const library of query.libraries) params.append("library", library);
  if (query.search) params.set("search", query.search);
  if (query.inDB) params.set("in_db", query.inDB);
  if (query.ignored) params.set("ignored", query.ignored);
  if (query.setsAvailable) params.set("sets_available", query.setsAvailable);
  params.set("sort", query.sort);
  params.set("order", query.order);
  params.set("page", String(query.page));
  params.set("per_page", String(query.perPage));
  return params;
};

// One page of the library, filtered, searched and sorted by aura
export const GetLibraryItems = async (query: LibraryItemsQuery): Promise<APIResponse<LibraryItems_Response>> => {
  const params = libraryParams(query);
  try {
    const response = await apiClient.get<APIResponse<LibraryItems_Response>>(
      `/mediaserver/library/items?${params.toString()}`
    );
    if (response.data.status === "error") {
      throw new Error(response.data.error?.message || "Unknown error getting the library");
    }
    return response.data;
  } catch (error) {
    log("ERROR", "API - Library", "Get Items", `Getting the library failed: ${errorMessage(error)}`, error);
    return ReturnErrorMessage<LibraryItems_Response>(error);
  }
};

// The libraries and how many items each has
export const GetLibrarySummaries = async (): Promise<LibrarySummary[]> => {
  // The libraries come with every page of items, so ask for the smallest page
  const response = await GetLibraryItems({
    libraries: [],
    search: "",
    inDB: "",
    ignored: "",
    setsAvailable: "",
    sort: "title",
    order: "asc",
    page: 1,
    perPage: 1,
  });
  return response.data?.libraries ?? [];
};

// The same item in other libraries, and the items around it on the Home page
export const GetRelatedLibraryItems = async (
  item: Pick<MediaItem, "library_title" | "rating_key" | "tmdb_id">,
  home: LibraryItemsQuery
): Promise<APIResponse<RelatedLibraryItems_Response>> => {
  const params = libraryParams(home);
  params.delete("page");
  params.delete("per_page");
  params.set("item_library", item.library_title);
  params.set("rating_key", item.rating_key);
  if (item.tmdb_id) params.set("tmdb_id", item.tmdb_id);

  try {
    const response = await apiClient.get<APIResponse<RelatedLibraryItems_Response>>(
      `/mediaserver/library/item/related?${params.toString()}`
    );
    if (response.data.status === "error") {
      throw new Error(response.data.error?.message || "Unknown error getting the related items");
    }
    return response.data;
  } catch (error) {
    log(
      "ERROR",
      "API - Library",
      "Get Related Items",
      `Getting the related items failed: ${errorMessage(error)}`,
      error
    );
    return ReturnErrorMessage<RelatedLibraryItems_Response>(error);
  }
};

export interface LibraryLoad {
  title: string;
  loaded: number; // Items loaded so far
  total: number; // Items in the library, from the last load until the media server says
  done: boolean;
}

export interface LibraryProgress {
  running: boolean;
  failed: boolean;
  libraries: LibraryLoad[];
  started_at: number;
  finished_at: number;
}

// Starts reloading the libraries from the media server in the background
export const RefreshLibrary = async (): Promise<APIResponse<LibraryProgress>> => {
  try {
    const response = await apiClient.post<APIResponse<LibraryProgress>>(`/mediaserver/library/refresh`);
    if (response.data.status === "error") {
      throw new Error(response.data.error?.message || "Unknown error refreshing the library");
    }
    return response.data;
  } catch (error) {
    log("ERROR", "API - Library", "Refresh", `Refreshing the library failed: ${errorMessage(error)}`, error);
    return ReturnErrorMessage<LibraryProgress>(error);
  }
};

// Gets how far loading the libraries has got, now and every time it changes
export const subscribeToLibraryProgress = (onProgress: (progress: LibraryProgress) => void): (() => void) => {
  let es: EventSource | null = null;
  let retry: ReturnType<typeof setTimeout> | undefined;
  let stopped = false;

  const connect = () => {
    es = new EventSource(`/api/mediaserver/library/refresh/stream`, { withCredentials: true });
    es.onmessage = (event: MessageEvent<string>) => {
      try {
        onProgress(JSON.parse(event.data));
      } catch (error) {
        log("ERROR", "API - Library", "Refresh Stream", "Failed to read the library progress", { error });
      }
    };
    es.onerror = () => {
      log("ERROR", "API - Library", "Refresh Stream", "Library progress stream connection error, will reconnect");
      // Reconnect after an error, as the browser stops trying
      if (es?.readyState === EventSource.CLOSED && !stopped) {
        retry = setTimeout(connect, 3000);
      }
    };
  };
  connect();

  return () => {
    stopped = true;
    clearTimeout(retry);
    es?.close();
  };
};
