"use client";

import { ReturnErrorMessage } from "@/services/api-error-return";
import { RateMediaItem } from "@/services/mediaserver/media-item-rate";
import { Star } from "lucide-react";
import { toast } from "sonner";

import { useEffect, useMemo, useState } from "react";

import { ErrorMessage } from "@/components/shared/error-message";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog";

import { cn } from "@/lib/cn";
import { useOnboardingStore } from "@/lib/stores/global-store-onboarding";

import type { APIResponse } from "@/types/api/api-response";
import type { MediaItem } from "@/types/media-and-posters/media-item-and-library";

export type MediaItemRatingModalProps = {
  mediaItem: MediaItem;
  isOpen: boolean;
  onClose: () => void;
};

// Clamp the rating to 0-5 stars, rounded to the nearest step (0.5 for half stars, 1 for whole stars)
const clampToStep = (value: number, step: number) => {
  const clamped = Math.max(0, Math.min(5, value));
  return Math.round(clamped / step) * step;
};

const getInitialRatingFromMediaItem = (mediaItem: MediaItem) => {
  const userGuid = mediaItem.guids?.find((g) => g.provider === "user" && g.rating);
  if (!userGuid?.rating) return 0;

  const parsed = parseFloat(userGuid.rating);
  if (!Number.isFinite(parsed)) return 0;

  // User ratings are on a scale of 10; modal uses 0-5 stars.
  return parsed / 2;
};

function StarVisual({ fill }: { fill: 0 | 0.5 | 1 }) {
  return (
    <span className="relative inline-flex h-7 w-7">
      <Star className="h-7 w-7 text-muted-foreground" strokeWidth={2} />
      {fill === 1 && <Star className="absolute inset-0 h-7 w-7 text-yellow-500 fill-yellow-500" strokeWidth={2} />}
      {fill === 0.5 && (
        <Star
          className={cn("absolute inset-0 h-7 w-7 text-yellow-500 fill-yellow-500", "[clip-path:inset(0_50%_0_0)]")}
          strokeWidth={2}
        />
      )}
    </span>
  );
}

