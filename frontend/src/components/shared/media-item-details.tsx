"use client";

import { IgnoreItemInDB, StopIgnoringItemInDB } from "@/services/database/ignore";
import {
  ChevronDown,
  Database,
  EyeClosedIcon,
  EyeOffIcon,
  FileIcon,
  ImageIcon,
  RefreshCcw,
  Star,
  Trash2,
} from "lucide-react";
import { toast } from "sonner";

import { useEffect, useRef, useState } from "react";

import Link from "next/link";
import { useRouter } from "next/navigation";

import { AssetImage } from "@/components/shared/asset-image";
import { ViewCurrentImagesModal } from "@/components/shared/media-item-images-modal";
import { ManualImportModal } from "@/components/shared/media-item-manual-import-modal";
import { MediaItemRatingModal } from "@/components/shared/media-item-rating-modal";
import { MediaItemRatings } from "@/components/shared/media-item-ratings";
import { RefreshMetadataModal } from "@/components/shared/media-item-refresh-modal";
import { Accordion, AccordionContent, AccordionItem, AccordionTrigger } from "@/components/ui/accordion";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { H1, Lead } from "@/components/ui/typography";

import { cn } from "@/lib/cn";
import { useMediaStore } from "@/lib/stores/global-store-media-store";
import { useSearchQueryStore } from "@/lib/stores/global-store-search-query";

import type { MediaItem } from "@/types/media-and-posters/media-item-and-library";

type MediaItemDetailsProps = {
  mediaItem?: MediaItem;

  status: string;
  otherMediaItem: MediaItem | null;
  posterImageKeys?: string[];
  serverType: string;

  existsInDB: boolean;
  ignoredInDB?: boolean;
  ignoredMode?: string;
  currentSetsAvailable?: string[];
  onArtworkApplied?: (item: MediaItem) => void;
};

