"use client";

import { formatExactDateTime } from "@/helper/format-date-last-updates";
import { makePlural } from "@/helper/make_plural";
import { RemoveCollectionFromQueue } from "@/services/downloads/collection-queue-remove";
import { RetryCollectionInQueue } from "@/services/downloads/collection-queue-retry";
import { AlertTriangle, RefreshCcw, Trash2 } from "lucide-react";
import { toast } from "sonner";

import React from "react";

import { AssetImage } from "@/components/shared/asset-image";
import { ConfirmDestructiveDialogActionButton } from "@/components/shared/dialog-destructive-action";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader } from "@/components/ui/card";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Separator } from "@/components/ui/separator";
import { H4 } from "@/components/ui/typography";

import { cn } from "@/lib/cn";

import type { CollectionQueueItem } from "@/types/database/db-collection-queue";

const CollectionQueueEntry: React.FC<{
  entry: CollectionQueueItem;
  fetchEntries?: () => Promise<void>;
  // When true (Error/Warning sections), show why the entry failed via a popover.
  showFailureDetails?: boolean;
  // When true (Error section), show a Retry button to re-queue the entry.
  showRetry?: boolean;
}> = ({ entry, fetchEntries, showFailureDetails = false, showRetry = false }) => {
  const images = Array.isArray(entry.images) ? entry.images : [];
  const hasPoster = images.some((img) => img.type === "collection_poster");
  const hasBackdrop = images.some((img) => img.type === "collection_backdrop");

  // Prefer the poster for the preview thumbnail, falling back to the backdrop.
  const previewImage = images.find((img) => img.type === "collection_poster") ?? images[0];

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

  const refresh = async () => {
    if (fetchEntries) await fetchEntries();
  };

  const onDeleteConfirm = async () => {
    try {
      const response = await RemoveCollectionFromQueue(entry);
      if (response.status === "error") {
        toast.error(`Error deleting from queue: ${response.error?.message || "Unknown error occurred."}`);
      } else {
        toast.success(response.data?.result || "Successfully deleted from queue.");
      }
    } catch (error) {
      toast.error(`Error deleting from queue: ${error instanceof Error ? error.message : "Unknown error occurred."}`);
    }
    await refresh();
  };

  const onRetry = async () => {
    try {
      const response = await RetryCollectionInQueue(entry);
      if (response.status === "error") {
        toast.error(`Error retrying: ${response.error?.message || "Unknown error occurred."}`);
      } else {
        toast.success(response.data?.result || "Re-queued for download.");
      }
    } catch (error) {
      toast.error(`Error retrying: ${error instanceof Error ? error.message : "Unknown error occurred."}`);
    }
    await refresh();
  };

  return (
    <Card className="relative w-full max-w-md mx-auto transition-shadow">
      <CardHeader>
        {/* Top Left: Delete */}
        <div className="absolute top-2 left-2">
          <ConfirmDestructiveDialogActionButton
            variant="outline"
            className="text-destructive border-1 shadow-none hover:text-red-500 cursor-pointer"
            confirmText="Delete Entry"
            title="Delete Queue Entry?"
            description="Are you sure you want to delete this collection entry from the download queue? This action cannot be undone."
            onConfirm={onDeleteConfirm}
          >
            <Trash2 className="w-5 h-5" />
          </ConfirmDestructiveDialogActionButton>
        </div>
        {/* Top Right: Retry (error section) */}
        {showRetry && (
          <div className="absolute top-2 right-2">
            <Button
              variant="outline"
              size="icon"
              onClick={onRetry}
              aria-label="Retry collection download"
              className="border-1 shadow-none cursor-pointer hover:text-primary"
            >
              <RefreshCcw className="w-5 h-5" />
            </Button>
          </div>
        )}
      </CardHeader>

      {/* Middle: Image */}
      {previewImage && (
        <div className="flex justify-center">
          <AssetImage
            image={previewImage}
            imageType="mediux"
            aspect={previewImage.type === "collection_backdrop" ? "backdrop" : "poster"}
            className="w-[80%] h-auto transition-transform hover:scale-105"
          />
        </div>
      )}

      {/* Content */}
      <CardContent className="p-0 ml-2 mr-2">
        <H4 className="text-primary">{entry.collection_item.title}</H4>

        <span className="text-xs sm:text-sm text-muted-foreground inline-block">
          {entry.collection_item.library_title}
        </span>

        <Separator className="my-4" />

        {hasPoster || hasBackdrop ? (
          <div className="flex flex-wrap gap-2">
            {hasPoster && (
              <Badge variant="outline" className="text-sm">
                Poster
              </Badge>
            )}
            {hasBackdrop && (
              <Badge variant="outline" className="text-sm">
                Backdrop
              </Badge>
            )}
          </div>
        ) : (
          <div className="flex flex-wrap gap-2">
            <Badge variant="outline" className="text-sm bg-red-500">
              No Selected Types
            </Badge>
          </div>
        )}

        {/* Failure reasons (Error/Warning sections). */}
        {hasFailureDetails && (
          <>
            <Separator className="my-4" />
            <Popover>
              <PopoverTrigger asChild>
                <Button
                  variant="outline"
                  size="sm"
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
              <PopoverContent align="center" className="max-h-80 w-80 overflow-y-auto text-sm">
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

export default CollectionQueueEntry;
