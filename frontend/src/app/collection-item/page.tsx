"use client";

import CollectionsDownloadModal from "@/app/collection-item/collection-download-modal";
import { formatLastUpdatedDate } from "@/helper/format-date-last-updates";
import { makePlural } from "@/helper/make_plural";
import { ReturnErrorMessage } from "@/services/api-error-return";
import { GetAllCollectionChildrenItems } from "@/services/mediaserver/get-movie-collection-children-items";
import {
  ArrowDownAZ,
  ArrowDownZA,
  CalendarArrowDown,
  CalendarArrowUp,
  ChartBarDecreasing,
  ChartBarIncreasing,
  User,
} from "lucide-react";

import { useEffect, useMemo, useRef, useState } from "react";

import Link from "next/link";
import { useRouter } from "next/navigation";

import { AssetImage } from "@/components/shared/asset-image";
import { CollectionItemDetails } from "@/components/shared/collection-item-details";
import { DimmedBackground } from "@/components/shared/dimmed_backdrop";
import { ErrorMessage } from "@/components/shared/error-message";
import { CollectionItemFilter } from "@/components/shared/filter-collection-item-sets";
import Loader from "@/components/shared/loader";
import { PopoverHelp } from "@/components/shared/popover-help";
import { ResponsiveGrid } from "@/components/shared/responsive-grid";
import { SortControl } from "@/components/shared/select-sort";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Button } from "@/components/ui/button";
import { Lead, P } from "@/components/ui/typography";

import { cn } from "@/lib/cn";
import { log } from "@/lib/logger";
import { useCollectionStore } from "@/lib/stores/global-store-collection-store";
import { useCollectionItemPageStore } from "@/lib/stores/page-store-collection-item";
import { useCollectionsPageStore } from "@/lib/stores/page-store-collections";

import type { APIResponse } from "@/types/api/api-response";
import type { CollectionItem } from "@/types/media-and-posters/collection-item";
import type { CollectionItemSetRef } from "@/types/media-and-posters/sets";
import type { MediuxUserInfo } from "@/types/mediux/mediux-user-follow-hide";