export function MediaItemRatingModal({ mediaItem, isOpen, onClose }: MediaItemRatingModalProps) {
  // Silo only supports whole star ratings
  const mediaServerType = useOnboardingStore((state) => state.status?.current_setup?.media_server?.type);
  const ratingStep = mediaServerType === "Silo" ? 1 : 0.5;

  const [initialRating, setInitialRating] = useState(0);
  const [rating, setRating] = useState<number>(0);
  const [hoverRating, setHoverRating] = useState<number | null>(null);
  const [hoverEnabled, setHoverEnabled] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<APIResponse<unknown> | null>(null);

  useEffect(() => {
    if (!isOpen) return;
    const start = clampToStep(getInitialRatingFromMediaItem(mediaItem), ratingStep);
    setInitialRating(start);
    setRating(start);
    setHoverRating(null);
    setHoverEnabled(false);
    setSaving(false);
    setError(null);
  }, [isOpen, mediaItem, ratingStep]);

  const displayRating = hoverRating !== null && hoverRating > 0.5 ? hoverRating : rating || 0;

  const starFills = useMemo(() => {
    return Array.from({ length: 5 }, (_, i) => {
      const starIndex = i + 1;
      const diff = displayRating - (starIndex - 1);
      if (diff >= 1) return 1 as const;
      if (diff >= 0.5) return 0.5 as const;
      return 0 as const;
    });
  }, [displayRating]);

  const handleSave = async () => {
    try {
      setSaving(true);
      const response = await RateMediaItem(mediaItem, rating);
      if (response.status === "error") {
        throw new Error(response.error?.message || "Unknown error rating media item");
      }
      const ratingDisplay = rating % 1 === 0 ? rating.toFixed(0) : rating.toFixed(1);
      toast.success(`Rated '${mediaItem.title}' ${ratingDisplay} / 5`);
      onClose();
    } catch (error) {
      setError(ReturnErrorMessage<unknown>(error));
    } finally {
      setSaving(false);
    }
  };

  return (
    <Dialog
      open={isOpen}
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
    >
      <DialogContent className={cn("max-h-[90vh] overflow-y-auto", "border border-primary/40 shadow-2xl")}>
        <DialogHeader>
          <DialogTitle className="text-lg font-bold">Rate</DialogTitle>
          <DialogDescription className="text-sm text-muted-foreground">
            Choose a rating for{" "}
            <span className="font-semibold text-foreground">
              '{mediaItem.title} ({mediaItem.year})'
            </span>
          </DialogDescription>
        </DialogHeader>

        <div className="mt-2 rounded-lg border border-border/60 bg-card/70 p-4">
          <div className="flex items-center justify-between">
            <div
              className="flex items-center rounded-md cursor-pointer select-none"
              onMouseMove={() => {
                if (!hoverEnabled) {
                  setHoverEnabled(true);
                }
              }}
              onMouseLeave={() => setHoverRating(null)}
              aria-label="Rating"
            >
              {Array.from({ length: 5 }, (_, i) => {
                const starNumber = i + 1;

                return (
                  <span key={starNumber} className="relative inline-flex">
                    <button
                      type="button"
                      className="absolute inset-y-0 left-0 w-1/2 z-10"
                      aria-label={`${clampToStep(starNumber - 0.5, ratingStep)} stars`}
                      onMouseEnter={() => {
                        if (!hoverEnabled) return;
                        setHoverRating(clampToStep(starNumber - 0.5, ratingStep));
                      }}
                      onFocus={() => setHoverRating(clampToStep(starNumber - 0.5, ratingStep))}
                      onClick={() => setRating(clampToStep(starNumber - 0.5, ratingStep))}
                    />
                    <button
                      type="button"
                      className="absolute inset-y-0 right-0 w-1/2 z-10"
                      aria-label={`${starNumber} stars`}
                      onMouseEnter={() => {
                        if (!hoverEnabled) return;
                        setHoverRating(starNumber);
                      }}
                      onFocus={() => setHoverRating(starNumber)}
                      onClick={() => setRating(starNumber)}
                    />

                    <span className="p-0.5">
                      <StarVisual fill={starFills[i]} />
                    </span>
                  </span>
                );
              })}
            </div>

            <label className="rounded-md border border-border/60 bg-background text-sm text-muted-foreground tabular-nums flex items-center">
              <input
                type="number"
                min={0}
                max={5}
                step={ratingStep}
                value={rating}
                onChange={(e) => {
                  const parsed = parseFloat(e.target.value);
                  setHoverRating(null);
                  if (!Number.isFinite(parsed)) {
                    setRating(0);
                    return;
                  }
                  setRating(clampToStep(parsed, ratingStep));
                }}
                className="w-10 bg-transparent text-right font-semibold text-foreground outline-none [appearance:textfield] [&::-webkit-inner-spin-button]:appearance-none [&::-webkit-outer-spin-button]:appearance-none"
                disabled={saving}
                aria-label="Rating input"
              />
              <span> / {ratingStep === 1 ? "5" : "5.0"} </span>
            </label>
          </div>

          <div className="mt-4 flex items-center gap-2">
            <Button
              type="button"
              variant="outline"
              onClick={() => {
                setHoverRating(null);
                setRating(initialRating);
              }}
              disabled={saving || rating === initialRating}
            >
              Reset
            </Button>

            <Button
              type="button"
              variant="outline"
              onClick={() => {
                setHoverRating(null);
                setRating((r) => clampToStep(r - ratingStep, ratingStep));
              }}
              disabled={saving || rating <= 0}
            >
              -
            </Button>

            <Button
              type="button"
              variant="outline"
              onClick={() => {
                setHoverRating(null);
                setRating((r) => clampToStep(r + ratingStep, ratingStep));
              }}
              disabled={saving || rating >= 5}
            >
              +
            </Button>
          </div>
        </div>

        <div className="mt-6 flex justify-end gap-2">
          <Button type="button" variant="outline" onClick={onClose} disabled={saving}>
            Cancel
          </Button>
          <Button type="button" onClick={handleSave} disabled={saving || rating === initialRating}>
            {saving ? "Saving..." : "Save"}
          </Button>
        </div>

        {error && <ErrorMessage error={error} />}
      </DialogContent>
    </Dialog>
  );
}
