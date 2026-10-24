"use client";

import { setRefsToFormItems } from "@/helper/download-modal/set-to-form-item";
import { formatLastUpdatedDate } from "@/helper/format-date-last-updates";
import { ReturnErrorMessage } from "@/services/api-error-return";
import {
  type CreatorSetsQuery,
  type CreatorTab,
  type CreatorTabCounts,
  GetCreatorSets,
} from "@/services/mediux/creator-sets";
import { ArrowDownAZ, ArrowDownZA, ClockArrowDown, ClockArrowUp } from "lucide-react";

import { useEffect, useMemo, useState } from "react";

import Link from "next/link";
import { useParams } from "next/navigation";

import { AssetImage } from "@/components/shared/asset-image";
import { RenderBoxsetDisplay, RenderShowAndCollectionDisplay } from "@/components/shared/boxset-display";
import { CreatorAvatar } from "@/components/shared/creator-avatar";
import { CustomPagination } from "@/components/shared/custom-pagination";
import DownloadModal from "@/components/shared/download-modal";
import { ErrorMessage } from "@/components/shared/error-message";
import { LoadTimingBadge } from "@/components/shared/load-timing-badge";
import Loader from "@/components/shared/loader";
import { RefreshButton } from "@/components/shared/refresh-button";
import { RefreshProgress } from "@/components/shared/refresh-progress";
import { ResponsiveGrid } from "@/components/shared/responsive-grid";
import { SelectItemsPerPage } from "@/components/shared/select-items-per-page";
import { SortControl } from "@/components/shared/select-sort";
import { Badge } from "@/components/ui/badge";
import { Label } from "@/components/ui/label";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Lead, P } from "@/components/ui/typography";

import { cn } from "@/lib/cn";
import { useSearchQueryStore } from "@/lib/stores/global-store-search-query";
import { useUserPageStore } from "@/lib/stores/page-store-user";

import { useFirstPageOnSearch, usePagedLoad } from "@/hooks/use-paged-load";
import { useRefreshing } from "@/hooks/use-refreshing";

import type { IncludedItem, SetRef } from "@/types/media-and-posters/sets";
import { TYPE_USER_PAGE_FILTER_IN_DB_OPTIONS, USER_PAGE_FILTER_IN_DB_OPTIONS } from "@/types/ui-options";

// The order tabs are shown in, and picked in when a tab has no sets
const TABS: { value: CreatorTab; label: string; count: keyof CreatorTabCounts }[] = [
  { value: "showSets", label: "Show Sets", count: "show_sets" },
  { value: "movieSets", label: "Movie Sets", count: "movie_sets" },
  { value: "collectionSets", label: "Collection Sets", count: "collection_sets" },
  { value: "boxSets", label: "Box Sets", count: "box_sets" },
];
// Only a selected library has pages
const hasPages = (q: CreatorSetsQuery) => !!q.library;

const TAB_PICK_ORDER: CreatorTab[] = ["boxSets", "showSets", "movieSets", "collectionSets"];
const countFor = (counts: CreatorTabCounts, tab: CreatorTab) => counts[TABS.find((t) => t.value === tab)!.count];

const getNextInDbFilter = (current: TYPE_USER_PAGE_FILTER_IN_DB_OPTIONS): TYPE_USER_PAGE_FILTER_IN_DB_OPTIONS => {
  const cycle: TYPE_USER_PAGE_FILTER_IN_DB_OPTIONS[] = ["", ...USER_PAGE_FILTER_IN_DB_OPTIONS.map((opt) => opt.value)];
  return cycle[(cycle.indexOf(current) + 1) % cycle.length];
};

