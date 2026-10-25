"use client";

import type { MediaServerSync } from "@/services/mediaserver/sync";
import { Loader2 } from "lucide-react";

import React, { useState } from "react";

import { PopoverHelp } from "@/components/shared/popover-help";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Label } from "@/components/ui/label";
import { Progress } from "@/components/ui/progress";

interface MediaServerSyncPanelProps {
  serverName: string;
  sync: MediaServerSync | null; // The latest sync of this media server
  disabledReason?: string; // Why the saved sets can't be applied right now
  onStart: () => Promise<void>;
  onStop: () => Promise<void>;
}

const plural = (count: number, one: string, many: string) => `${count.toLocaleString()} ${count === 1 ? one : many}`;

// Applies the saved sets to a media server that isn't the main one, and shows the progress
export const MediaServerSyncPanel: React.FC<MediaServerSyncPanelProps> = ({
  serverName,
  sync,
  disabledReason,
  onStart,
  onStop,
}) => {
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [starting, setStarting] = useState(false);
  const [stopping, setStopping] = useState(false);
  const running = !!sync?.running;

  const handleStart = async () => {
    setStarting(true);
    await onStart();
    setStarting(false);
    setConfirmOpen(false);
  };

  const handleStop = async () => {
    setStopping(true);
    await onStop();
    setStopping(false);
  };

  return (
    <div className="space-y-2 rounded-md border border-muted p-3">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div className="flex items-center gap-2">
          <Label>Saved Sets</Label>
          <PopoverHelp ariaLabel="help-media-server-sync">
            <p className="mb-2">Applies the images of every saved set to the same items on {serverName}.</p>
            <p className="text-muted-foreground">
              New images are applied to {serverName} on their own. Use this to catch up on sets saved before{" "}
              {serverName} was added, or after it was turned off.
            </p>
          </PopoverHelp>
        </div>
        {running ? (
          <Button variant="outline" size="sm" onClick={handleStop} disabled={stopping} className="cursor-pointer">
            {stopping && <Loader2 className="h-4 w-4 animate-spin" />}
            Stop
          </Button>
        ) : (
          <Button
            variant="outline"
            size="sm"
            onClick={() => setConfirmOpen(true)}
            disabled={!!disabledReason}
            title={disabledReason}
            className="cursor-pointer"
          >
            Apply Saved Sets
          </Button>
        )}
      </div>

      {running && sync && (
        <div className="space-y-1">
          <Progress value={sync.total > 0 ? (sync.done / sync.total) * 100 : 0} />
          <p className="text-xs text-muted-foreground">
            {sync.total > 0
              ? `${sync.done.toLocaleString()} of ${plural(sync.total, "saved item", "saved items")} checked`
              : "Loading the libraries and saved sets..."}
            {sync.applied > 0 && ` · ${plural(sync.applied, "image", "images")} applied`}
          </p>
          {sync.current && <p className="truncate text-xs text-muted-foreground">{sync.current}</p>}
        </div>
      )}

      {!running && sync && (
        <div className="space-y-1 text-xs">
          {sync.error ? (
            <p className="text-red-500">Stopped: {sync.error}</p>
          ) : (
            <p className="text-muted-foreground">
              {sync.stopped ? "Stopped. " : "Done. "}
              {plural(sync.applied, "image", "images")} applied to {serverName}
              {sync.not_found > 0 && `, ${plural(sync.not_found, "saved item isn't", "saved items aren't")} on it`}.
            </p>
          )}
          {sync.failed > 0 && (
            <details className="text-amber-500">
              <summary className="cursor-pointer">{plural(sync.failed, "image", "images")} couldn't be applied</summary>
              <ul className="mt-1 space-y-0.5 text-muted-foreground">
                {sync.failures.map((failure, index) => (
                  <li key={index} className="break-words">
                    {failure}
                  </li>
                ))}
                {sync.failed > sync.failures.length && (
                  <li>and {sync.failed - sync.failures.length} more in the log</li>
                )}
              </ul>
            </details>
          )}
        </div>
      )}

      <Dialog open={confirmOpen} onOpenChange={(open) => !starting && setConfirmOpen(open)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Apply your saved sets to {serverName}?</DialogTitle>
            <DialogDescription>
              The images of every saved set are applied to the same items on {serverName}, replacing the images it has
              now. It runs in the background and can take a while for a large library. You can stop it at any time.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button
              variant="ghost"
              onClick={() => setConfirmOpen(false)}
              disabled={starting}
              className="cursor-pointer"
            >
              Cancel
            </Button>
            <Button onClick={handleStart} disabled={starting} className="cursor-pointer">
              {starting && <Loader2 className="h-4 w-4 animate-spin" />}
              Apply Saved Sets
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
};
