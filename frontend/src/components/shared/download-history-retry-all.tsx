"use client";

import { RetryDownloadHistoryEntry } from "@/services/downloads/history-retry";
import { toast } from "sonner";

import { useState } from "react";

import { Button } from "@/components/ui/button";

import type { DownloadHistoryEntry } from "@/types/database/download-queue";

// Fork: bulk retry for the Failed tab, on top of upstream's per-entry retry.
export function DownloadHistoryRetryAll({ entries, onDone }: { entries: DownloadHistoryEntry[]; onDone: () => void }) {
  const [retrying, setRetrying] = useState(false);

  const retryAll = async () => {
    setRetrying(true);
    let failed = 0;
    // One at a time, so the queue worker isn't flooded and errors stay attributable
    for (const entry of entries) {
      const response = await RetryDownloadHistoryEntry(entry.id);
      if (response.status === "error") failed++;
    }
    setRetrying(false);
    if (failed > 0) {
      toast.error(`${failed} of ${entries.length} failed downloads could not be retried`);
    } else {
      toast.success(`Retrying ${entries.length} failed downloads`);
    }
    onDone();
  };

  return (
    <Button
      variant="outline"
      className="cursor-pointer"
      onClick={() => void retryAll()}
      disabled={retrying || entries.length === 0}
    >
      {retrying ? "Retrying…" : "Retry All"}
    </Button>
  );
}
