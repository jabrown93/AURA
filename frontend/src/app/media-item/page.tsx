"use client";

import { comparePopularity } from "@/helper/compare-popularity";
import { makePlural } from "@/helper/make_plural";
import { ReturnErrorMessage } from "@/services/api-error-return";
import { GetMediaItemDetails } from "@/services/mediaserver/get-media-item-details";
import { GetRelatedLibraryItems, homeLibraryQuery } from "@/services/mediaserver/library-items";
import { GetItemSets } from "@/services/mediux/get-item-sets";
import { GetUserFollowingAndHiding } from "@/services/mediux/get-user-follow-hiding";
import {
  ArrowDown01,
  ArrowDown10,
  ArrowDownAZ,
  ArrowDownZA,
  CalendarArrowDown,
  CalendarArrowUp,
  ChartBarDecreasing,
  ChartBarIncreasing,
} from "lucide-react";

import { useEffect, useMemo, useRef, useState } from "react";

import Link from "next/link";
import { useRouter } from "next/navigation";

import { DimmedBackground } from "@/components/shared/dimmed_backdrop";
import { ErrorMessage } from "@/components/shared/error-message";
import { MediaItemFilter } from "@/components/shared/filter-media-item";
import Loader from "@/components/shared/loader";
import { MediaCarousel } from "@/components/shared/media-carousel";
import { MediaItemDetails } from "@/components/shared/media-item-details";
import { PopoverHelp } from "@/components/shared/popover-help";
import { SortControl } from "@/components/shared/select-sort";
import { Button } from "@/components/ui/button";

import { cn } from "@/lib/cn";
import { log } from "@/lib/logger";
import { useMediaStore } from "@/lib/stores/global-store-media-store";
import { useSearchQueryStore } from "@/lib/stores/global-store-search-query";
import { useUserPreferencesStore } from "@/lib/stores/global-user-preferences";
import { useHomePageStore } from "@/lib/stores/page-store-home";
import { useMediaPageStore } from "@/lib/stores/page-store-media";

import type { APIResponse } from "@/types/api/api-response";
import type { MediaItem } from "@/types/media-and-posters/media-item-and-library";
import type { IncludedItem, SetRef } from "@/types/media-and-posters/sets";
import { TYPE_MEDIA_ITEM_TYPE_OPTIONS } from "@/types/ui-options";

