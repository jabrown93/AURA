"use client";

import {
  AlertTriangle,
  Badge,
  Delete,
  DownloadCloudIcon,
  Edit,
  Loader,
  MoreHorizontal,
  RefreshCcw,
  RefreshCwOff,
} from "lucide-react";
import { toast } from "sonner";

import { useMemo, useState } from "react";

import Link from "next/link";

import { AssetImage } from "@/components/shared/asset-image";
import { hasSelectedTypesOverlapOnAutoDownload } from "@/components/shared/saved-sets-cards";
import {
  SavedSetDeleteModal,
  SavedSetEditModal,
  SavedSetsList,
  onCloseSavedSetsEditDeleteModals,
  refreshPosterSet,
  renderTypeBadges,
  savedSetsConfirmDelete,
  savedSetsConfirmEdit,
} from "@/components/shared/saved-sets-shared";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { TableCell, TableRow } from "@/components/ui/table";

import { cn } from "@/lib/cn";
import { useMediaStore } from "@/lib/stores/global-store-media-store";

import type { APIResponse } from "@/types/api/api-response";
import type { DBSavedItem } from "@/types/database/db-poster-set";
import type { IncludedItem, SetRef } from "@/types/media-and-posters/sets";

const SavedSetsTableRow: React.FC<{
  savedSet: DBSavedItem;
  onUpdate: () => void;
  handleRecheckItem: (title: string, item: DBSavedItem) => void;
  handleRedownloadItem: (item: DBSavedItem) => void;
  bulkEditMode: boolean;
  bulkEditSelectedItems: Set<string>;
  setBulkEditSelectedItems: React.Dispatch<React.SetStateAction<Set<string>>>;
}> = ({
  savedSet,
  onUpdate,
  handleRecheckItem,
  handleRedownloadItem,
  bulkEditMode,
  bulkEditSelectedItems,
  setBulkEditSelectedItems,
}) => {
  // Normalize poster_sets so we never crash on undefined
  const posterSets = useMemo(() => savedSet.poster_sets ?? [], [savedSet.poster_sets]);
  const normalizedSavedSet = useMemo(() => ({ ...savedSet, poster_sets: posterSets }), [savedSet, posterSets]);

  // Initialize edit state from the poster sets array.
  const [editSets, setEditSets] = useState(() =>
    posterSets.map((set) => ({
      ...set,
      previousDateUpdated: set.date_updated,
    }))
  );

  const [refreshedSets, setRefreshedSets] = useState<SetRef[]>([]);
  const [refreshedIncludedItems, setRefreshedIncludedItems] = useState<{ [tmdb_id: string]: IncludedItem }>({});

  // State to track Modal visibility
  const [isEditModalOpen, setIsEditModalOpen] = useState(false);
  const [isDeleteModalOpen, setIsDeleteModalOpen] = useState(false);

  // State to track any error messages during updates.
  const [updateError, setUpdateError] = useState<APIResponse<unknown> | null>(null);

  // State to prevent multiple simultaneous operations.
  const [isMounted, setIsMounted] = useState(false);

  // Access global stores
  const { setMediaItem } = useMediaStore();

  // State to track if we are currently refreshing poster sets
  const [isRefreshing, setIsRefreshing] = useState(false);

  // Check if all sets are marked for deletion
  const allToDelete = editSets.every((set) => set.to_delete);

  // Compute last-downloaded safely
  const lastDownloadedLabel = useMemo(() => {
    if (!posterSets.length) return "Never";
    const latest = posterSets.reduce<number>((max, ps) => {
      const t = ps.last_downloaded ? new Date(ps.last_downloaded).getTime() : 0;
      return Math.max(max, Number.isFinite(t) ? t : 0);
    }, 0);
    if (!latest) return "Never";
    const latestDate = new Date(latest);
    return `${latestDate.toLocaleDateString("en-US")} at ${latestDate.toLocaleTimeString("en-US", {
      hour: "numeric",
      minute: "numeric",
      second: "numeric",
      hour12: true,
    })}`;
  }, [posterSets]);

  return (
    <>
      <TableRow
        className={cn(
          "group hover:bg-muted/50 cursor-pointer relative",
          bulkEditMode && "cursor-default",
          isRefreshing && "cursor-wait",
          bulkEditSelectedItems.has(
            `${normalizedSavedSet.media_item.tmdb_id}|||${normalizedSavedSet.media_item.library_title}`
          ) && "border-2 border-primary"
        )}
        key={savedSet.media_item.tmdb_id}
      >
        <TableCell>
          {bulkEditMode && (
            <Checkbox
              checked={bulkEditSelectedItems.has(
                `${savedSet.media_item.tmdb_id}|||${savedSet.media_item.library_title}`
              )}
              onCheckedChange={() => {
                const key = `${savedSet.media_item.tmdb_id}|||${savedSet.media_item.library_title}`;
                setBulkEditSelectedItems((prev) => {
                  const newSet = new Set(prev);
                  if (newSet.has(key)) {
                    newSet.delete(key);
                  } else {
                    newSet.add(key);
                  }
                  return newSet;
                });
              }}
              className="cursor-pointer border-1 border-primary mr-2"
              aria-label={`Select ${savedSet.media_item.title} for bulk edit`}
            />
          )}
        </TableCell>
        <TableCell>
          <div>
            {posterSets.some((set) => set.auto_download) ? (
              <Popover>
                <PopoverTrigger asChild>
                  <RefreshCcw className="text-green-500 cursor-help" size={24} />
                </PopoverTrigger>
                <PopoverContent className="p-2 max-w-xs">
                  <p className="text-sm">
                    Auto Download is enabled for this item. It will be periodically checked for new updates based on
                    your Auto Download settings.
                  </p>
                </PopoverContent>
              </Popover>
            ) : (
              <Popover>
                <PopoverTrigger asChild>
                  <RefreshCwOff className="text-red-500 cursor-help" size={24} />
                </PopoverTrigger>
                <PopoverContent className="p-2 max-w-xs">
                  <p className="text-sm">
                    Auto Download is disabled for this item. Click the edit button to enable it on one or more poster
                    sets.
                  </p>
                </PopoverContent>
              </Popover>
            )}
            {hasSelectedTypesOverlapOnAutoDownload(posterSets) && (
              <Popover>
                <PopoverTrigger asChild>
                  <AlertTriangle className="text-yellow-500 cursor-help" size={24} />
                </PopoverTrigger>
                <PopoverContent className="p-2 max-w-xs">
                  <p className="text-sm">
                    Some poster sets have overlapping selected types with Auto Download enabled. This may cause
                    unexpected behavior.
                  </p>
                </PopoverContent>
              </Popover>
            )}
          </div>
        </TableCell>
        <TableCell className="font-medium">
          {
            <AssetImage
              image={savedSet.media_item}
              imageType="item"
              className="w-[100%] h-auto transition-transform hover:scale-105"
            />
          }
        </TableCell>
        <TableCell className="font-medium">
          {
            <Link
              //href={formatMediaItemUrl(savedSet.MediaItem)}
              href={"/media-item/"}
              className="text-primary hover:underline"
              onClick={() => {
                setMediaItem(savedSet.media_item);
              }}
            >
              {savedSet.media_item.title}
            </Link>
          }
        </TableCell>
        <TableCell>{savedSet.media_item.year}</TableCell>
        <TableCell>{savedSet.media_item.library_title}</TableCell>
        <TableCell>{lastDownloadedLabel}</TableCell>
        <TableCell>
          <SavedSetsList savedSet={normalizedSavedSet} layout="table" />
        </TableCell>
        <TableCell>
          {posterSets.some(
            (set) =>
              set.selected_types.poster ||
              set.selected_types.backdrop ||
              set.selected_types.season_poster ||
              set.selected_types.titlecard
          ) ? (
            <div className="flex flex-wrap gap-2">{renderTypeBadges(normalizedSavedSet)}</div>
          ) : (
            <div className="flex flex-wrap gap-2">
              <Badge key={"no-types"} className="text-sm bg-red-500">
                No Selected Types
              </Badge>
            </div>
          )}
        </TableCell>

        {isRefreshing && (
          <div className="absolute inset-0 flex items-center justify-center bg-black/50 z-10">
            <Loader className="animate-spin h-8 w-8 text-primary" />
            <span className="ml-2 text-white">Refreshing sets...</span>
          </div>
        )}

        <TableCell className="text-right">
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="ghost" className="cursor-pointer p-1 hover:bg-muted/50 focus:bg-muted/50" size="icon">
                <MoreHorizontal />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem
                className="cursor-pointer"
                onClick={async () => {
                  await refreshPosterSet({
                    editSets,
                    setEditSets,
                    savedSet: normalizedSavedSet,
                    setIsRefreshing,
                    setUpdateError,
                    setRefreshedSets,
                    setRefreshedIncludedItems,
                  });
                  setIsEditModalOpen(true);
                }}
              >
                <Edit className="ml-2" />
                {isRefreshing ? "Refreshing..." : "Edit"}
              </DropdownMenuItem>

              {posterSets.some((set) => set.auto_download || normalizedSavedSet.media_item.type === "movie") && (
                <DropdownMenuItem
                  className="cursor-pointer"
                  onClick={() => {
                    handleRecheckItem(normalizedSavedSet.media_item.title, normalizedSavedSet);
                  }}
                >
                  <RefreshCcw className="ml-2" />
                  Force Autodownload Recheck
                </DropdownMenuItem>
              )}
              <DropdownMenuItem
                onClick={() => {
                  handleRedownloadItem(normalizedSavedSet);
                  toast.success("Redownload triggered", {
                    description: "The item has been queued for redownload. Check the download queue page for progress.",
                  });
                }}
                className="cursor-pointer"
              >
                <DownloadCloudIcon className="ml-2" />
                Redownload
              </DropdownMenuItem>
              <DropdownMenuItem onClick={() => setIsDeleteModalOpen(true)} className="text-destructive cursor-pointer">
                <Delete className="ml-2" />
                Delete
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </TableCell>
      </TableRow>

      {/* Edit Modal */}
      <SavedSetEditModal
        open={isEditModalOpen}
        onClose={() =>
          onCloseSavedSetsEditDeleteModals({
            setIsEditModalOpen,
            setIsDeleteModalOpen,
            setUpdateError,
            setIsMounted,
          })
        }
        editSets={editSets}
        setEditSets={setEditSets}
        savedSet={normalizedSavedSet}
        allToDelete={allToDelete}
        updateError={updateError}
        refreshedSets={refreshedSets}
        refreshedIncludedItems={refreshedIncludedItems}
        confirmEdit={() =>
          savedSetsConfirmEdit({
            editSets,
            savedSet: normalizedSavedSet,
            onUpdate,
            isMounted,
            setIsMounted,
            setUpdateError,
            setIsEditModalOpen,
            setIsDeleteModalOpen,
            allToDelete,
          })
        }
      />

      {/* Delete Modal */}
      <SavedSetDeleteModal
        open={isDeleteModalOpen}
        onClose={() =>
          onCloseSavedSetsEditDeleteModals({
            setIsEditModalOpen,
            setIsDeleteModalOpen,
            setUpdateError,
            setIsMounted,
          })
        }
        title={normalizedSavedSet.media_item.title}
        confirmDelete={() =>
          savedSetsConfirmDelete({
            savedSet: normalizedSavedSet,
            onUpdate,
            isMounted,
            setIsMounted,
            setUpdateError,
            setIsDeleteModalOpen,
          })
        }
      />
    </>
  );
};

export default SavedSetsTableRow;