// A creator's sets, matched, filtered, sorted and paged by aura
export default function UserSetPage() {
  const { username: usernameParam } = useParams();
  const username = String(usernameParam);

  const { searchQuery, setSearchQuery } = useSearchQueryStore();
  const {
    currentPage,
    setCurrentPage,
    itemsPerPage,
    setItemsPerPage,
    sortOption,
    setSortOption,
    sortOrder,
    setSortOrder,
  } = useUserPageStore();
  const hasHydrated = useUserPageStore((state) => state.hasHydrated);

  const [library, setLibrary] = useState("");
  const [activeTab, setActiveTab] = useState<CreatorTab>("boxSets");
  const [filterInDB, setFilterInDB] = useState<TYPE_USER_PAGE_FILTER_IN_DB_OPTIONS>("");

  const { shown, loading, error, timing, load } = usePagedLoad(
    GetCreatorSets,
    "Could not load the sets",
    setCurrentPage,
    hasPages
  );
  // Get the creator's sets from MediUX again
  const [refreshing, handleRefresh] = useRefreshing(() => load(query, true));
  const data = shown?.data ?? null;

  // The user page only sorts by title or date updated
  useEffect(() => {
    if (!hasHydrated) return;
    if (sortOption !== "title" && sortOption !== "dateLastUpdate") {
      setSortOption("dateLastUpdate");
      setSortOrder("desc");
    }
  }, [hasHydrated, sortOption, setSortOption, setSortOrder]);

  const query = useMemo<CreatorSetsQuery>(
    () => ({
      username,
      library,
      tab: activeTab,
      search: library ? searchQuery : "",
      inDB: filterInDB,
      sort: sortOption,
      order: sortOrder,
      page: currentPage,
      perPage: itemsPerPage,
    }),
    [username, library, activeTab, searchQuery, filterInDB, sortOption, sortOrder, currentPage, itemsPerPage]
  );

  useEffect(() => {
    if (!hasHydrated) return;
    if (query.sort !== "title" && query.sort !== "dateLastUpdate") return;
    void load(query);
  }, [hasHydrated, query, load]);

  useFirstPageOnSearch(searchQuery, setCurrentPage);

  // When the tab has no sets for this library, show the first tab that does
  useEffect(() => {
    if (!shown || loading || !library) return;
    if (shown.query.library !== library || shown.query.tab !== activeTab) return;
    if (countFor(shown.data.counts, activeTab) > 0) return;
    const next = TAB_PICK_ORDER.find((tab) => countFor(shown.data.counts, tab) > 0);
    if (next) {
      setActiveTab(next);
      setCurrentPage(1);
    }
  }, [shown, loading, library, activeTab, setCurrentPage]);

  const selectLibrary = (title: string) => {
    setLibrary(library === title ? "" : title);
    setActiveTab("boxSets");
    setCurrentPage(1);
    setFilterInDB("");
    setSearchQuery("");
  };

  const includedItems: { [tmdb_id: string]: IncludedItem } = data?.included_items ?? {};
  const totals = data?.totals;
  const hasSets = !!totals && totals.show_sets + totals.movie_sets + totals.collection_sets + totals.box_sets > 0;
  const libraryCounts = library && data && shown?.query.library === library ? data.counts : totals;
  const libraryHasSets = !!data && TABS.some((tab) => countFor(data.counts, tab.value) > 0);

  const filterBadge = (
    <div className="w-full flex items-center mb-2">
      <Label htmlFor="filter-out-in-db" className="text-lg font-semibold mr-2">
        Filter:
      </Label>
      <Badge
        key="filter-out-in-db"
        className={cn(
          "cursor-pointer text-sm active:scale-95 hover:brightness-120",
          filterInDB === "inDB" && "bg-green-600 text-white",
          filterInDB === "notInDB" && "bg-red-600 text-white",
          filterInDB === "otherSetInDB" && "bg-yellow-600 text-white"
        )}
        variant={filterInDB !== "" ? "default" : "outline"}
        onClick={() => {
          setFilterInDB(getNextInDbFilter(filterInDB));
          setCurrentPage(1);
        }}
      >
        {filterInDB === ""
          ? "All Items"
          : USER_PAGE_FILTER_IN_DB_OPTIONS.find((option) => option.value === filterInDB)?.label || "Filter"}
      </Badge>
    </div>
  );

  return (
    <div className="flex flex-col">
      {/* Header */}
      <div className="flex flex-col items-center mt-8 mb-4 gap-2">
        <h1 className="text-4xl font-extrabold text-center mb-2 tracking-tight text-primary flex items-center justify-center gap-2">
          <span className="text-white opacity-80">Sets by</span>
          <span className="text-primary">{username}</span>
          <CreatorAvatar username={username} className="rounded-lg w-7 h-7 ml-2 align-middle" iconClassName="w-7 h-7" />
        </h1>
        <LoadTimingBadge
          label="User"
          startedAt={timing.startedAt}
          finishedAt={timing.finishedAt}
          urlMatch="/api/mediux/sets/user"
        />
      </div>

      {refreshing && (
        <RefreshProgress title={`Getting the sets for ${username} from MediUX`} className="mx-4 w-auto sm:mx-8" />
      )}

      {/* First load, while the sets are fetched from MediUX */}
      {!data && !error && (
        <div className="flex justify-center mt-4">
          <Loader message={`Loading sets for ${username}`} />
        </div>
      )}

      {error && !data && (
        <div className="flex justify-center">
          <ErrorMessage error={error} />
        </div>
      )}

      {data && !hasSets && (
        <div className="flex justify-center mt-4">
          <ErrorMessage error={ReturnErrorMessage<string>(`No sets found for ${username}`)} />
        </div>
      )}

      {data && hasSets && (
        <div className="min-h-screen px-4 sm:px-8 pb-20">
          {/* Set counts, for the library if one is selected */}
          {libraryCounts && (
            <div className="flex flex-wrap gap-3 justify-center">
              {TAB_PICK_ORDER.map((tab) => TABS.find((t) => t.value === tab)!)
                .filter((tab) => libraryCounts[tab.count] > 0)
                .map((tab) => (
                  <div
                    key={tab.value}
                    className="flex items-center gap-2 bg-background border border-border rounded-lg px-4 py-2 shadow-sm"
                  >
                    <span className="font-semibold text-primary">{tab.label}</span>
                    <Badge variant="secondary" className="text-xs px-2 py-1">
                      {libraryCounts[tab.count]}
                    </Badge>
                  </div>
                ))}
            </div>
          )}

          {/* Library Selection */}
          <div className="w-full max-w-3xl mt-6">
            <div className="flex flex-col sm:flex-row mb-4 mt-2">
              <Label htmlFor="library-filter" className="text-lg font-semibold mb-2 sm:mb-0 sm:mr-4">
                Libraries:
              </Label>
              <div className="flex flex-wrap sm:flex-nowrap gap-2">
                {data.libraries.map((section) => (
                  <Badge
                    key={section.title}
                    variant={library === section.title ? "default" : "outline"}
                    onClick={() => selectLibrary(section.title)}
                    className={cn(
                      "cursor-pointer text-sm active:scale-95 hover:brightness-120",
                      !!library && library !== section.title && "opacity-50 hover:opacity-100"
                    )}
                  >
                    {section.title} ({section.sets})
                  </Badge>
                ))}
              </div>
            </div>
          </div>

          {!library ? (
            <div className="flex justify-center mt-8">
              <ErrorMessage
                isWarning={true}
                error={ReturnErrorMessage<string>("No library selected. Select one to get started.")}
              />
            </div>
          ) : shown?.query.library !== library ? (
            <div className="flex justify-center mt-4">
              <Loader message={`Matching sets to ${library}`} />
            </div>
          ) : !libraryHasSets ? (
            <>
              <div className="flex justify-center">{filterBadge}</div>
              <div className="flex justify-center mt-8">
                <ErrorMessage
                  error={ReturnErrorMessage<string>(
                    `No sets found in ${library} library${
                      filterInDB === "inDB"
                        ? " that exist in your database"
                        : filterInDB === "notInDB"
                          ? " that are missing from your database"
                          : filterInDB === "otherSetInDB"
                            ? " that are in other sets in your database"
                            : ""
                    }${searchQuery ? ` for search query "${searchQuery}"` : ""}`
                  )}
                />
              </div>
            </>
          ) : (
            <div className="flex flex-col items-center mt-4 mb-4">
              {filterBadge}

              {/* Items Per Page Selection */}
              <div className="w-full flex items-center mb-2">
                <SelectItemsPerPage
                  setCurrentPage={setCurrentPage}
                  itemsPerPage={itemsPerPage}
                  setItemsPerPage={setItemsPerPage}
                />
              </div>

              {/* Sort Control */}
              <div className="w-full flex items-center mb-4">
                <SortControl
                  options={[
                    {
                      value: "dateLastUpdate",
                      label: "Date Updated",
                      ascIcon: <ClockArrowUp />,
                      descIcon: <ClockArrowDown />,
                      type: "date" as const,
                    },
                    {
                      value: "title",
                      label: "Title",
                      ascIcon: <ArrowDownAZ />,
                      descIcon: <ArrowDownZA />,
                      type: "string" as const,
                    },
                  ]}
                  sortOption={sortOption}
                  sortOrder={sortOrder}
                  setSortOption={(value) => {
                    setSortOption(value as "title" | "dateLastUpdate");
                    if (value === "title") setSortOrder("asc");
                    else if (value === "dateLastUpdate") setSortOrder("desc");
                  }}
                  setSortOrder={setSortOrder}
                />
              </div>

              <Tabs
                value={activeTab}
                onValueChange={(val) => {
                  if (!TABS.some((tab) => tab.value === val)) return;
                  setActiveTab(val as CreatorTab);
                  setCurrentPage(1);
                }}
                className="mt-2 w-full"
              >
                <TabsList className="flex flex-wrap w-full rounded-none bg-transparent gap-2 justify-start px-2 mb-2 border-b">
                  {TABS.filter((tab) => countFor(data.counts, tab.value) > 0).map((tab) => (
                    <TabsTrigger
                      key={tab.value}
                      value={tab.value}
                      className="flex-1 cursor-pointer text-primary data-[state=active]:bg-primary data-[state=active]:text-background dark:data-[state=active]:bg-primary dark:data-[state=active]:text-background hover:brightness-120 active:scale-95"
                    >
                      {tab.label} ({countFor(data.counts, tab.value)})
                    </TabsTrigger>
                  ))}
                </TabsList>

                <div className={cn("mt-4", loading && "opacity-60 transition-opacity")}>
                  {data.tab === "movieSets" ? (
                    <TabsContent value="movieSets" forceMount>
                      <ResponsiveGrid size="regular">
                        {data.sets.map((set) => (
                          <MovieSetCard key={set.id} set={set} includedItems={includedItems} />
                        ))}
                      </ResponsiveGrid>
                    </TabsContent>
                  ) : data.tab === "boxSets" ? (
                    <TabsContent value="boxSets" forceMount>
                      <div className="divide-y divide-primary-dynamic/20 space-y-6">
                        {data.boxsets.map((boxset) => (
                          <div key={`${boxset.id}-boxset`} className="pb-6">
                            <RenderBoxsetDisplay includedItems={includedItems} set={boxset} />
                          </div>
                        ))}
                      </div>
                    </TabsContent>
                  ) : (
                    <TabsContent value={data.tab} forceMount>
                      <div className="divide-y divide-primary-dynamic/20 space-y-6">
                        {data.sets.map((set) => (
                          <div key={`${set.id}-${data.tab}`} className="pb-6">
                            <RenderShowAndCollectionDisplay includedItems={includedItems} set={set} />
                          </div>
                        ))}
                      </div>
                    </TabsContent>
                  )}
                </div>
              </Tabs>

              {/* Pagination */}
              <CustomPagination
                currentPage={data.page}
                totalPages={data.total_pages}
                setCurrentPage={setCurrentPage}
                scrollToTop={true}
                filterItemsLength={data.total}
                itemsPerPage={data.per_page}
              />
            </div>
          )}
        </div>
      )}

      {/* Refresh Button */}
      <RefreshButton onClick={() => void handleRefresh()} refreshing={refreshing} />
    </div>
  );
}