const MediaItemPage = () => {
  const router = useRouter();
  const isMounted = useRef(false);

  // Partial Media Item States
  const partialMediaItem = useMediaStore((state) => state.mediaItem);

  // Response Loading States
  const [detailsLoading, setDetailsLoading] = useState<boolean>(true);
  const [setsLoading, setSetsLoading] = useState<boolean>(true);
  const fetchedRatingKey = useRef<string | null>(null);

  // Main Media Item States
  const [mediaItem, setMediaItem] = useState<MediaItem | null>(null);
  const [otherLibraryMediaItems, setOtherLibraryMediaItems] = useState<MediaItem[]>([]);
  const existsInOtherSections = otherLibraryMediaItems[0] ?? null;

  const [existsInDB, setExistsInDB] = useState<boolean>(
    (mediaItem?.db_saved_sets && mediaItem.db_saved_sets.length > 0) || false
  );
  const [ignoredInDB, setIgnoredInDB] = useState<boolean>(mediaItem?.ignored_in_db || false);
  const [ignoredMode, setIgnoredMode] = useState<string>(mediaItem?.ignored_mode || "");

  const [serverType, setServerType] = useState<string>("Media Server");

  // User Follows/Hides States
  const [userFollows, setUserFollows] = useState<{ id: string; username: string }[]>([]);
  const [userHides, setUserHides] = useState<{ id: string; username: string }[]>([]);

  // Poster Sets States
  const [posterSets, setPosterSets] = useState<SetRef[] | null>(null);
  const [filteredPosterSets, setFilteredPosterSets] = useState<SetRef[] | null>(null);
  const [posterSetsIncludedItems, setPosterSetsIncludedItems] = useState<{
    [tmdb_id: string]: IncludedItem;
  } | null>(null);

  // UI States from Media Page Store
  const {
    sortStates,
    setSortOption,
    setSortOrder,
    showOnlyTitlecardSets,
    setShowOnlyTitlecardSets,
    showHiddenUsers,
    setShowHiddenUsers,
    filterByLanguage,
    setFilterByLanguage,
  } = useMediaPageStore();
  const sortType = partialMediaItem?.type as TYPE_MEDIA_ITEM_TYPE_OPTIONS;
  const sortOption = sortStates[sortType]?.sortOption ?? "date";
  const sortOrder = sortStates[sortType]?.sortOrder ?? "desc";

  // Download Defaults from User Preferences Store
  const downloadDefaultsTypes = useUserPreferencesStore((state) => state.downloadDefaults);
  const showOnlyDownloadDefaults = useUserPreferencesStore((state) => state.showOnlyDownloadDefaults);

  // Loading States
  const isLoading = detailsLoading || setsLoading;
  const loadingMessage = useMemo(() => {
    const title = mediaItem?.title || partialMediaItem?.title || "";
    if (detailsLoading && setsLoading) return `Loading Details and Poster Sets for ${title}...`;
    if (detailsLoading) return `Loading Details for ${title} from ${serverType}...`;
    return `Loading Poster Sets for ${title}...`;
  }, [detailsLoading, setsLoading, mediaItem?.title, partialMediaItem?.title, serverType]);

  // Error States
  const [hasError, setHasError] = useState(false);
  const [error, setError] = useState<APIResponse<unknown> | null>(null);
  const [setsError, setSetsError] = useState<APIResponse<unknown> | null>(null);

  // Image Version State (for forcing image reloads)
  const [imageVersion, setImageVersion] = useState(Date.now());

  // The items before and after this one on the Home page, for the arrows in the navbar
  const { setNextMediaItem, setPreviousMediaItem } = useHomePageStore();

  // Update the sortOption and sortOrder based on type
  useEffect(() => {
    if (!sortType) return;
    // If the current sortOption or sortOrder is not set, initialize them
    if (!sortStates[sortType]) {
      setSortOption(sortType, "date");
      setSortOrder(sortType, "desc");
    }
  }, [sortType, sortStates, setSortOption, setSortOrder]);

  // When the Media Item changes, set ExistsInDB
  useEffect(() => {
    if (mediaItem && mediaItem.db_saved_sets && mediaItem.db_saved_sets.length > 0) {
      setExistsInDB(true);
    } else {
      setExistsInDB(false);
    }
    if (mediaItem && mediaItem.ignored_in_db) {
      setIgnoredInDB(true);
      setIgnoredMode(mediaItem.ignored_mode || "");
    } else {
      setIgnoredInDB(false);
      setIgnoredMode("");
    }
  }, [mediaItem]);

  // 1. If no partial media item, show error and stop further effects
  useEffect(() => {
    if (!isMounted.current) {
      isMounted.current = true;
      return;
    }
    if (!partialMediaItem) {
      setHasError(true);
      setError(ReturnErrorMessage("No media item selected. Please go back and select a media item."));
      setDetailsLoading(false);
      setSetsLoading(false);
      return;
    }
  }, [partialMediaItem]);

  // 2. Show the partial media item right away, then fetch the full media item and the poster sets at the same time
  useEffect(() => {
    if (!partialMediaItem) return;
    if (fetchedRatingKey.current === partialMediaItem.rating_key) return;
    fetchedRatingKey.current = partialMediaItem.rating_key;

    const currentItem = partialMediaItem;
    const isCurrentItem = () => fetchedRatingKey.current === currentItem.rating_key;

    // Reset the page for the new media item
    setMediaItem(currentItem);
    setOtherLibraryMediaItems([]);
    setPreviousMediaItem(null);
    setNextMediaItem(null);
    setPosterSets(null);
    setFilteredPosterSets(null);
    setPosterSetsIncludedItems(null);
    setUserFollows([]);
    setUserHides([]);
    setHasError(false);
    setError(null);
    setSetsError(null);
    setDetailsLoading(true);
    setSetsLoading(true);

    const fetchPosterSets = async (item: MediaItem) => {
      try {
        log("INFO", "Media Item Page", "Fetch", `Getting poster sets for ${item.title} (${item.tmdb_id})`);
        const setsResp = await GetItemSets(item.tmdb_id, item.type, item.library_title, item.edition);
        if (!isCurrentItem()) return;

        log("INFO", "Media Item Page", "Fetch", `Poster Sets Response`, { setsResp });

        // Check Poster Sets
        if (setsResp.status === "success" && Array.isArray(setsResp.data?.sets) && setsResp.data.sets.length > 0) {
          setPosterSets(setsResp.data.sets);
          setPosterSetsIncludedItems(setsResp.data.included_items);
        } else {
          setPosterSets([] as SetRef[]);
          setSetsError({
            status: "error",
            error: {
              message: `No poster sets found for '${item.title}'`,
              help: "Use the MediUX site to confirm that images exist for this item",
              detail: setsResp.error?.detail || {
                tmdb_id: item.tmdb_id,
                item_type: item.type,
              },
              function: "",
              line_number: 0,
            },
          });
        }
      } catch (err) {
        log("ERROR", "Media Item Page", "Fetch", "Exception while fetching poster sets", err);
        if (isCurrentItem()) setSetsError(ReturnErrorMessage<unknown>(err));
      } finally {
        if (isCurrentItem()) setSetsLoading(false);
      }
    };

    // The same item in other libraries, and the items before and after it on the Home page
    const fetchRelatedItems = async (item: MediaItem) => {
      const resp = await GetRelatedLibraryItems(
        item,
        homeLibraryQuery(useHomePageStore.getState(), useSearchQueryStore.getState().searchQuery)
      );
      if (!isCurrentItem() || resp.status === "error" || !resp.data) return;
      log("INFO", "Media Item Page", "Fetch", `Media Item - Exists in other libraries?`, resp.data.other_libraries);
      setOtherLibraryMediaItems(resp.data.other_libraries ?? []);
      setPreviousMediaItem(resp.data.previous as MediaItem | null);
      setNextMediaItem(resp.data.next as MediaItem | null);
    };

    // User Follows/Hides are optional, so an error here does not stop the page
    const fetchUserFollowHide = async () => {
      try {
        const userFollowHideResp = await GetUserFollowingAndHiding();
        if (!isCurrentItem()) return;

        log("INFO", "Media Item Page", "Fetch", `User Follow/Hide Response`, { userFollowHideResp });
        const users = userFollowHideResp.data?.users;
        if (userFollowHideResp.status === "success" && Array.isArray(users)) {
          setUserFollows(users.filter((info) => info.follow));
          setUserHides(users.filter((info) => info.hide));
        }
      } catch (err) {
        log("ERROR", "Media Item Page", "Fetch", "Exception while fetching user follow/hide", err);
      }
    };

    const fetchMediaItem = async () => {
      try {
        log(
          "INFO",
          "Media Item Page",
          "Fetch",
          `Getting full media item for ${currentItem.title} (${currentItem.rating_key})`
        );
        const resp = await GetMediaItemDetails(
          currentItem.title,
          currentItem.rating_key,
          currentItem.library_title,
          "item"
        );
        if (!isCurrentItem()) return;

        const mediaItemResponse = resp.data?.media_item;
        if (resp.status === "error" && !mediaItemResponse) {
          setError(resp);
          setHasError(true);
          return;
        }

        const serverTypeResponse = resp.data?.server_type || "";
        log("INFO", "Media Item Page", "Fetch", `Server Type: ${serverTypeResponse}`, { serverTypeResponse });
        log("INFO", "Media Item Page", "Fetch", `Full Media Item Response`, { mediaItemResponse });

        // Check to see if the serverTypeResponse is valid
        // Valid types are "Plex", "Emby", "Jellyfin", "Silo"
        if (!["Plex", "Emby", "Jellyfin", "Silo"].includes(serverTypeResponse)) {
          setServerType("Media Server");
        } else {
          setServerType(serverTypeResponse);
        }

        // Check to see if mediaItemResponse is valid
        if (
          !mediaItemResponse ||
          !mediaItemResponse.rating_key ||
          !mediaItemResponse.title ||
          !mediaItemResponse.library_title
        ) {
          setError(ReturnErrorMessage("Invalid media item data found in response from server."));
          log("ERROR", "Media Item Page", "Fetch", "Invalid media item data", { mediaItemResponse });
          setHasError(true);
          return;
        }
        setMediaItem(mediaItemResponse);

        // Without a TMDB ID before, the poster sets and related items are fetched now
        if (!currentItem.tmdb_id) {
          fetchPosterSets(mediaItemResponse);
          fetchRelatedItems(mediaItemResponse);
        }
      } catch (err) {
        log("ERROR", "Media Item Page", "Fetch", "Exception while fetching media item", err);
        if (isCurrentItem()) {
          setError(ReturnErrorMessage<unknown>(err));
          setHasError(true);
        }
      } finally {
        if (isCurrentItem()) setDetailsLoading(false);
      }
    };

    fetchMediaItem();
    fetchUserFollowHide();
    if (currentItem.tmdb_id) {
      fetchPosterSets(currentItem);
      fetchRelatedItems(currentItem);
    }
  }, [partialMediaItem, setNextMediaItem, setPreviousMediaItem]);

  // 3. Filtering logic for poster sets
  useEffect(() => {
    if (setsLoading) return; // Wait for the poster sets to finish loading
    if (!mediaItem) return; // If no mediaItem, do nothing
    if (!posterSets || posterSets.length === 0) return; // If no poster sets, do nothing

    log("INFO", "Media Item Page", "Filters Sets", "Applying filters to poster sets", {
      posterSets,
      showHiddenUsers,
      userHides,
      userFollows,
      sortStates,
      mediaItem,
      showOnlyTitlecardSets,
      filterByLanguage,
    });

    let filtered = posterSets.filter((set) => {
      if (showHiddenUsers) return true;
      const isHidden = userHides.some((hide) => hide.username === set.user_created);
      return !isHidden;
    });

    // If there is no titlecard sets, then showOnlyTitlecardSets should be false
    if (mediaItem && mediaItem.type === "show") {
      const hasTitlecardSets = posterSets.some(
        (set) => Array.isArray(set.images) && set.images.some((img) => img.type === "titlecard")
      );
      if (!hasTitlecardSets) {
        setShowOnlyTitlecardSets(false);
      }
    }

    if (mediaItem && mediaItem.type === "show" && showOnlyTitlecardSets) {
      filtered = filtered.filter((set) => set.images.some((img) => img.type === "titlecard"));
    }

    if (
      filterByLanguage &&
      filterByLanguage.length > 0 &&
      !(filterByLanguage.length === 1 && filterByLanguage[0] === "")
    ) {
      filtered = filtered.filter((set) =>
        set.images.some((img) => img.language && filterByLanguage.includes(img.language))
      );
    }

    // If showOnlyDownloadDefaults is true, check sets to see if they have at least one of the download default types
    if (showOnlyDownloadDefaults && downloadDefaultsTypes && downloadDefaultsTypes.length > 0) {
      filtered = filtered.filter((set) => {
        for (const imageType of downloadDefaultsTypes) {
          if (imageType === "poster" && set.images.some((img) => img.type === "poster")) return true;
          if (imageType === "backdrop" && set.images.some((img) => img.type === "backdrop")) return true;
          if (imageType === "season_poster" && set.images.some((img) => img.type === "season_poster")) return true;
          if (imageType === "season_poster" && set.images.some((img) => img.type === "season_poster")) return true;
          if (
            imageType === "special_season_poster" &&
            set.images.some((img) => img.type === "season_poster") &&
            set.images.some((img) => img.type === "season_poster" && img.season_number === 0)
          )
            return true;
          if (imageType === "titlecard" && set.images.some((img) => img.type === "titlecard")) return true;
        }
        return false;
      });
    }

    const downloadedSetIDs = new Set(mediaItem.db_saved_sets?.map((s) => s.id));
    // Sort the filtered poster sets
    filtered.sort((a, b) => {
      const aDownloaded = downloadedSetIDs.has(a.id);
      const bDownloaded = downloadedSetIDs.has(b.id);

      if (aDownloaded && !bDownloaded) return -1;
      if (!aDownloaded && bDownloaded) return 1;

      const followedUsernames = new Set(userFollows.map((f) => f.username));
      const isAFollow = followedUsernames.has(a.user_created);
      const isBFollow = followedUsernames.has(b.user_created);
      if (isAFollow && !isBFollow) return -1;
      if (!isAFollow && isBFollow) return 1;

      if (sortOption === "user") {
        // If users are the same, sort by date updated
        if (a.user_created === b.user_created) {
          const dateA = new Date(a.date_updated);
          const dateB = new Date(b.date_updated);
          return dateB.getTime() - dateA.getTime();
        }
        // Otherwise, sort by user name
        return sortOrder === "asc"
          ? a.user_created.localeCompare(b.user_created)
          : b.user_created.localeCompare(a.user_created);
      }

      const dateA = new Date(a.date_updated);
      const dateB = new Date(b.date_updated);
      if (sortOption === "date") {
        return sortOrder === "asc" ? dateA.getTime() - dateB.getTime() : dateB.getTime() - dateA.getTime();
      }

      if (sortOption === "popularity") {
        // Use the MediUX-wide rank, as movie and collection sets are ranked separately
        const byRank = comparePopularity(a.popularity_global, b.popularity_global, sortOrder);
        // If the ranks are equal, sort by date
        return byRank !== 0 ? byRank : dateB.getTime() - dateA.getTime();
      }

      if (mediaItem?.type === "show" && sortOption === "numberOfSeasons") {
        const seasonsA = a.images.some((img) => img.type === "season_poster")
          ? a.images.filter((img) => img.type === "season_poster").length
          : 0;
        const seasonsB = b.images.some((img) => img.type === "season_poster")
          ? b.images.filter((img) => img.type === "season_poster").length
          : 0;
        if (seasonsA === seasonsB) {
          // If number of seasons are equal, sort by number of titlecards
          const titlecardsA = a.images.some((img) => img.type === "titlecard")
            ? a.images.filter((img) => img.type === "titlecard").length
            : 0;
          const titlecardsB = b.images.some((img) => img.type === "titlecard")
            ? b.images.filter((img) => img.type === "titlecard").length
            : 0;
          if (titlecardsA === titlecardsB) {
            // If number of titlecards are also equal, sort by date
            return dateB.getTime() - dateA.getTime();
          }
          return sortOrder === "asc" ? titlecardsA - titlecardsB : titlecardsB - titlecardsA;
        }
        return sortOrder === "asc" ? seasonsA - seasonsB : seasonsB - seasonsA;
      }

      if (mediaItem?.type === "show" && sortOption === "numberOfTitlecards") {
        const titlecardsA = a.images.some((img) => img.type === "titlecard")
          ? a.images.filter((img) => img.type === "titlecard").length
          : 0;
        const titlecardsB = b.images.some((img) => img.type === "titlecard")
          ? b.images.filter((img) => img.type === "titlecard").length
          : 0;
        if (titlecardsA === titlecardsB) {
          // If number of titlecards are equal, sort by date
          return dateB.getTime() - dateA.getTime();
        }
        return sortOrder === "asc" ? titlecardsA - titlecardsB : titlecardsB - titlecardsA;
      }

      if (mediaItem?.type === "movie" && sortOption === "numberOfItemsInCollection") {
        const countAPosters =
          a.images?.filter((img) => img.type === "poster" && img.item_tmdb_id !== mediaItem.tmdb_id).length ?? 0;
        const countABackdrops =
          a.images?.filter((img) => img.type === "backdrop" && img.item_tmdb_id !== mediaItem.tmdb_id).length ?? 0;
        const countAMax = Math.max(countAPosters, countABackdrops);
        const countASum = countAPosters + countABackdrops;

        const countBPosters =
          b.images?.filter((img) => img.type === "poster" && img.item_tmdb_id !== mediaItem.tmdb_id).length ?? 0;
        const countBBackdrops =
          b.images?.filter((img) => img.type === "backdrop" && img.item_tmdb_id !== mediaItem.tmdb_id).length ?? 0;
        const countBMax = Math.max(countBPosters, countBBackdrops);
        const countBSum = countBPosters + countBBackdrops;

        if (countAMax === countBMax) {
          // If max counts are equal, sort by sum of counts
          if (countASum === countBSum) {
            // If sum of counts are also equal, sort by date
            return dateB.getTime() - dateA.getTime();
          }
          return sortOrder === "asc" ? countASum - countBSum : countBSum - countASum;
        }
        return sortOrder === "asc" ? countAMax - countBMax : countBMax - countAMax;
      }

      return dateB.getTime() - dateA.getTime();
    });

    log("INFO", "Media Item Page", "Filters Sets", "Filtered and sorted poster sets", filtered);
    setFilteredPosterSets(filtered);
  }, [
    posterSets,
    showHiddenUsers,
    userHides,
    userFollows,
    sortStates,
    mediaItem,
    showOnlyTitlecardSets,
    setShowOnlyTitlecardSets,
    filterByLanguage,
    downloadDefaultsTypes,
    showOnlyDownloadDefaults,
    setsLoading,
    sortOption,
    sortOrder,
  ]);

  // 4. Compute hiddenCount based on posterSets and userHides
  const hiddenCount = useMemo(() => {
    if (!posterSets) return 0;
    if (!userHides || userHides.length === 0) return 0;
    const uniqueHiddenUsers = new Set<string>();
    posterSets.forEach((set) => {
      if (userHides.some((hide) => hide.username === set.user_created)) {
        uniqueHiddenUsers.add(set.user_created);
      }
    });

    return uniqueHiddenUsers.size;
  }, [posterSets, userHides]);

  // 4b. Compute hidden by language count based on posterSets and filterByLanguage
  const hiddenByLanguageCount = useMemo(() => {
    if (!posterSets) return 0;
    if (!filterByLanguage || filterByLanguage.length === 0) return 0;
    if (filterByLanguage.some((lang) => lang === "")) return 0; // If "All Languages" is selected, nothing is hidden by language

    let count = 0;
    posterSets.forEach((set) => {
      // If NONE of the images in the set match the selected languages, the set is hidden by language
      if (!set.images.some((img) => img.language && filterByLanguage.includes(img.language))) {
        count++;
      }
    });
    return count;
  }, [posterSets, filterByLanguage]);

  const handleShowSetsWithTitleCardsOnly = () => {
    setShowOnlyTitlecardSets(!showOnlyTitlecardSets);
  };

  const handleShowHiddenUsers = () => {
    setShowHiddenUsers(!showHiddenUsers);
  };

  const handleMediaItemChange = (item: MediaItem) => {
    // Ignore completions for the same title's copy in another library (tmdb_id matches,
    // but rating_key or library_title doesn't) - only the currently viewed library's copy should update this page.
    // Some media servers (e.g. Silo) use the same rating_key for an item in every library it is in.
    if (item.rating_key !== mediaItem?.rating_key || item.library_title !== mediaItem?.library_title) return;

    setImageVersion(Date.now());
    if (item.db_saved_sets && item.db_saved_sets.length > 0) {
      setExistsInDB(true);
    } else if (item.ignored_in_db) {
      setIgnoredInDB(true);
      setIgnoredMode(item.ignored_mode || "");
    }
    setMediaItem(item);
  };

  // Calculate number of active filters
  const numberOfActiveFilters = useMemo(() => {
    let count = 0;
    if (!showHiddenUsers) count++;
    if (showOnlyTitlecardSets) count++;
    if (showOnlyDownloadDefaults) count++;
    if (
      filterByLanguage &&
      filterByLanguage.length > 0 &&
      !filterByLanguage.some((lang) => lang === "") && // "All Languages" disables the filter
      posterSets &&
      posterSets.some((set) => set.images.some((img) => img.language && !filterByLanguage.includes(img.language)))
    ) {
      count++;
    }
    return count;
  }, [
    showHiddenUsers,
    showOnlyTitlecardSets,
    showOnlyDownloadDefaults,
    filterByLanguage,
    posterSets, // <-- add this to dependencies
  ]);

  if (!partialMediaItem && !mediaItem && hasError) {
    return (
      <div className="flex flex-col items-center">
        <ErrorMessage error={error} />
        <Button
          className="mt-4"
          variant="secondary"
          onClick={() => {
            router.push("/");
          }}
        >
          Go to Home
        </Button>
      </div>
    );
  }

  if (!mediaItem && hasError) {
    return (
      <div className="flex flex-col items-center">
        <ErrorMessage error={error} />
        <Button
          className="mt-4"
          variant="secondary"
          onClick={() => {
            router.push("/");
          }}
        >
          Go to Home
        </Button>
      </div>
    );
  }

  // Wait for the partial media item to be loaded from the store
  if (!mediaItem) {
    return (
      <div className="mt-4 flex flex-col items-center">
        <Loader message="Loading..." />
      </div>
    );
  }

  return (
    <>
      <DimmedBackground
        backdropURL={`/api/images/media/item?rating_key=${mediaItem?.rating_key}&image_type=backdrop&cb=${imageVersion}`}
      />

      <div className="p-4 lg:p-6">
        <div className="pb-6">
          {/* Header */}
          <MediaItemDetails
            mediaItem={mediaItem || undefined}
            status={posterSetsIncludedItems?.[String(mediaItem?.tmdb_id)]?.mediux_info.status || ""}
            otherMediaItem={existsInOtherSections}
            serverType={serverType}
            posterImageKeys={
              [mediaItem?.rating_key, ...(mediaItem?.series?.seasons?.map((season) => season.rating_key) || [])].filter(
                Boolean
              ) as string[]
            }
            existsInDB={existsInDB}
            ignoredInDB={ignoredInDB}
            ignoredMode={ignoredMode}
            currentSetsAvailable={posterSets?.map((set) => set.id) || []}
          />

          {/* Loading and Error States */}
          {isLoading && (
            <div className={cn("mt-4 flex flex-col items-center", hasError ? "hidden" : "block")}>
              <Loader message={loadingMessage} />
            </div>
          )}
          {hasError && error && <ErrorMessage error={error} />}
          {!isLoading && !hasError && setsError && <ErrorMessage error={setsError} />}

          {/* Render filtered poster sets once both the details and poster sets have loaded */}
          {/* The sets need the full media item to know which seasons and episodes exist */}
          {!isLoading && !hasError && posterSets && posterSets.length > 0 && mediaItem && (
            <>
              <div
                className="flex flex-col w-full mb-4 gap-4 justify-center items-center sm:justify-between sm:items-center sm:flex-row"
                style={{
                  background: "oklch(0.16 0.0202 282.55)",
                  opacity: "0.95",
                  padding: "0.5rem",
                }}
              >
                {/* Left column: Filters */}
                <MediaItemFilter
                  numberOfActiveFilters={numberOfActiveFilters}
                  hiddenCount={hiddenCount}
                  showHiddenUsers={showHiddenUsers}
                  handleShowHiddenUsers={handleShowHiddenUsers}
                  hasTitleCards={
                    mediaItem?.type === "show"
                      ? posterSets.some(
                          (set) => Array.isArray(set.images) && set.images.some((img) => img.type === "titlecard")
                        )
                      : false
                  }
                  showOnlyTitlecardSets={showOnlyTitlecardSets}
                  handleShowSetsWithTitleCardsOnly={handleShowSetsWithTitleCardsOnly}
                  showOnlyDownloadDefaults={showOnlyDownloadDefaults}
                  filterByLanguage={filterByLanguage}
                  setFilterByLanguage={setFilterByLanguage}
                />

                {/* Right column: sort options */}
                <div className="flex items-center sm:justify-end sm:ml-4">
                  <SortControl
                    options={[
                      {
                        value: "date",
                        label: "Date Updated",
                        ascIcon: <CalendarArrowUp />,
                        descIcon: <CalendarArrowDown />,
                        type: "date",
                      },
                      {
                        value: "user",
                        label: "User Name",
                        ascIcon: <ArrowDownAZ />,
                        descIcon: <ArrowDownZA />,
                        type: "string",
                      },
                      {
                        value: "popularity",
                        label: "Popularity",
                        ascIcon: <ChartBarIncreasing />,
                        descIcon: <ChartBarDecreasing />,
                        type: "number" as const,
                      },
                      ...(mediaItem?.type === "movie"
                        ? [
                            {
                              value: "numberOfItemsInCollection",
                              label: "Number in Collection",
                              ascIcon: <ArrowDown01 />,
                              descIcon: <ArrowDown10 />,
                              type: "number" as const,
                            },
                          ]
                        : []),
                      ...(mediaItem?.type === "show"
                        ? [
                            {
                              value: "numberOfSeasons",
                              label: "Number of Seasons",
                              ascIcon: <ArrowDown01 />,
                              descIcon: <ArrowDown10 />,
                              type: "number" as const,
                            },
                            {
                              value: "numberOfTitlecards",
                              label: "Number of Titlecards",
                              ascIcon: <ArrowDown01 />,
                              descIcon: <ArrowDown10 />,
                              type: "number" as const,
                            },
                          ]
                        : []),
                    ]}
                    sortOption={sortOption}
                    sortOrder={sortOrder}
                    setSortOption={(option) => setSortOption(sortType, option)}
                    setSortOrder={(order) => setSortOrder(sortType, order)}
                    showLabel={false}
                  />
                </div>
              </div>

              <div className="text-center mb-4">
                {filteredPosterSets && filteredPosterSets.length !== posterSets.length ? (
                  <div className="flex items-center justify-center gap-2 text-sm text-muted-foreground">
                    <span>
                      Showing {filteredPosterSets.length} of {posterSets.length}{" "}
                      {makePlural(posterSets.length, "Poster Set")}
                    </span>
                    <PopoverHelp ariaLabel="help-filters">
                      <p className="mb-2">
                        {filteredPosterSets.length === 0 ? "All" : filteredPosterSets.length} of your{" "}
                        {makePlural(posterSets.length, "Poster Set")} are being hidden by{" "}
                        {`${numberOfActiveFilters ? `${numberOfActiveFilters} active ${makePlural(numberOfActiveFilters, "filter")}` : "no filters"}`}
                        .
                      </p>
                      <ul className="list-disc list-inside mb-2">
                        {hiddenCount > 0 && (
                          <li>
                            You have {hiddenCount} {makePlural(hiddenCount, "hidden user")}.
                          </li>
                        )}
                        {mediaItem?.type === "show" &&
                          showOnlyTitlecardSets &&
                          posterSets.some(
                            (set) => Array.isArray(set.images) && set.images.some((img) => img.type === "titlecard")
                          ) && <li>You are filtering to show only titlecard sets.</li>}
                        {showOnlyDownloadDefaults && downloadDefaultsTypes && downloadDefaultsTypes.length > 0 && (
                          <li>You are filtering to show only sets with your selected download default types.</li>
                        )}
                        {hiddenByLanguageCount > 0 && (
                          <li>
                            You have {hiddenByLanguageCount} {makePlural(hiddenByLanguageCount, "hidden set")} due to
                            language filters.
                          </li>
                        )}
                      </ul>

                      <p>
                        You can adjust your filters using the checkboxes on this page. You can also adjust your default
                        download image types in{" "}
                        <Link href="/settings#preferences-section" className="underline">
                          User Preferences
                        </Link>
                        .
                      </p>
                    </PopoverHelp>
                  </div>
                ) : (
                  <p className="text-sm text-muted-foreground">
                    {posterSets.length} {makePlural(posterSets.length, "Poster Set")}
                  </p>
                )}
              </div>

              {filteredPosterSets && filteredPosterSets.length === 0 && posterSets.length > 0 && (
                <div className="flex flex-col items-center">
                  <ErrorMessage
                    error={ReturnErrorMessage<string>(`
                      ${filteredPosterSets.length === 0 ? "All" : filteredPosterSets.length} 
                      of your ${makePlural(posterSets.length, "Poster Set")} are being hidden 
                      by ${`${numberOfActiveFilters ? `${numberOfActiveFilters} active ${makePlural(numberOfActiveFilters, "filter")}` : "no filters"}`}.
                      `)}
                  />
                  {!showHiddenUsers && (
                    <Button className="mt-4" variant="secondary" onClick={handleShowHiddenUsers}>
                      Show Hidden Users
                    </Button>
                  )}
                  {mediaItem?.type === "show" && (
                    <Button className="mt-4" variant="secondary" onClick={handleShowSetsWithTitleCardsOnly}>
                      Show Non-Titlecard Sets
                    </Button>
                  )}
                  {showOnlyDownloadDefaults ||
                  (filterByLanguage && filterByLanguage.length > 0) ||
                  (filterByLanguage && filterByLanguage.some((lang) => lang !== "")) ? (
                    <Button
                      className="mt-4"
                      variant="secondary"
                      onClick={() => {
                        setFilterByLanguage([]);
                        setShowOnlyTitlecardSets(false);
                        setShowHiddenUsers(false);
                      }}
                    >
                      Clear All Filters
                    </Button>
                  ) : null}
                </div>
              )}

              <div className="divide-y divide-primary-dynamic/20 space-y-6">
                {/* Display the first 3 filtered sets */}
                {(filteredPosterSets ?? []).map((set) => (
                  <MediaCarousel
                    key={set.id}
                    set={set}
                    includedItems={posterSetsIncludedItems || undefined}
                    mediaItem={mediaItem}
                    onMediaItemChange={handleMediaItemChange}
                    dimNotFound={true}
                    otherLibraryMediaItems={otherLibraryMediaItems}
                  />
                ))}
              </div>
            </>
          )}
        </div>
      </div>
    </>
  );
};

export default MediaItemPage;