export default function CollectionItemPage() {
  const router = useRouter();
  const isMounted = useRef(false);

  // Partial Collection Item from Store
  const partialCollectionItem = useCollectionStore((state) => state.collectionItem);

  // Main Collection Item State
  const [collectionItem, setCollectionItem] = useState<CollectionItem | null>(null);
  const [collectionItemSets, setCollectionItemSets] = useState<CollectionItemSetRef[]>([]);
  const [filteredCollectionItemSets, setFilteredCollectionItemSets] = useState<CollectionItemSetRef[]>([]);

  // User Follows/Hides States
  const [userFollows, setUserFollows] = useState<MediuxUserInfo[]>([]);
  const [userHides, setUserHides] = useState<MediuxUserInfo[]>([]);

  // Loading States
  const [responseLoading, setResponseLoading] = useState<boolean>(true);
  const [loadingMessage, setLoadingMessage] = useState("Loading...");
  const isLoading = useMemo(() => {
    return responseLoading;
  }, [responseLoading]);

  // Error States
  const [hasError, setHasError] = useState(false);
  const [error, setError] = useState<APIResponse<unknown> | null>(null);

  // Image Version State (for forcing image reloads)
  const imageVersion = useState(Date.now());

  // UI States from Store
  const { sortOrder, setSortOrder, sortOption, setSortOption, showHiddenUsers, setShowHiddenUsers } =
    useCollectionItemPageStore();

  // Collections Page Store (for adjacent items)
  const { setNextCollectionItem, setPreviousCollectionItem, getAdjacentCollectionItem } = useCollectionsPageStore();

  // Set the default sort order and option on mount
  useEffect(() => {
    if (sortOption !== "dateUpdated" && sortOption !== "username" && sortOption !== "popularity") {
      setSortOption("dateUpdated");
      setSortOrder("desc");
    }
  }, [setSortOption, setSortOrder, sortOption]);

  // 1. If no partial collection item, show error and stop further effects
  useEffect(() => {
    if (!isMounted.current) {
      isMounted.current = true;
      return;
    }
    if (!partialCollectionItem) {
      setHasError(true);
      setError(ReturnErrorMessage("No media item selected. Please go back and select a media item."));
      setResponseLoading(false);
      return;
    }
    // If we have a partialCollectionItem, reset state for new load
    setCollectionItem(null);
    setResponseLoading(true);
    setHasError(false);
    setError(null);
  }, [partialCollectionItem]);

  // 2. Fetch full collection item details when partialCollectionItem is ready
  useEffect(() => {
    if (!partialCollectionItem) return;

    setError(null);
    const fetchFullCollectionItem = async () => {
      try {
        setResponseLoading(true);
        setLoadingMessage(`Loading Collection Item: ${partialCollectionItem.title}`);
        log(
          "INFO",
          "Collection Item Page",
          "Fetch",
          `Fetching full collection item for: ${partialCollectionItem.title} (${partialCollectionItem.rating_key})`
        );

        const resp = await GetAllCollectionChildrenItems(partialCollectionItem);
        if (resp.status === "error") {
          setError(resp);
          setHasError(true);
          setResponseLoading(false);
          return;
        }

        if (!resp.data) {
          setError(ReturnErrorMessage("No collection item data returned from server."));
          setHasError(true);
          setResponseLoading(false);
          return;
        }

        const errorResponse = resp.error || null;
        const collectionItem = resp.data.collection_item || null;
        const collectionItemSets = resp.data.sets || [];
        const userFollowHide = resp.data.user_follow_hide || null;

        log("INFO", "Collection Item Page", "Fetch", "Collection Item Response", { collectionItem });
        log("INFO", "Collection Item Page", "Fetch", "Collection Sets Response", { collectionItemSets });
        log("INFO", "Collection Item Page", "Fetch", "User Follow/Hide Response", { userFollowHide });
        log("INFO", "Collection Item Page", "Fetch", `Error Response`, { errorResponse });

        setCollectionItem(collectionItem);

        // Check if collectionItemSets is an array
        if (collectionItemSets && Array.isArray(collectionItemSets) && collectionItemSets.length > 0) {
          setCollectionItemSets(collectionItemSets);
        } else {
          setCollectionItemSets([]);
          setResponseLoading(false);
          setHasError(true);
          setError({
            status: "error",
            error: {
              message: errorResponse?.message || `No collection sets found for '${partialCollectionItem.title}'`,
              help: errorResponse?.help || "",
              detail: errorResponse?.detail ?? undefined,
              function: errorResponse?.function || "Unknown",
              line_number: errorResponse?.line_number || 0,
            },
          });
        }

        if (userFollowHide && Array.isArray(userFollowHide)) {
          for (const info of userFollowHide) {
            if (info.follow) setUserFollows((prev) => [...prev, info]);
            if (info.hide) setUserHides((prev) => [...prev, info]);
          }
        } else {
          setUserFollows([]);
          setUserHides([]);
        }

        setResponseLoading(false);
      } catch (error) {
        log("ERROR", "Collection Item Page", "Fetch", "Exception while fetching collection item", error);
        setError(ReturnErrorMessage<unknown>(error));
        setHasError(true);
        setResponseLoading(false);
      } finally {
        setResponseLoading(false);
      }
    };

    fetchFullCollectionItem();
  }, [partialCollectionItem]);

  // 3. Filtering Logic
  useEffect(() => {
    if (hasError) return; // Stop if there is an error
    if (responseLoading) return; // Stop if still loading
    if (!collectionItem) return; // Stop if no collection item
    if (collectionItemSets.length === 0) return; // Stop if no sets

    log(
      "INFO",
      "Collection Item Page",
      "Filter",
      `Applying filters: sortOption=${sortOption}, sortOrder=${sortOrder}, showHiddenUsers=${showHiddenUsers}`
    );

    const filtered = collectionItemSets.filter((set) => {
      if (showHiddenUsers) return true;
      const isHidden = userHides.some((hide) => hide.username === set.user_created);
      return !isHidden;
    });

    filtered.sort((a, b) => {
      const isAFollow = userFollows.some((follow) => follow.username === a.user_created);
      const isBFollow = userFollows.some((follow) => follow.username === b.user_created);
      if (isAFollow && !isBFollow) return -1;
      if (!isAFollow && isBFollow) return 1;

      if (sortOption === "username") {
        // If users are the same, sort by date updated
        if (a.user_created === b.user_created) {
          const dateA = new Date(a.date_updated || a.date_created || "");
          const dateB = new Date(b.date_updated || b.date_created || "");
          return dateB.getTime() - dateA.getTime();
        }
        // Otherwise, sort by user name
        return sortOrder === "asc"
          ? a.user_created.localeCompare(b.user_created)
          : b.user_created.localeCompare(a.user_created);
      }

      if (sortOption === "popularity") {
        const aPopularity = a.popularity || 0;
        const bPopularity = b.popularity || 0;
        if (aPopularity === bPopularity) {
          const dateA = new Date(a.date_updated || a.date_created || "");
          const dateB = new Date(b.date_updated || b.date_created || "");
          return dateB.getTime() - dateA.getTime();
        }
        return sortOrder === "asc" ? aPopularity - bPopularity : bPopularity - aPopularity;
      }

      const dateA = new Date(a.date_updated || a.date_created || "");
      const dateB = new Date(b.date_updated || b.date_created || "");
      if (sortOption === "dateUpdated") {
        return sortOrder === "asc" ? dateA.getTime() - dateB.getTime() : dateB.getTime() - dateA.getTime();
      }

      return dateB.getTime() - dateA.getTime();
    });
    log("INFO", "Collection Item Page", "Filter", "Filtered Collection Item Sets", { filtered });
    setFilteredCollectionItemSets(filtered);
  }, [
    hasError,
    responseLoading,
    collectionItem,
    collectionItemSets,
    sortOption,
    sortOrder,
    showHiddenUsers,
    userHides,
    userFollows,
  ]);

  // 4. Compute hiddenCount based on filtering
  const hiddenCount = useMemo(() => {
    if (!collectionItemSets || collectionItemSets.length === 0) return 0;
    if (!userHides || userHides.length === 0) return 0;
    const uniqueHiddenUsers = new Set<string>();
    collectionItemSets.forEach((set) => {
      const isHidden = userHides.some((hide) => hide.username === set.user_created);
      if (isHidden) {
        uniqueHiddenUsers.add(set.user_created);
      }
    });
    return uniqueHiddenUsers.size;
  }, [collectionItemSets, userHides]);

  // 5. Compute adjacent items when collectionItem changes
  useEffect(() => {
    if (!collectionItem) return;
    if (!collectionItem?.rating_key) return;
    setNextCollectionItem(getAdjacentCollectionItem(collectionItem.rating_key, "next"));
    setPreviousCollectionItem(getAdjacentCollectionItem(collectionItem.rating_key, "previous"));
  }, [getAdjacentCollectionItem, collectionItem, setNextCollectionItem, setPreviousCollectionItem]);

  const handleShowHiddenUsers = () => {
    setShowHiddenUsers(!showHiddenUsers);
  };

  // Calculate number of active filters
  const numberOfActiveFilters = useMemo(() => {
    let count = 0;
    if (!showHiddenUsers) count++;

    return count;
  }, [showHiddenUsers]);

  if (!partialCollectionItem && !collectionItem && hasError) {
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

  if (!collectionItem && hasError) {
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
        backdropURL={`/api/images/media/collection?rating_key=${collectionItem?.rating_key}&image_type=backdrop&cb=${imageVersion}`}
      />

      <div className="p-4 lg:p-6">
        <div className="pb-6">
          {/* Header */}
          <CollectionItemDetails collectionItem={collectionItem || partialCollectionItem!} />

          {/* Loading and Error States */}
          {isLoading && (
            <div className={cn("mt-4 flex flex-col items-center", hasError ? "hidden" : "block")}>
              <Loader message={loadingMessage} />
            </div>
          )}
          {hasError && error && <ErrorMessage error={error} />}

          {/* Render filtered poster sets */}
          {collectionItemSets && collectionItemSets.length > 0 && collectionItem && (
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
                <CollectionItemFilter
                  numberOfActiveFilters={numberOfActiveFilters}
                  hiddenCount={hiddenCount}
                  showHiddenUsers={showHiddenUsers}
                  handleShowHiddenUsers={handleShowHiddenUsers}
                />

                {/* Right column: sort options */}
                <div className="flex items-center sm:justify-end sm:ml-4">
                  <SortControl
                    options={[
                      {
                        value: "dateUpdated",
                        label: "Date Updated",
                        ascIcon: <CalendarArrowUp />,
                        descIcon: <CalendarArrowDown />,
                        type: "date",
                      },
                      {
                        value: "username",
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
                    ]}
                    sortOption={sortOption}
                    sortOrder={sortOrder}
                    setSortOption={setSortOption}
                    setSortOrder={setSortOrder}
                    showLabel={false}
                  />
                </div>
              </div>

              <div className="text-center mb-4">
                {filteredCollectionItemSets && filteredCollectionItemSets.length !== collectionItemSets.length ? (
                  <div className="flex items-center justify-center gap-2 text-sm text-muted-foreground">
                    <span>
                      Showing {filteredCollectionItemSets.length} of {collectionItemSets.length}{" "}
                      {makePlural(collectionItemSets.length, "Collection Set")}
                    </span>
                    <PopoverHelp ariaLabel="help-filters">
                      <p className="mb-2">
                        Some of your {makePlural(collectionItemSets.length, "Collection Set")} are being hidden by{" "}
                        {`${numberOfActiveFilters ? `${numberOfActiveFilters} active ${makePlural(numberOfActiveFilters, "filter")}` : "no filters"}`}
                        .
                      </p>
                      <ul className="list-disc list-inside mb-2">
                        {hiddenCount > 0 && (
                          <li>
                            You have {hiddenCount} {makePlural(hiddenCount, "hidden user")}.
                          </li>
                        )}
                      </ul>
                      <p>You can adjust your filters using the checkboxes on this page.</p>
                    </PopoverHelp>
                  </div>
                ) : (
                  <p className="text-sm text-muted-foreground">
                    {collectionItemSets.length} {makePlural(collectionItemSets.length, "Collection Set")}
                  </p>
                )}
              </div>

              {filteredCollectionItemSets &&
                filteredCollectionItemSets.length === 0 &&
                collectionItemSets.length > 0 && (
                  <div className="flex flex-col items-center">
                    <ErrorMessage
                      error={ReturnErrorMessage<string>("All sets are hidden. Check your filters or hidden users.")}
                    />
                    {!showHiddenUsers && (
                      <Button className="mt-4" variant="secondary" onClick={handleShowHiddenUsers}>
                        Show Hidden Users
                      </Button>
                    )}
                  </div>
                )}

              <ResponsiveGrid size="regular">
                {filteredCollectionItemSets.map((set) => (
                  <div
                    key={set.id}
                    className="relative flex flex-col items-center p-2 border rounded-md"
                    style={{
                      background: "oklch(0.16 0.0202 282.55)",
                      opacity: "0.95",
                      padding: "0.5rem",
                    }}
                  >
                    <div className="relative w-full mb-1">
                      {/* Download Button - absolute top right */}
                      <div className="absolute top-0 right-0 z-10">
                        <CollectionsDownloadModal item={collectionItem} set={set} />
                      </div>
                      {/* Set Name */}
                      <P className="text-primary-dynamic text-sm font-semibold w-full mb-1 truncate pr-10">
                        {set.title}
                      </P>
                    </div>

                    {/* Set User Name */}
                    <div className="flex items-center justify-start w-full mb-1">
                      <div className="flex items-center gap-1">
                        <Avatar className="rounded-lg mr-1 w-4 h-4">
                          <AvatarImage
                            src={`/api/images/mediux/avatar?username=${set.user_created}`}
                            className="w-4 h-4"
                          />
                          <AvatarFallback className="">
                            <User className="w-4 h-4" />
                          </AvatarFallback>
                        </Avatar>
                        <Link
                          href={`/user/${set.user_created}`}
                          className="text-sm hover:text-primary cursor-pointer underline truncate"
                          style={{ wordBreak: "break-word" }}
                        >
                          {set.user_created}
                        </Link>
                      </div>
                    </div>

                    {/* Last Update */}
                    <Lead className="text-sm text-muted-foreground w-full mb-2">
                      Last Update:{" "}
                      {formatLastUpdatedDate(set.date_updated, set.date_created || set.images[0]?.modified || "")}
                    </Lead>

                    {/* Poster */}
                    {set.images.find((image) => image.type === "collection_poster") && (
                      <AssetImage
                        image={set.images.find((image) => image.type === "collection_poster")!}
                        imageType="mediux"
                        aspect="poster"
                        className="w-full mb-2"
                      />
                    )}

                    {/* Backdrop */}
                    {set.images.find((image) => image.type === "collection_backdrop") && (
                      <AssetImage
                        image={set.images.find((image) => image.type === "collection_backdrop")!}
                        imageType="mediux"
                        aspect="backdrop"
                        className="w-full"
                      />
                    )}
                  </div>
                ))}
              </ResponsiveGrid>
            </>
          )}
        </div>
      </div>
    </>
  );
}