function MovieSetCard({ set, includedItems }: { set: SetRef; includedItems: { [tmdb_id: string]: IncludedItem } }) {
  const setImages = set.images ?? [];
  const poster = setImages.find((image) => image.type === "poster");
  const backdrop = setImages.find((image) => image.type === "backdrop");
  return (
    <div
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
          <DownloadModal baseSetInfo={set} formItems={setRefsToFormItems([set], includedItems)} />
        </div>
        {/* Set Name */}
        <P className="text-primary-dynamic text-sm font-semibold w-full mb-1 truncate pr-10">{set.title}</P>
      </div>

      {/* Set User Name */}
      <div className="flex items-center justify-start w-full mb-1">
        <div className="flex items-center gap-1">
          <CreatorAvatar username={set.user_created} className="rounded-lg mr-1 w-4 h-4" />
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
        Last Update: {formatLastUpdatedDate(set.date_updated, set.date_created || setImages[0]?.modified || "")}
      </Lead>

      {/* Poster */}
      {poster && (
        <AssetImage
          image={poster}
          imageType="mediux"
          aspect="poster"
          className="w-full mb-2"
          includedItems={includedItems}
          matchedToItem={true}
        />
      )}

      {/* Backdrop */}
      {backdrop && (
        <AssetImage
          image={backdrop}
          imageType="mediux"
          aspect="backdrop"
          className="w-full"
          includedItems={includedItems}
          matchedToItem={true}
        />
      )}
    </div>
  );
}
