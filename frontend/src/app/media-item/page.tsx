"use client";

import { makePlural } from "@/helper/make_plural";
import { ReturnErrorMessage } from "@/services/api-error-return";
import { GetCompleteLibrarySection } from "@/services/mediaserver/get-library-section-items";
import { GetMediaItemDetails } from "@/services/mediaserver/get-media-item-details";
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
import { useLibrarySectionsStore } from "@/lib/stores/global-store-library-sections";
import { useMediaStore } from "@/lib/stores/global-store-media-store";
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
  const membershipFetchRatingKey = useRef("");

  // Partial Media Item States
  const partialMediaItem = useMediaStore((state) => state.mediaItem);

  // Library Sections States (From Library Section Store)
  const librarySectionsMap = useLibrarySectionsStore((state) => state.sections);
  const setLibrarySection = useLibrarySectionsStore((state) => state.setSection);
  const librarySectionsHasHydrated = useLibrarySectionsStore((state) => state.hasHydrated);

  // Response Loading State
  const [responseLoading, setResponseLoading] = useState<boolean>(true);

  // Main Media Item States
  const [mediaItem, setMediaItem] = useState<MediaItem | null>(null);
  const [existsInOtherSections, setExistsInOtherSections] = useState<MediaItem | null>(null);

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
  const [loadingMessage, setLoadingMessage] = useState("Loading...");
  const isLoading = useMemo(() => {
    return responseLoading;
  }, [responseLoading]);

  // Error States
  const [hasError, setHasError] = useState(false);
  const [error, setError] = useState<APIResponse<unknown> | null>(null);

  // Image Version State (for forcing image reloads)
  const [imageVersion, setImageVersion] = useState(Date.now());

  // Get Adjacent Items from Home Page Store
  const { setNextMediaItem, setPreviousMediaItem, getAdjacentMediaItem } = useHomePageStore();

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
      setResponseLoading(false);
      return;
    }
    setHasError(false);
    setError(null);
  }, [partialMediaItem]);

  // 2. Fetch full media item from server when partialMediaItem and librarySectionsMap are ready
  useEffect(() => {
    if (!librarySectionsHasHydrated) return;
    if (!partialMediaItem || Object.keys(librarySectionsMap).length === 0) return;
    if (mediaItem && mediaItem.rating_key === partialMediaItem.rating_key) return;
    if (hasError) return;
    if (responseLoading === true && mediaItem !== null) return;
    if (membershipFetchRatingKey.current === partialMediaItem.rating_key) return;
    membershipFetchRatingKey.current = partialMediaItem.rating_key;

    setError(null);

    const fetchMediaItem = async () => {
      try {
        setResponseLoading(true);
        setLoadingMessage(`Loading ${partialMediaItem.library_title} membership...`);

        const loadedSectionsMap = { ...librarySectionsMap };
        const sectionsMissingMembership = Object.values(librarySectionsMap).filter(
          (section) =>
            (section.title === partialMediaItem.library_title || section.type === partialMediaItem.type) &&
            section.media_items.length === 0
        );
        for (const section of sectionsMissingMembership) {
          const membershipResponse = await GetCompleteLibrarySection(section.title);
          if (membershipResponse.status === "error" || !membershipResponse.data) {
            setError(membershipResponse);
            setHasError(true);
            return;
          }
          const loadedSection = {
            ...section,
            total_size: membershipResponse.data.total_items,
            media_items: membershipResponse.data.media_items,
          };
          loadedSectionsMap[section.title] = loadedSection;
          setLibrarySection(section.title, loadedSection);
        }

        setLoadingMessage(`Loading Details for ${partialMediaItem.title}...`);
        log(
          "INFO",
          "Media Item Page",
          "Fetch",
          `Getting full media item for ${partialMediaItem.title} (${partialMediaItem.rating_key})`
        );
        const resp = await GetMediaItemDetails(
          partialMediaItem.title,
          partialMediaItem.rating_key,
          partialMediaItem.library_title
        );

        if (resp.status === "error" && !resp.data?.media_item) {
          setError(resp);
          setHasError(true);
          setResponseLoading(false);
          return;
        }

        const mediaItemPageResponse = resp.data;

        if (!mediaItemPageResponse) {
          setError(ReturnErrorMessage("No data found in response from server."));
          setHasError(true);
          setResponseLoading(false);
          return;
        }

        const serverTypeResponse = mediaItemPageResponse.server_type;
        const mediaItemResponse = mediaItemPageResponse.media_item;
        const posterSetsResponse = mediaItemPageResponse.poster_sets;
        const userFollowHideResponse = mediaItemPageResponse.user_follow_hide;

        log("INFO", "Media Item Page", "Fetch", `Server Type: ${serverTypeResponse}`, { serverTypeResponse });
        log("INFO", "Media Item Page", "Fetch", `Full Media Item Response`, { mediaItemResponse });
        log("INFO", "Media Item Page", "Fetch", `Poster Sets Response`, { posterSetsResponse });
        log("INFO", "Media Item Page", "Fetch", `User Follow/Hide Response`, { userFollowHideResponse });

        // Check to see if the serverTypeResponse is valid
        // Valid types are "Plex", "Emby", "Jellyfin"
        if (!["Plex", "Emby", "Jellyfin"].includes(serverTypeResponse)) {
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
          setResponseLoading(false);
          return;
        } else {
          setMediaItem(mediaItemResponse);
        }

        // Find if this item exists in other sections
        const otherSections = Object.values(loadedSectionsMap).filter(
          (s) => s.type === mediaItemResponse.type && s.title !== mediaItemResponse.library_title
        );

        if (otherSections && otherSections.length > 0) {
          log(
            "INFO",
            "Media Item Page",
            "Fetch",
            `Found other sections of type ${mediaItemResponse.type}`,
            otherSections
          );
          let foundOther: MediaItem | null = null;
          if (mediaItemResponse.guids?.length) {
            const tmdbID = mediaItemResponse.guids.find((guid) => guid.provider === "tmdb")?.id;
            if (tmdbID) {
              for (const section of otherSections) {
                if (!section.media_items || section.media_items.length === 0) continue;
                const otherMediaItem = section.media_items.find((item) =>
                  item.guids?.some((guid) => guid.provider === "tmdb" && guid.id === tmdbID)
                );
                if (otherMediaItem) {
                  foundOther = otherMediaItem;
                  break;
                }
              }
            }
          }
          log("INFO", "Media Item Page", "Fetch", `Media Item - Exists in other sections?`, foundOther);
          setExistsInOtherSections(foundOther);
        }

        // Check Poster Sets
        if (posterSetsResponse && Array.isArray(posterSetsResponse.sets) && posterSetsResponse.sets.length > 0) {
          setPosterSets(posterSetsResponse.sets);
          setPosterSetsIncludedItems(posterSetsResponse.included_items);
        } else {
          setPosterSets([] as SetRef[]);
          setResponseLoading(false);
          setHasError(true);
          setError({
            status: "error",
            error: {
              message: `No poster sets found for '${mediaItemResponse.title}'`,
              help: "Use the MediUX site to confirm that images exist for this item",
              detail: resp.error?.detail || {
                tmdb_id: mediaItemResponse.tmdb_id,
                item_type: mediaItemResponse.type,
              },
              function: "",
              line_number: 0,
            },
          });
        }

        if (userFollowHideResponse && Array.isArray(userFollowHideResponse)) {
          for (const info of userFollowHideResponse) {
            if (info.follow) setUserFollows((prev) => [...prev, info]);
            if (info.hide) setUserHides((prev) => [...prev, info]);
          }
        } else {
          setUserFollows([]);
          setUserHides([]);
        }
      } catch (err) {
        log("ERROR", "Media Item Page", "Fetch", "Exception while fetching media item", err);
        setError(ReturnErrorMessage<unknown>(err));
        setHasError(true);
        setResponseLoading(false);
      } finally {
        setResponseLoading(false);
      }
    };

    fetchMediaItem();
  }, [
    librarySectionsHasHydrated,
    hasError,
    responseLoading,
    partialMediaItem,
    librarySectionsMap,
    mediaItem,
    setLibrarySection,
  ]);

  // 3. Filtering logic for poster sets
  useEffect(() => {
    if (hasError) return; // Stop if there's already an error
    if (responseLoading) return; // Wait for response to finish loading
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
        const countA = a.popularity_global ?? 0;
        const countB = b.popularity_global ?? 0;
        if (countA === countB) {
          // If popularity counts are equal, sort by date
          return dateB.getTime() - dateA.getTime();
        }
        return sortOrder === "asc" ? countA - countB : countB - countA;
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
    hasError,
    responseLoading,
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

  // 5. Compute adjacent items when mediaItem changes
  useEffect(() => {
    if (!mediaItem) return;
    if (!mediaItem?.rating_key) return;
    setNextMediaItem(getAdjacentMediaItem(mediaItem.tmdb_id, "next"));
    setPreviousMediaItem(getAdjacentMediaItem(mediaItem.tmdb_id, "previous"));
  }, [getAdjacentMediaItem, mediaItem, setNextMediaItem, setPreviousMediaItem]);

  const handleShowSetsWithTitleCardsOnly = () => {
    setShowOnlyTitlecardSets(!showOnlyTitlecardSets);
  };

  const handleShowHiddenUsers = () => {
    setShowHiddenUsers(!showHiddenUsers);
  };

  const handleMediaItemChange = (item: MediaItem) => {
    setImageVersion(Date.now());
    if (item.tmdb_id === mediaItem?.tmdb_id) {
      if (item.db_saved_sets && mediaItem.db_saved_sets.length > 0) {
        setExistsInDB(true);
      } else if (item.ignored_in_db) {
        setIgnoredInDB(true);
        setIgnoredMode(item.ignored_mode || "");
      }
      setMediaItem(item);
    }
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

  if (responseLoading) {
    return (
      <div className={cn("mt-4 flex flex-col items-center", hasError ? "hidden" : "block")}>
        <Loader message={loadingMessage} />
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
            onArtworkApplied={handleMediaItemChange}
          />

          {/* Loading and Error States */}
          {isLoading && (
            <div className={cn("mt-4 flex flex-col items-center", hasError ? "hidden" : "block")}>
              <Loader message={loadingMessage} />
            </div>
          )}
          {hasError && error && <ErrorMessage error={error} />}

          {/* Render filtered poster sets */}
          {posterSets && posterSets.length > 0 && mediaItem && (
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
                        ascIcon: <ChartBarDecreasing />,
                        descIcon: <ChartBarIncreasing />,
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
