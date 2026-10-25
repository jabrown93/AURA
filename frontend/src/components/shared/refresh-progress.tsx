"use client";

import { Loader2 } from "lucide-react";

import React, { useEffect, useState } from "react";

import { DownloadProgressBar } from "@/components/shared/download-progress-bar";

import { cn } from "@/lib/cn";

interface RefreshProgressProps {
  title: string; // What is being refreshed
  percent?: number; // 0 to 100, or a busy bar without it
  label?: string; // Shown in the bar
  shortLabel?: string; // Shown in the bar on narrow screens
  detail?: string; // Shown under the bar
  className?: string;
}

// A refresh quicker than this finishes before the progress shows, so it doesn't flash
const SHOW_AFTER_MS = 400;

// Shows a refresh's progress, like the progress in the Download Modal and the Download Queue
export const RefreshProgress: React.FC<RefreshProgressProps> = ({
  title,
  percent,
  label,
  shortLabel,
  detail,
  className,
}) => {
  const [show, setShow] = useState(false);
  useEffect(() => {
    const timer = setTimeout(() => setShow(true), SHOW_AFTER_MS);
    return () => clearTimeout(timer);
  }, []);
  if (!show) return null;

  return (
    <div className={cn("w-full space-y-2 rounded-lg border p-4 mb-4", className)}>
      <p className="flex min-w-0 items-center gap-2 text-sm font-medium">
        <Loader2 className="h-4 w-4 shrink-0 animate-spin" />
        <span className="min-w-0">{title}</span>
      </p>

      {percent === undefined ? (
        <div className="relative h-5 w-full overflow-hidden rounded-md bg-primary/20">
          <div className="absolute inset-y-0 w-1/3 animate-refresh-progress rounded-md bg-primary" />
          {label && (
            <span className="pointer-events-none absolute inset-0 mt-0.5 flex items-center justify-center px-2 text-xs text-white">
              <span className="w-full min-w-0 truncate text-center">{label}</span>
            </span>
          )}
        </div>
      ) : (
        <DownloadProgressBar value={percent} label={label} shortLabel={shortLabel} />
      )}

      {detail && <p className="text-xs text-muted-foreground">{detail}</p>}
    </div>
  );
};
