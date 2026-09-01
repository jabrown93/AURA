"use client";

import { formatUnixSeconds } from "@/helper/format-date-last-updates";
import { ReturnErrorMessage } from "@/services/api-error-return";
import { HandleSearch } from "@/services/search/api-search";
import { BookmarkIcon, EyeOff, Film, FilmIcon, Search, Star, TvIcon, User, UserIcon, X } from "lucide-react";
import { AnimatePresence, motion } from "motion/react";

import * as React from "react";
import { useEffect, useMemo, useRef, useState } from "react";

import Image from "next/image";
import Link from "next/link";
import { useRouter } from "next/navigation";

import { ErrorMessage } from "@/components/shared/error-message";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import AuraSpinner from "@/components/ui/mediux-spinner";
import { Separator } from "@/components/ui/separator";

import { cn } from "@/lib/cn";
import { log } from "@/lib/logger";
import { useMediaStore } from "@/lib/stores/global-store-media-store";
import { useSearchQueryStore } from "@/lib/stores/global-store-search-query";

import type { APIResponse } from "@/types/api/api-response";
import type { DBSavedItem } from "@/types/database/db-poster-set";
import type { MediaItem } from "@/types/media-and-posters/media-item-and-library";
import type { MediuxUserInfo } from "@/types/mediux/mediux-user-follow-hide";

// Props for DynamicSearch component
export interface DynamicSearchProps {
  placeholder?: string;
  className?: string;
}

// Types for filter keys
type SearchTypeFilter = "mediaItem" | "collectionItem" | "mediuxUser" | "savedSets";
const filterOrder: SearchTypeFilter[] = ["mediaItem", "collectionItem", "mediuxUser", "savedSets"];

// Animation variants for results
const wrapperVariants = {
  open: {
    transition: { staggerChildren: 0.1, delayChildren: 0.2 },
  },
  closed: {
    transition: { staggerChildren: 0.1, staggerDirection: -1 },
  },
};

const itemVariants = {
  open: {
    opacity: 1,
    y: 0,
    transition: { duration: 0.2 },
  },
  closed: {
    opacity: 0,
    y: 20,
    transition: { duration: 0.2 },
  },
};

