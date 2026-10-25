"use client";

import React from "react";

import { Progress } from "@/components/ui/progress";

import { cn } from "@/lib/cn";

interface DownloadProgressBarProps {
  value: number; // 0 to 100
  label?: string; // What is downloading, shown in the bar until it is done
  shortLabel?: string; // Shown instead of the label on narrow screens
  hasErrors?: boolean; // Whether any image failed
}

// The progress of a download, used by the Download Modal and the Download Queue page
export const DownloadProgressBar: React.FC<DownloadProgressBarProps> = ({
  value,
  label,
  shortLabel,
  hasErrors = false,
}) => {
  const done = value >= 100;

  return (
    <div className="flex w-full items-center justify-between">
      <div className="relative w-full min-w-0">
        <Progress
          value={value}
          className={cn(
            "w-full overflow-hidden rounded-md h-3",
            !done && "animate-pulse h-5",
            !done && hasErrors && "[&>div]:bg-amber-500",
            done && !hasErrors && "[&>div]:bg-green-500",
            done && hasErrors && "[&>div]:bg-destructive"
          )}
        />

        {!done && label && (
          <span
            className="pointer-events-none absolute inset-0 mt-0.5 flex min-w-0 items-center justify-center px-2 text-xs text-white"
            title={label}
          >
            <span className="hidden w-full min-w-0 truncate text-center sm:block">{label}</span>
            <span className="w-full min-w-0 truncate text-center sm:hidden">{shortLabel || label}</span>
          </span>
        )}
      </div>

      <span className="ml-1 min-w-[30px] text-right text-sm text-muted-foreground">{Math.round(value)}%</span>
    </div>
  );
};
