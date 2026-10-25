"use client";

import { formatDownloadProgressShortText, formatDownloadProgressText } from "@/helper/format-download-progress";
import { Loader2 } from "lucide-react";

import React from "react";

import { DownloadProgressBar } from "@/components/shared/download-progress-bar";

import type { DownloadQueueProgress } from "@/types/database/download-queue";

interface DownloadQueueProgressPanelProps {
  progress: DownloadQueueProgress | null; // The image being downloaded
  processingTitle?: string; // Title of the job being downloaded, shown before its first image starts
  waiting: number; // Jobs waiting after this one
}

const plural = (count: number, one: string, many: string) => `${count.toLocaleString()} ${count === 1 ? one : many}`;

// Shows the progress of the job being downloaded, like the progress in the Download Modal
export const DownloadQueueProgressPanel: React.FC<DownloadQueueProgressPanelProps> = ({
  progress,
  processingTitle,
  waiting,
}) => {
  const title = progress?.media_item_title || processingTitle;
  if (!title) return null;

  const total = progress?.total ?? 0;
  const done = progress?.done ?? 0;
  const percent = total > 0 ? Math.round((done / total) * 100) : 0;
  // The image being downloaded, counted from 1
  const label =
    progress && total > 0 ? formatDownloadProgressText(progress, title, Math.min(done + 1, total), total) : undefined;

  return (
    <div className="w-full space-y-2 rounded-lg border p-4 mb-4">
      <div className="flex items-center justify-between gap-3">
        <p className="flex min-w-0 items-center gap-2 text-sm font-medium">
          <Loader2 className="h-4 w-4 shrink-0 animate-spin" />
          <span className="truncate">Downloading {title}</span>
        </p>
        {waiting > 0 && (
          <span className="shrink-0 text-xs text-muted-foreground">{plural(waiting, "job", "jobs")} waiting</span>
        )}
      </div>

      <DownloadProgressBar
        value={percent}
        label={label}
        shortLabel={progress && total > 0 ? formatDownloadProgressShortText(progress) : undefined}
        hasErrors={!!progress && progress.failed > 0}
      />

      <p className="text-xs text-muted-foreground">
        {total > 0 ? `${done.toLocaleString()} of ${plural(total, "image", "images")}` : "Getting ready..."}
        {progress && progress.failed > 0 && (
          <span className="text-amber-500"> · {plural(progress.failed, "image", "images")} failed</span>
        )}
      </p>
    </div>
  );
};