export function DynamicSearch({ placeholder = "Search", className }: DynamicSearchProps) {
  const router = useRouter();

  // --- States and Refs ---

  const [error, setError] = useState<APIResponse<unknown> | null>(null);

  // Global Search Store
  const { searchQuery, setSearchQuery } = useSearchQueryStore(); // Global store for search query

  const [searchInput, setSearchInput] = useState(searchQuery); // Local state for input field

  // Results UI States
  const [isExpanded, setIsExpanded] = useState(false);
  const [isSearching, setIsSearching] = useState(false);
  const [isLoading, setIsLoading] = useState(false);

  // Refs for click outside detection
  const inputRef = useRef<HTMLInputElement>(null);
  const resultsRef = useRef<HTMLDivElement>(null);

  // Media Store
  const { setMediaItem } = useMediaStore();

  // Filter States
  const [filters, setFilters] = useState<{ [key: string]: boolean }>({
    mediaItem: true,
    collectionItem: true,
    mediuxUser: true,
    savedSets: false,
  });

  // Focused Index for keyboard navigation
  const [focusedIndex, setFocusedIndex] = useState<number>(-1);

  // Search Results States
  const [searchResultMediaItems, setSearchResultMediaitems] = useState<MediaItem[]>([]);
  const [searchResultMediaItemLastUpdate, setSearchResultMediaItemLastUpdate] = useState<number>(0);
  const [searchResultMediuxUsers, setSearchResultsMediuxUsers] = useState<MediuxUserInfo[]>([]);
  const [searchResultMediuxUserLastUpdate, setSearchResultMediuxUserLastUpdate] = useState<number>(0);
  const [searchResultSavedSets, setSearchResultsSavedSets] = useState<DBSavedItem[]>([]);

  // --- Handlers and Effects ---

  // Input Change Handler
  const handleInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const value = e.target.value;
    setSearchInput(value);
    setIsExpanded(value.length > 0 || document.activeElement === inputRef.current);
    setFocusedIndex(-1);
  };

  // Input Focus Handler
  const handleInputFocus = () => {
    setIsExpanded(searchInput.length > 0);
  };

  // Toggle Filter Handler
  const toggleFilter = (filter: SearchTypeFilter) => {
    setFilters((prev) => ({ ...prev, [filter]: !prev[filter] }));
  };

  // Memoized Filtered Results
  const filteredMediaItems = useMemo(() => {
    return filters.mediaItem ? searchResultMediaItems : [];
  }, [searchResultMediaItems, filters.mediaItem]);

  const filteredMediuxUsers = useMemo(() => {
    return filters.mediuxUser ? searchResultMediuxUsers : [];
  }, [searchResultMediuxUsers, filters.mediuxUser]);

  const filteredSavedSets = useMemo(() => {
    return filters.savedSets ? searchResultSavedSets : [];
  }, [searchResultSavedSets, filters.savedSets]);

  const hasFilteredResults = filteredMediaItems.length > 0 || filteredMediuxUsers.length > 0;

  // Use Effect - To handle clicks outside the search results to close the dropdown
  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      if (
        resultsRef.current &&
        !resultsRef.current.contains(event.target as Node) &&
        inputRef.current &&
        !inputRef.current.contains(event.target as Node)
      ) {
        setIsExpanded(false);
        setFocusedIndex(-1);
      }
    }
    document.addEventListener("mousedown", handleClickOutside);
    return () => {
      document.removeEventListener("mousedown", handleClickOutside);
    };
  }, [resultsRef, inputRef]);

  // Clear Results
  const clearAllResults = () => {
    setSearchResultMediaitems([]);
    setSearchResultMediaItemLastUpdate(0);
    setSearchResultsMediuxUsers([]);
    setSearchResultMediuxUserLastUpdate(0);
    setSearchResultsSavedSets([]);
  };

  // If the search query is changed externally, update the local input state
  useEffect(() => {
    if (searchQuery.trim() === "") {
      setSearchInput("");
      setIsSearching(false);
      clearAllResults();
    } else {
      setSearchInput(searchQuery);
    }
  }, [searchQuery]);

  // Use Effect - To perform search when searchInput changes (with debounce of 500ms)
  useEffect(() => {
    if (searchInput.trim() === "") {
      setIsSearching(false);
      clearAllResults();
      setSearchQuery("");
      return;
    }

    setIsLoading(true);
    setIsSearching(true);
    setIsExpanded(true);

    const delayDebounceFn = setTimeout(async () => {
      setIsSearching(true);
      setError(null);
      setSearchQuery(searchInput);

      try {
        const searchResp = await HandleSearch({
          searchQuery: searchInput,
          searchMediaItems: filters.mediaItem,
          searchCollectionItems: filters.collectionItem,
          searchMediuxUsers: filters.mediuxUser,
          searchSavedSets: filters.savedSets,
        });
        if (searchResp.status === "error") {
          setError(searchResp);
          return;
        }
        const results = searchResp.data;
        const respError = searchResp.error;

        if (respError) {
          setError(searchResp);
        }

        setSearchResultMediaitems(results?.media_items || []);
        setSearchResultMediaItemLastUpdate(results?.media_items_last_full_update || 0);
        setSearchResultsMediuxUsers(results?.mediux_usernames || []);
        setSearchResultMediuxUserLastUpdate(results?.mediux_usernames_last_full_update || 0);
        setSearchResultsSavedSets(results?.saved_sets || []);
      } catch (error) {
        clearAllResults();
        log("ERROR", "Search Bar", "Fetch", "Search failed");
        setError(ReturnErrorMessage<unknown>(error));
      } finally {
        setIsLoading(false);
      }
    }, 500);

    return () => clearTimeout(delayDebounceFn);
  }, [filters.mediaItem, filters.collectionItem, filters.mediuxUser, filters.savedSets, searchInput, setSearchQuery]);

  // Keyboard Navigation Handler
  // Handles keyboard events for navigating and selecting search results
  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (!isExpanded) return;

    const totalItems = filteredMediaItems.length + filteredMediuxUsers.length + filteredSavedSets.length;

    if (e.key === "ArrowDown") {
      e.preventDefault();
      setFocusedIndex(
        (prev) => (totalItems === 0 ? -1 : prev < totalItems - 1 ? prev + 1 : 0) // loop to start
      );
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setFocusedIndex(
        (prev) => (totalItems === 0 ? -1 : prev > 0 ? prev - 1 : totalItems - 1) // loop to end
      );
    } else if (e.key === "Enter") {
      e.preventDefault();

      // If an item is focused, select it
      if (focusedIndex >= 0) {
        // Determine if we're selecting Media Items or MediUX Users
        if (focusedIndex < filteredMediaItems.length) {
          // Selecting from Media Items
          handleMediaItemClick(filteredMediaItems[focusedIndex]);
        } else if (focusedIndex < filteredMediaItems.length + filteredMediuxUsers.length) {
          // Selecting from MediUX Users
          handleMediuxUserClick(filteredMediuxUsers[focusedIndex - filteredMediaItems.length].username);
        } else {
          // Selecting from Saved Sets
          handleSavedSetsClick(
            filteredSavedSets[focusedIndex - filteredMediaItems.length - filteredMediuxUsers.length]
          );
        }

        setIsExpanded(false);
      } else {
        // No item focused, trigger manual search
      }
    } else if (e.key === "Escape") {
      setIsExpanded(false);
      setFocusedIndex(-1);
      inputRef.current?.blur();
    }
  };

  // Handle Media Item Click
  const handleMediaItemClick = (item: MediaItem) => {
    setMediaItem(item);
    setIsExpanded(false);
    router.push("/media-item/");
  };

  // Handle MediUX User Click
  const handleMediuxUserClick = (username: string) => {
    setIsExpanded(false);
    setSearchQuery("");
    setSearchInput("");
    router.push(`/user/${username}`);
  };

  // Handle Saved Sets Click
  const handleSavedSetsClick = (set: DBSavedItem) => {
    setSearchQuery(
      `${set.media_item.title} Y:${set.media_item.year}: ID:${set.media_item.tmdb_id}: L:${set.media_item.library_title}:`
    );
    setIsExpanded(false);
    router.push("/saved-sets/");
  };

  const mediaTypes = Array.from(new Set(filteredMediaItems.map((item) => item.type)));
  let mediaSectionTitle = "Media Items";
  if (mediaTypes.length === 1) {
    if (mediaTypes[0] === "movie") mediaSectionTitle = "Movies";
    else if (mediaTypes[0] === "show" || mediaTypes[0] === "series" || mediaTypes[0] === "tv")
      mediaSectionTitle = "Shows";
  } else if (mediaTypes.includes("movie") && mediaTypes.some((t) => t === "show")) {
    mediaSectionTitle = "Movies & Shows";
  }

  return (
    <div className={cn("w-full", className)}>
      <div className="flex w-full relative sm:max-w-lg sm:mx-auto">
        {/* Search Input */}
        <div className="relative flex-1">
          <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 text-muted-foreground h-4 w-4" />
          <Input
            ref={inputRef}
            type="text"
            placeholder={placeholder}
            value={searchInput}
            onChange={handleInputChange}
            onFocus={handleInputFocus}
            onKeyDown={handleKeyDown}
            className="pl-10 pr-10 py-3 text-lg border-2 rounded-xl transition-all duration-200 focus:border-primary-dynamic w-full"
          />
          {searchInput && (
            <button
              type="button"
              className="absolute right-3 top-1/2 transform -translate-y-1/2 text-muted-foreground hover:text-destructive focus:outline-none"
              aria-label="Clear search"
              onClick={() => {
                setSearchInput("");
                setSearchQuery("");
                clearAllResults();
                inputRef.current?.focus();
              }}
              tabIndex={0}
            >
              <X className="w-4 h-4" />
            </button>
          )}
        </div>
      </div>

      {/* Expanded Results */}
      <AnimatePresence>
        {isExpanded && isSearching && (
          <motion.div
            ref={resultsRef}
            initial={{ opacity: 0, y: 10, scale: 0.98 }}
            animate={{ opacity: 1, y: 0, scale: 1 }}
            exit={{ opacity: 0, y: 10, scale: 0.98 }}
            transition={{ duration: 0.2, ease: "easeOut" }}
            style={{ originY: "top" }}
            className={cn(
              // Mobile: fixed, full width, centered, with padding and max width
              "fixed top-16 left-0 right-0 w-full max-w-md mx-auto px-1 py-2 z-50 rounded-2xl bg-background dark:bg-background-dark border border-primary shadow-lg",
              // Desktop: absolute, matches search bar width
              "sm:absolute sm:top-full sm:left-0 sm:w-full sm:max-w-lg sm:mx-auto sm:mt-2 sm:rounded-xl"
            )}
          >
            <div className="p-4 max-h-[60vh] overflow-y-auto scrollbar-hide">
              {/* Filter Controls - Always show when searching */}
              {isSearching && (
                <>
                  <div className="flex items-center gap-2 mb-4 flex-wrap">
                    <span className="text-xs font-medium text-muted-foreground mr-2 shrink-0">Filter:</span>
                    <div className="flex items-center gap-2 flex-wrap">
                      {filterOrder.map((filterType) => (
                        <Badge
                          key={filterType}
                          variant={filters[filterType] ? "default" : "secondary"}
                          className={cn(
                            "capitalize text-xs cursor-pointer transition-colors",
                            filters[filterType]
                              ? "hover:bg-primary-dynamic/20"
                              : "text-muted-foreground hover:text-primary-dynamic hover:border-primary-dynamic/50"
                          )}
                          onClick={() => toggleFilter(filterType)}
                        >
                          {filterType === "mediaItem"
                            ? "Media Items"
                            : filterType === "collectionItem"
                              ? "Collection Items"
                              : filterType === "mediuxUser"
                                ? "MediUX Users"
                                : filterType === "savedSets"
                                  ? "Saved Sets"
                                  : filterType}
                        </Badge>
                      ))}
                    </div>
                  </div>
                  <Separator className="mb-4" />
                </>
              )}

              {/* Close Icon */}
              <Button
                size={"icon"}
                variant={"ghost"}
                className="absolute top-1 right-1"
                aria-label="Close search results"
                onClick={() => setIsExpanded(false)}
              >
                <X className="w-4 h-4 text-destructive" />
              </Button>

              {isLoading ? (
                <div className="text-center py-8 text-muted-foreground">
                  <AuraSpinner size="lg" className="mx-auto mb-2 text-primary-dynamic" />
                  <p>Searching...</p>
                </div>
              ) : hasFilteredResults ? (
                <motion.div variants={wrapperVariants} initial="closed" animate="open" exit="closed">
                  {/* Movies & TV Shows Section */}
                  {filteredMediaItems.length > 0 && (
                    <motion.div variants={itemVariants}>
                      <div className="mb-6">
                        <h3 className="text-sm font-semibold text-foreground mb-3 flex items-center gap-2">
                          {mediaTypes.includes("movie") && mediaTypes.includes("show") ? (
                            <>
                              <FilmIcon className="h-4 w-4" />
                              <TvIcon className="h-4 w-4" />
                            </>
                          ) : mediaTypes.includes("movie") ? (
                            <FilmIcon className="h-4 w-4" />
                          ) : (
                            <TvIcon className="h-4 w-4" />
                          )}
                          {mediaSectionTitle} ({filteredMediaItems.length})
                          {searchResultMediaItemLastUpdate > 0 && (
                            <span className="ml-auto text-[10px] text-muted-foreground whitespace-nowrap">
                              (Last Update: {formatUnixSeconds(searchResultMediaItemLastUpdate)})
                            </span>
                          )}
                        </h3>
                        <div className="space-y-1">
                          {filteredMediaItems.map((item, index) => {
                            const isHighlighted = index === focusedIndex;
                            return (
                              <SearchResultMediaItem
                                key={`media-item-${item.rating_key}`}
                                title={item.title}
                                subtitle={`${item.library_title} • ${item.year}`}
                                href={`/media-item/`}
                                isHighlighted={isHighlighted}
                                imageSrc={`/api/images/media/item?rating_key=${item.rating_key}&image_type=poster&v=${item.artwork_version}`}
                                imageAlt={item.title}
                                fallbackIcon={
                                  item.type === "movie" ? (
                                    <FilmIcon className="h-4 w-4 text-muted-foreground" />
                                  ) : (
                                    <TvIcon className="h-4 w-4 text-muted-foreground" />
                                  )
                                }
                                onLinkClick={() => {
                                  handleMediaItemClick(item);
                                }}
                              />
                            );
                          })}
                        </div>
                      </div>
                    </motion.div>
                  )}

                  {/* Saved Sets Section */}
                  {filteredSavedSets.length > 0 && (
                    <motion.div variants={itemVariants}>
                      {(filteredMediaItems.length > 0 || filteredMediuxUsers.length > 0) && (
                        <Separator className="mb-6" />
                      )}
                      <div className="mb-6">
                        <h3 className="text-sm font-semibold text-foreground mb-3 flex items-center gap-2">
                          <BookmarkIcon className="h-4 w-4" />
                          Saved Sets ({filteredSavedSets.length})
                        </h3>
                        <div className="space-y-1">
                          {filteredSavedSets.map((set, index) => {
                            const adjustedIndex = filteredMediaItems.length + index;
                            const isHighlighted = adjustedIndex === focusedIndex;
                            const subtitle = `${set.media_item.library_title} • ${
                              set.poster_sets.length > 1
                                ? `${set.poster_sets.length} sets`
                                : `Set by ${set.poster_sets[0].user_created}`
                            }`;
                            return (
                              <SearchResultMediaItem
                                key={`saved-set-${set.media_item.library_title}-${set.media_item.tmdb_id}`}
                                title={set.media_item.title}
                                subtitle={subtitle}
                                href={`/saved-sets/`}
                                isHighlighted={isHighlighted}
                                imageSrc={`/api/images/media/item?rating_key=${set.media_item.rating_key}&image_type=poster&v=${set.media_item.artwork_version}`}
                                imageAlt={set.media_item.title}
                                fallbackIcon={<Film className="h-4 w-4 text-muted-foreground" />}
                                onLinkClick={() => {
                                  handleSavedSetsClick(set);
                                }}
                              />
                            );
                          })}
                        </div>
                      </div>
                    </motion.div>
                  )}

                  {/* MediUX Users Section */}
                  {filteredMediuxUsers.length > 0 && (
                    <motion.div variants={itemVariants}>
                      {filteredMediaItems.length > 0 && <Separator className="mb-6" />}
                      <div className="mb-6">
                        <h3 className="text-sm font-semibold text-foreground mb-3 flex items-center gap-2">
                          <UserIcon className="h-4 w-4" />
                          MediUX Users ({filteredMediuxUsers.length})
                          {searchResultMediuxUserLastUpdate > 0 && (
                            <span className="ml-auto text-[10px] text-muted-foreground whitespace-nowrap">
                              (Last Update: {formatUnixSeconds(searchResultMediuxUserLastUpdate)})
                            </span>
                          )}
                        </h3>

                        <div className="space-y-1">
                          {filteredMediuxUsers.map((item, index) => {
                            const adjustedIndex = filteredMediaItems.length + filteredSavedSets.length + index;
                            const isHighlighted = adjustedIndex === focusedIndex;
                            return (
                              <SearchResultUserItem
                                key={`user-${item.username}`}
                                user={item}
                                isHighlighted={isHighlighted}
                                onLinkClick={() => {
                                  handleMediuxUserClick(item.username);
                                }}
                              />
                            );
                          })}
                        </div>
                      </div>
                    </motion.div>
                  )}

                  {error && <ErrorMessage error={error} />}
                </motion.div>
              ) : isSearching ? (
                <div className="text-center py-8 text-muted-foreground">
                  {!error ? (
                    <>
                      <Search className="h-8 w-8 mx-auto mb-2 opacity-50" />
                      <p>No results found for '{searchInput}'</p>
                      <p className="text-xs mt-1">Try adjusting your search terms or filter settings</p>
                    </>
                  ) : (
                    <>
                      <Search className="h-8 w-8 mx-auto mb-2 opacity-50" />
                      <p className="text-sm mt-1">There was an error processing your search of</p>
                      <p className="text-sm mt-1">'{searchInput}'</p>
                      <ErrorMessage error={error} />
                    </>
                  )}
                </div>
              ) : null}
            </div>
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  );
}

// Reusable search result item component
interface SearchResultItemProps {
  title: string;
  subtitle: string;
  href: string;
  isHighlighted: boolean;
  imageSrc?: string;
  imageAlt: string;
  fallbackIcon?: React.ReactNode;
  onLinkClick?: () => void;
}

const SearchResultMediaItem: React.FC<SearchResultItemProps> = ({
  title,
  subtitle,
  href,
  isHighlighted,
  imageSrc,
  imageAlt,
  fallbackIcon,
  onLinkClick,
}) => {
  return (
    <Link
      href={href}
      className={cn(
        "flex items-center gap-3 p-1.5 rounded-lg cursor-pointer transition-colors group",
        isHighlighted ? "bg-muted" : "hover:bg-primary/50"
      )}
      onClick={onLinkClick}
      aria-label={title}
      title={title}
    >
      {imageSrc ? (
        <div className="relative w-[24px] h-[35px] rounded overflow-hidden">
          <Image src={imageSrc} alt={imageAlt} fill className="object-cover" loading="lazy" unoptimized />
        </div>
      ) : (
        <div className="h-10 w-10 shrink-0 rounded-md bg-muted flex items-center justify-center">{fallbackIcon}</div>
      )}
      <div className="flex-1 min-w-0">
        <div className="font-medium text-sm truncate">{title}</div>
        <div className="text-xs text-muted-foreground truncate">{subtitle}</div>
      </div>
    </Link>
  );
};

interface SearchResultUserItemProps {
  user: MediuxUserInfo;
  isHighlighted: boolean;
  onLinkClick?: () => void;
}

const SearchResultUserItem: React.FC<SearchResultUserItemProps> = ({ user, isHighlighted, onLinkClick }) => {
  const avatarSrc = user.avatar
    ? `/api/images/mediux/avatar?avatar_id=${user.avatar}`
    : `/api/images/mediux/avatar?username=${user.username}`;

  return (
    <Link
      href={`/user/${user.username}`}
      className={cn(
        "flex items-center gap-3 p-1.5 rounded-lg cursor-pointer transition-colors group",
        isHighlighted ? "bg-muted" : "hover:bg-primary/50"
      )}
      onClick={onLinkClick}
      aria-label={user.username}
      title={user.username}
    >
      <Avatar className="rounded-lg mr-1 w-7 h-7 min-w-[1.75rem] min-h-[1.75rem]">
        <AvatarImage src={avatarSrc} className="w-7 h-7" />
        <AvatarFallback className="">
          <User className="w-4 h-4" />
        </AvatarFallback>
      </Avatar>
      <div className="flex-1 min-w-0">
        <div className="font-medium text-sm truncate">{user.username}</div>
        <div className="text-xs text-muted-foreground truncate">{`${user.total_sets ?? 0} ${user.total_sets === 1 ? "set" : "sets"}`}</div>
      </div>

      <div className="flex items-center gap-1">
        {user.follow && <Star className="h-4 w-4 text-yellow-400" />}
        {user.hide && <EyeOff className="h-4 w-4 text-red-500" />}
      </div>
    </Link>
  );
};