export function MediaItemDetails({
  mediaItem,
  status,
  otherMediaItem,
  posterImageKeys,
  serverType,
  existsInDB,
  ignoredInDB,
  ignoredMode,
  currentSetsAvailable = [],
  onArtworkApplied,
}: MediaItemDetailsProps) {
  const [isInDBLocal, setIsInDBLocal] = useState(existsInDB);
  const [isIgnoredLocal, setIsIgnoredLocal] = useState(ignoredInDB);
  const [ignoreModeLocal, setIgnoreModeLocal] = useState(ignoredMode);

  const router = useRouter();
  const { setMediaItem } = useMediaStore();
  const { setSearchQuery } = useSearchQueryStore();

  const [currentPosterIndex, setCurrentPosterIndex] = useState(0);

  // Action Modals
  const [isRefreshMetadataModalOpen, setIsRefreshMetadataModalOpen] = useState(false);
  const [isRatingModalOpen, setIsRatingModalOpen] = useState(false);
  const [isManualImportModalOpen, setIsManualImportModalOpen] = useState(false);
  const [isViewCurrentImagesModalOpen, setIsViewCurrentImagesModalOpen] = useState(false);

  const touchStartXRef = useRef<number | undefined>(undefined);
  const mouseStartXRef = useRef<number | undefined>(undefined);

  const title = mediaItem?.title || "";
  const year = mediaItem?.year || 0;
  const contentRating = mediaItem?.content_rating || "";
  const mediaItemType = mediaItem?.type || "";
  const summary = mediaItem?.summary || "";
  const tmdbID = mediaItem?.tmdb_id || "";
  const libraryTitle = mediaItem?.library_title || "";
  const seasonCount = mediaItemType === "show" ? mediaItem?.series?.season_count || 0 : 0;
  const episodeCount = mediaItemType === "show" ? mediaItem?.series?.episode_count || 0 : 0;
  const moviePath = mediaItem?.movie?.file?.path || "N/A";
  const movieSize = mediaItem?.movie?.file?.size || 0;
  const movieDuration = mediaItem?.movie?.file?.duration || 0;
  const guids = mediaItem?.guids || [];

  // Sync when parent prop changes (e.g. route change or external update)
  useEffect(() => {
    setIsInDBLocal(existsInDB);
    setIsIgnoredLocal(ignoredInDB || false);
    setIgnoreModeLocal(ignoredMode || "");
  }, [existsInDB, ignoredInDB, ignoredMode]);

  const updateIgnoredInDB = (next: boolean) => {
    setIsIgnoredLocal(next); // local optimistic
  };

  const updateIgnoreModeInDB = (next: string) => {
    setIgnoreModeLocal(next); // local optimistic
  };

  const handleSavedSetsPageClick = () => {
    if (isInDBLocal) {
      setSearchQuery(`${title} Y:${year}: ID:${tmdbID}: L:${libraryTitle}:`);
      router.push("/saved-sets");
    }
  };

  const handleAddToIgnoredClick = async (ignoreMode: string) => {
    const addToDBResp = await IgnoreItemInDB(tmdbID, libraryTitle, ignoreMode, currentSetsAvailable);
    if (addToDBResp.status === "error") {
      toast.error(`Failed to add ${title} to DB: ${addToDBResp.error?.message || "Unknown error"}`);
      return;
    }

    updateIgnoredInDB(true);
    updateIgnoreModeInDB(ignoreMode);
    let toastMessage = "";
    if (ignoreMode === "always") {
      toastMessage = `Will always ignore ${title}`;
    } else if (ignoreMode === "until-set-available") {
      toastMessage = `Will ignore ${title} until a set is available`;
    } else if (ignoreMode === "until-new-set-available") {
      toastMessage = `Will ignore ${title} until a new set is available`;
    }
    toast.success(toastMessage);
  };

  const handleStopIgnoringClick = async () => {
    if (!mediaItem) {
      toast.error("Media item data is missing");
      return;
    }

    const addToDBResp = await StopIgnoringItemInDB(mediaItem.tmdb_id, mediaItem.library_title);
    if (addToDBResp.status === "error") {
      toast.error(`Failed to stop ignoring ${title}`);
      return;
    }
    updateIgnoredInDB(false);
    toast.success(`No longer ignoring ${title}`);
  };

  return (
    <div>
      <div className="flex flex-col lg:flex-row pt-30 items-center lg:items-start text-center lg:text-left">
        {/* Poster Image */}
        {posterImageKeys && posterImageKeys.length > 0 && (
          <div
            className="flex-shrink-0 mb-4 lg:mb-0 lg:mr-8 flex justify-center"
            onDoubleClick={() => setCurrentPosterIndex(0)}
            onTouchStart={(e) => {
              touchStartXRef.current = e.touches[0].clientX;
            }}
            onTouchEnd={(e) => {
              const startX = touchStartXRef.current;
              const endX = e.changedTouches[0].clientX;
              if (startX !== undefined) {
                if (endX - startX > 50) {
                  setCurrentPosterIndex((prev) => (prev - 1 + posterImageKeys.length) % posterImageKeys.length);
                } else if (startX - endX > 50) {
                  setCurrentPosterIndex((prev) => (prev + 1) % posterImageKeys.length);
                }
              }
              touchStartXRef.current = undefined;
            }}
            onMouseDown={(e) => {
              mouseStartXRef.current = e.clientX;
            }}
            onMouseUp={(e) => {
              const startX = mouseStartXRef.current;
              const endX = e.clientX;
              if (startX !== undefined) {
                if (endX - startX > 50) {
                  setCurrentPosterIndex((prev) => (prev - 1 + posterImageKeys.length) % posterImageKeys.length);
                } else if (startX - endX > 50) {
                  setCurrentPosterIndex((prev) => (prev + 1) % posterImageKeys.length);
                }
              }
              mouseStartXRef.current = undefined;
            }}
          >
            <AssetImage
              image={`/api/images/media/item?rating_key=${mediaItem?.rating_key}&image_rating_key=${posterImageKeys[currentPosterIndex]}&image_type=poster&cb=${Date.now()}`}
              imageType="url"
              className="w-[200px] h-auto transition-transform hover:scale-105 select-none"
            />
          </div>
        )}

        {/* Title and Summary */}
        <div className="flex flex-col items-center lg:items-start">
          <H1 className="mb-1">{title}</H1>
          {/* Hide summary on mobile */}
          <Lead className="text-primary-dynamic max-w-xl hidden lg:block">{summary}</Lead>

          {/* Year, Content Rating And External Ratings/Links */}
          <div className="flex flex-wrap lg:flex-nowrap justify-center lg:justify-start items-center gap-4 tracking-wide mt-4">
            {/* Year */}
            {year && <Badge className="flex items-center text-sm">{year}</Badge>}

            {/* Content Rating */}
            {contentRating && <Badge className="flex items-center text-sm">{contentRating}</Badge>}

            {status ? (
              <Badge
                className={cn(
                  "flex items-center text-sm",
                  (status.toLowerCase() === "ended" ||
                    status.toLowerCase() === "cancelled" ||
                    status.toLowerCase() === "canceled") &&
                    "bg-red-700 text-white",
                  status.toLowerCase().startsWith("returning") && "bg-green-700 text-white"
                )}
              >
                {status.toLowerCase().startsWith("returning") ? "Continuing" : status}
              </Badge>
            ) : null}
          </div>

          {/* External Ratings/Links from GUIDs */}
          <div className="flex flex-wrap lg:flex-nowrap justify-center lg:justify-start items-center gap-4 tracking-wide mt-4">
            <MediaItemRatings guids={guids} mediaItemType={mediaItemType} title={title} />
          </div>
        </div>
      </div>

      {/* Library Information */}
      {libraryTitle ? (
        <div className="flex flex-wrap lg:flex-nowrap justify-center lg:justify-start items-center gap-4 tracking-wide mt-0 md:mt-2">
          <Lead className="text-md text-primary-dynamic ml-1">
            <span className="font-semibold">{libraryTitle} Library</span>{" "}
          </Lead>
        </div>
      ) : null}

      {/* Other Media Item Information */}
      {otherMediaItem ? (
        <div className="flex flex-wrap lg:flex-nowrap justify-center lg:justify-start items-center gap-4 tracking-wide mt-1">
          <Lead className="text-md text-primary-dynamic ml-1">
            Also available in{" "}
            <Link
              href={"/media-item/"}
              onClick={() => {
                setMediaItem(otherMediaItem);
                router.push("/media-item/");
              }}
              className="text-primary-dynamic hover:text-primary underline"
            >
              {otherMediaItem.library_title}
            </Link>{" "}
            Library
          </Lead>
        </div>
      ) : null}

      {/* Show Existence in Database */}
      {mediaItem && !isIgnoredLocal && (
        <div className="flex flex-wrap lg:flex-nowrap justify-center lg:justify-start items-center gap-4 tracking-wide mt-2">
          <Lead
            className={cn("text-md", isInDBLocal ? "hover:cursor-pointer text-green-500" : "text-red-500")}
            onClick={() => {
              if (isInDBLocal) handleSavedSetsPageClick();
            }}
          >
            <Database className="inline ml-1 mr-1" /> {isInDBLocal ? "Already in Database" : "Not in Database"}
          </Lead>
        </div>
      )}

      {/* Show Existence in Database */}
      {mediaItem && isIgnoredLocal && (
        <div className="flex flex-wrap lg:flex-nowrap justify-center lg:justify-start items-center gap-4 tracking-wide mt-2">
          {ignoreModeLocal === "always" ? (
            <Lead className="text-md text-red-500">
              <EyeOffIcon className="inline ml-1 mr-1" size={16} /> This item is set to be always ignored
            </Lead>
          ) : ignoreModeLocal === "until-set-available" ? (
            <Lead className="text-md text-orange-500">
              <EyeClosedIcon className="inline ml-1 mr-1" size={16} /> This item is set to be temporarily ignored until
              a set is available
            </Lead>
          ) : (
            <Lead className="text-md text-yellow-500">
              <EyeClosedIcon className="inline ml-1 mr-1" size={16} /> This item is set to be temporarily ignored until
              a new set is available
            </Lead>
          )}
        </div>
      )}

      {/* Actions */}
      <div className="flex flex-wrap lg:flex-nowrap justify-center lg:justify-start items-center gap-4 tracking-wide mt-2">
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button
              variant="outline"
              size="sm"
              className={cn("gap-2 px-3", "border border-border shadow-sm", "hover:bg-secondary/80")}
            >
              Actions
              <ChevronDown className="h-4 w-4 opacity-80" />
            </Button>
          </DropdownMenuTrigger>

          <DropdownMenuContent align="start" side="bottom" className="min-w-[220px]">
            <DropdownMenuItem className="cursor-pointer" onSelect={() => setIsRefreshMetadataModalOpen(true)}>
              <RefreshCcw className="mr-2 h-4 w-4" />
              Refresh Metadata
            </DropdownMenuItem>
            <DropdownMenuItem className="cursor-pointer" onSelect={() => setIsRatingModalOpen(true)}>
              <Star className="mr-2 h-4 w-4" />
              Rate {mediaItemType === "movie" ? "Movie" : "Show"}
            </DropdownMenuItem>
            <DropdownMenuItem className="cursor-pointer" onSelect={() => setIsManualImportModalOpen(true)}>
              <FileIcon className="mr-2 h-4 w-4" />
              Manual YAML Import
            </DropdownMenuItem>
            {mediaItem && mediaItem.type === "show" && mediaItem.series && (
              <DropdownMenuItem className="cursor-pointer" onSelect={() => setIsViewCurrentImagesModalOpen(true)}>
                <ImageIcon className="mr-2 h-4 w-4" />
                View Current Images
              </DropdownMenuItem>
            )}
            <DropdownMenuSeparator />
            {isInDBLocal ? (
              <DropdownMenuItem
                className="cursor-pointer"
                onSelect={() => {
                  setSearchQuery(`${title} Y:${year}: ID:${tmdbID}: L:${libraryTitle}:`);
                  router.push("/saved-sets");
                }}
              >
                <Database className="mr-2 h-4 w-4" />
                View in Saved Sets
              </DropdownMenuItem>
            ) : isIgnoredLocal ? (
              <DropdownMenuItem
                className="cursor-pointer"
                onSelect={() => {
                  void handleStopIgnoringClick();
                }}
              >
                <Trash2 className="mr-2 h-4 w-4" />
                Remove Ignore Status
              </DropdownMenuItem>
            ) : (
              <>
                <DropdownMenuItem
                  className="cursor-pointer text-destructive focus:text-destructive"
                  onSelect={() => {
                    void handleAddToIgnoredClick("always");
                  }}
                >
                  <EyeOffIcon className="text-red-500" size={20} />
                  Always Ignore
                </DropdownMenuItem>

                {!mediaItem?.has_mediux_sets && currentSetsAvailable.length === 0 && (
                  <DropdownMenuItem
                    className="cursor-pointer text-orange-500 focus:text-orange-800"
                    onSelect={() => {
                      void handleAddToIgnoredClick("until-set-available");
                    }}
                  >
                    <EyeClosedIcon className="text-orange-500" size={20} />
                    Ignore Until Set is Available
                  </DropdownMenuItem>
                )}

                {currentSetsAvailable.length > 0 && (
                  <DropdownMenuItem
                    className="cursor-pointer text-yellow-500 focus:text-yellow-800"
                    onSelect={() => {
                      void handleAddToIgnoredClick("until-new-set-available");
                    }}
                  >
                    <EyeClosedIcon className="text-yellow-500" size={20} />
                    Ignore Until New Set is Available
                  </DropdownMenuItem>
                )}
              </>
            )}
          </DropdownMenuContent>
        </DropdownMenu>
      </div>

      {/* Season/Episode Information */}
      {mediaItemType === "show" && seasonCount > 0 && episodeCount > 0 ? (
        <div className="flex flex-wrap lg:flex-nowrap justify-center lg:justify-start items-center gap-4 tracking-wide mt-2">
          <Lead className="flex items-center text-md text-primary-dynamic ml-1">
            {seasonCount} {seasonCount > 1 ? "Seasons" : "Season"} with {episodeCount}{" "}
            {episodeCount > 1 ? "Episodes" : "Episode"} in {serverType}
          </Lead>
        </div>
      ) : null}

      {/* Movie Information */}
      <div className="lg:flex items-center text-white gap-8 tracking-wide mt-4">
        {mediaItemType === "movie" ? (
          <div className="flex flex-col w-full items-center">
            <Accordion type="single" collapsible className="w-full">
              <AccordionItem value="movie-details">
                <AccordionTrigger className="text-primary font-semibold">Movie Details</AccordionTrigger>
                <AccordionContent className="mt-2 text-sm text-muted-foreground space-y-2">
                  {moviePath ? (
                    <p>
                      <span className="font-semibold">File Path:</span> {moviePath}
                    </p>
                  ) : null}

                  {movieSize ? (
                    <p>
                      <span className="font-semibold">File Size:</span>{" "}
                      {movieSize >= 1024 * 1024 * 1024
                        ? `${(movieSize / (1024 * 1024 * 1024)).toFixed(2)} GB`
                        : `${(movieSize / (1024 * 1024)).toFixed(2)} MB`}
                    </p>
                  ) : null}

                  {movieDuration ? (
                    <p>
                      <span className="font-semibold">Duration:</span>{" "}
                      {movieDuration < 3600000
                        ? `${Math.floor(movieDuration / 60000)} minutes`
                        : `${Math.floor(movieDuration / 3600000)}hr ${Math.floor(
                            (movieDuration % 3600000) / 60000
                          )}min`}
                    </p>
                  ) : null}
                </AccordionContent>
              </AccordionItem>
            </Accordion>
          </div>
        ) : null}
      </div>

      {/*	Action Modals */}
      {/* Refresh Metadata Modal */}
      {mediaItem && (
        <RefreshMetadataModal
          mediaItem={mediaItem}
          isOpen={isRefreshMetadataModalOpen}
          onClose={() => setIsRefreshMetadataModalOpen(false)}
        />
      )}

      {/* Rating Modal */}
      {mediaItem && (
        <MediaItemRatingModal
          mediaItem={mediaItem}
          isOpen={isRatingModalOpen}
          onClose={() => setIsRatingModalOpen(false)}
        />
      )}

      {/* Manual Import Modal */}
      {mediaItem && (
        <ManualImportModal
          mediaItem={mediaItem}
          isOpen={isManualImportModalOpen}
          onClose={() => setIsManualImportModalOpen(false)}
          onArtworkApplied={onArtworkApplied}
        />
      )}

      {/* View Current Images Modal */}
      {mediaItem && mediaItem.type === "show" && mediaItem.series && (
        <ViewCurrentImagesModal
          mediaItem={mediaItem}
          isOpen={isViewCurrentImagesModalOpen}
          onClose={() => setIsViewCurrentImagesModalOpen(false)}
        />
      )}
    </div>
  );
}
