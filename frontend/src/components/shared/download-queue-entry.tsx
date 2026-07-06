"use client";

import { formatExactDateTime } from "@/helper/format-date-last-updates";
import { makePlural } from "@/helper/make_plural";
import { RemoveItemFromQueue } from "@/services/downloads/queue-remove";
import { AlertTriangle, Trash2 } from "lucide-react";
import { toast } from "sonner";

import React from "react";

import Link from "next/link";

import { AssetImage } from "@/components/shared/asset-image";
import { ConfirmDestructiveDialogActionButton } from "@/components/shared/dialog-destructive-action";
import type { FormItemDisplay } from "@/components/shared/download-modal";
import DownloadModal from "@/components/shared/download-modal";
import { renderTypeBadges } from "@/components/shared/saved-sets-shared";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader } from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Separator } from "@/components/ui/separator";
import { H4 } from "@/components/ui/typography";

import { cn } from "@/lib/cn";
import { useMediaStore } from "@/lib/stores/global-store-media-store";

import type { DBSavedItem } from "@/types/database/db-poster-set";
import type { BaseSetInfo } from "@/types/media-and-posters/sets";

const DownloadQueueEntry: React.FC<{
  entry: DBSavedItem;
  fetchQueueEntries?: () => Promise<void>;
  // Bulk-selection support (used by the download queue Error section).
  selectable?: boolean;
  selected?: boolean;
  onToggleSelected?: (checked: boolean) => void;
  // When true (Error/Warning sections), show why the entry failed via a popover.
  showFailureDetails?: boolean;
}> = ({
  entry,
  fetchQueueEntries,
  selectable = false,
  selected = false,
  onToggleSelected,
  showFailureDetails = false,
}) => {
  const posterSets = Array.isArray(entry.poster_sets) ? entry.poster_sets : [];

  // Failure reasons persisted by the backend when the entry was moved to the
  // error/warning state. Only surfaced in the Error/Warning sections.
  const queueErrors = Array.isArray(entry.queue_errors) ? entry.queue_errors : [];
  const queueWarnings = Array.isArray(entry.queue_warnings) ? entry.queue_warnings : [];
  const hasFailureDetails = showFailureDetails && (queueErrors.length > 0 || queueWarnings.length > 0);
  const failureLabelParts: string[] = [];
  if (queueErrors.length > 0) {
    failureLabelParts.push(`${queueErrors.length} ${makePlural(queueErrors.length, "error")}`);
  }
  if (queueWarnings.length > 0) {
    failureLabelParts.push(`${queueWarnings.length} ${makePlural(queueWarnings.length, "warning")}`);
  }
  const baseSetInfo: BaseSetInfo = {
    id: posterSets[0]?.id || "",
    title: posterSets[0]?.title || "",
    type: posterSets[0]?.type || "movie",
    user_created: posterSets[0]?.user_created || "",
    date_created: posterSets[0]?.date_created || "",
    date_updated: posterSets[0]?.date_updated || "",
    popularity: posterSets[0]?.popularity || 0,
    popularity_global: posterSets[0]?.popularity_global || 0,
  };

  const formItems: FormItemDisplay[] = posterSets.map((set) => ({
    MediaItem: entry.media_item,
    Set: set,
  }));

  // Access global stores
  const { setMediaItem } = useMediaStore();

  const onDeleteConfirm = async () => {
    const safeEntry = { ...entry, poster_sets: Array.isArray(entry.poster_sets) ? entry.poster_sets : [] };
    try {
      const response = await RemoveItemFromQueue(safeEntry);
      if (response.status === "error") {
        toast.error(
          `Error deleting from queue: ${response.error?.message || "Unknown error occurred trying to delete."}`
        );
      } else {
        toast.success(response.data?.result || "Successfully deleted from queue.");
      }
    } catch (error) {
      toast.error(
        `Error deleting from queue: ${error instanceof Error ? error.message : "Unknown error occurred trying to delete."}`
      );
    }
    if (fetchQueueEntries) {
      await fetchQueueEntries();
    }
  };

  return (
    <Card
      className={cn(
        "relative w-full max-w-md mx-auto transition-shadow",
        selectable && "cursor-pointer",
        selected && "ring-2 ring-primary"
      )}
      role={selectable ? "button" : undefined}
      tabIndex={selectable ? 0 : undefined}
      aria-pressed={selectable ? selected : undefined}
      onClick={selectable ? () => onToggleSelected?.(!selected) : undefined}
      onKeyDown={
        selectable
          ? (e) => {
              if (e.key === "Enter" || e.key === " ") {
                e.preventDefault();
                onToggleSelected?.(!selected);
              }
            }
          : undefined
      }
    >
      <CardHeader>
        {/* Top Left: Bulk-select checkbox (bulk mode) or Delete File (normal) */}
        <div className="absolute top-2 left-2">
          {selectable ? (
            <Checkbox
              checked={selected}
              onCheckedChange={(checked) => onToggleSelected?.(checked === true)}
              onClick={(e) => e.stopPropagation()}
              aria-label={`Select ${entry.media_item.title}`}
              className="h-6 w-6 border-1 border-primary cursor-pointer"
            />
          ) : (
            <ConfirmDestructiveDialogActionButton
              variant="outline"
              className="text-destructive border-1 shadow-none hover:text-red-500 cursor-pointer"
              confirmText="Delete File"
              title="Delete Downloaded File?"
              description="Are you sure you want to delete the downloaded file for this media item? This action cannot be undone."
              onConfirm={onDeleteConfirm}
            >
              <Trash2 className="w-5 h-5" />
            </ConfirmDestructiveDialogActionButton>
          )}
        </div>
        {/* Top Right: Dropdown Menu */}
        <div
          className="absolute top-2 right-2 justify-end"
          onClick={selectable ? (e) => e.stopPropagation() : undefined}
        >
          <DownloadModal baseSetInfo={baseSetInfo} formItems={formItems} />
        </div>
      </CardHeader>

      {/* Middle: Image */}
      <div className="flex justify-center">
        <AssetImage
          image={entry.media_item}
          imageType="item"
          className="w-[80%] h-auto transition-transform hover:scale-105"
        />
      </div>

      {/* Content */}
      <CardContent className="p-0 ml-2 mr-2">
        {/* Title */}
        <H4>
          <Link
            //href={formatMediaItemUrl(entry.MediaItem)}
            href={"/media-item/"}
            className="text-primary hover:underline"
            onClick={(e) => {
              if (selectable) e.stopPropagation();
              setMediaItem(entry.media_item);
            }}
          >
            {entry.media_item.title}
          </Link>
        </H4>

        {/* Year and Library */}
        <span className="text-xs sm:text-sm text-muted-foreground inline-block">
          {entry.media_item.year} · {entry.media_item.library_title}
        </span>

        {/* Set Author */}
        {entry.poster_sets && entry.poster_sets.length > 0 && (
          <span className="text-xs sm:text-sm text-muted-foreground inline-block">
            {`Set by: ${entry.poster_sets[0].user_created}`}
          </span>
        )}

        <Separator className="my-4" />

        {posterSets.some(
          (set) =>
            set.selected_types.poster ||
            set.selected_types.backdrop ||
            set.selected_types.season_poster ||
            set.selected_types.titlecard
        ) ? (
          <div className="flex flex-wrap gap-2">{renderTypeBadges(entry)}</div>
        ) : (
          <div className="flex flex-wrap gap-2">
            <Badge key={"no-types"} variant="outline" className="text-sm bg-red-500">
              No Selected Types
            </Badge>
          </div>
        )}

        {/* Failure reasons (Error/Warning sections): why this entry failed. */}
        {hasFailureDetails && (
          <>
            <Separator className="my-4" />
            <Popover>
              <PopoverTrigger asChild>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={selectable ? (e) => e.stopPropagation() : undefined}
                  onKeyDown={selectable ? (e) => e.stopPropagation() : undefined}
                  className={cn(
                    "w-full flex items-center justify-center gap-2 text-xs sm:text-sm cursor-pointer",
                    queueErrors.length > 0
                      ? "border-red-500 text-red-500 hover:text-red-600"
                      : "border-yellow-500 text-yellow-600 hover:text-yellow-700"
                  )}
                >
                  <AlertTriangle className="h-4 w-4" />
                  {`View ${failureLabelParts.join(", ")}`}
                </Button>
              </PopoverTrigger>
              <PopoverContent
                align="center"
                onClick={(e) => e.stopPropagation()}
                className="max-h-80 w-80 overflow-y-auto text-sm"
              >
                {entry.failed_at && (
                  <p className="mb-2 text-xs text-muted-foreground">Failed {formatExactDateTime(entry.failed_at)}</p>
                )}
                {queueErrors.length > 0 && (
                  <div className={queueWarnings.length > 0 ? "mb-3" : undefined}>
                    <p className="mb-1 font-semibold text-red-500">
                      {queueErrors.length} {makePlural(queueErrors.length, "Error")}
                    </p>
                    <ul className="list-disc space-y-1 pl-4">
                      {queueErrors.map((err, i) => (
                        <li key={`err-${i}`} className="break-words text-red-500/90">
                          {err}
                        </li>
                      ))}
                    </ul>
                  </div>
                )}
                {queueWarnings.length > 0 && (
                  <div>
                    <p className="mb-1 font-semibold text-yellow-600">
                      {queueWarnings.length} {makePlural(queueWarnings.length, "Warning")}
                    </p>
                    <ul className="list-disc space-y-1 pl-4">
                      {queueWarnings.map((warn, i) => (
                        <li key={`warn-${i}`} className="break-words text-yellow-700/90">
                          {warn}
                        </li>
                      ))}
                    </ul>
                  </div>
                )}
              </PopoverContent>
            </Popover>
          </>
        )}
      </CardContent>
    </Card>
  );
};

export default DownloadQueueEntry;
