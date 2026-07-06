"use client";

import { makePlural } from "@/helper/make_plural";
import { downloadImageFileForMediaItem } from "@/services/downloads/download-image";
import * as yaml from "js-yaml";
import { User } from "lucide-react";

import { useEffect, useState } from "react";

import Link from "next/link";

import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Progress } from "@/components/ui/progress";
import { Textarea } from "@/components/ui/textarea";

import { cn } from "@/lib/cn";

import type { PosterSet } from "@/types/database/db-poster-set";
import type { MediaItem } from "@/types/media-and-posters/media-item-and-library";
import type { ImageFile } from "@/types/media-and-posters/sets";
import { TYPE_DOWNLOAD_IMAGE_TYPE_OPTIONS } from "@/types/ui-options";

export type MediaItemManualImportModalProps = {
  mediaItem: MediaItem;
  isOpen: boolean;
  onClose: () => void;
};

type ImportTaskStatus = "pending" | "in-progress" | "completed" | "failed";
type ImportTask = {
  id: string;
  status: ImportTaskStatus;
  label: string;
  imageFile: ImageFile;
  error?: string;
};

type ImportProgress = {
  currentText: string;
  totalPlanned: number;
  tasks: ImportTask[];
};

const newId = () => globalThis.crypto?.randomUUID?.() ?? `${Date.now()}-${Math.random().toString(16).slice(2)}`;

const getRefreshCounts = (state: ImportProgress) => {
  const relevant = state.tasks;
  const done = relevant.filter((t) => t.status === "completed" || t.status === "failed").length;

  const total = Math.max(1, state.totalPlanned || relevant.length || 1);
  return { done, total };
};

const getImportPercentComplete = (progress: ImportProgress) => {
  const { done, total } = getRefreshCounts(progress);
  return Math.min(100, Math.round((done / total) * 100));
};

export function ManualImportModal({ mediaItem, isOpen, onClose }: MediaItemManualImportModalProps) {
  // States for YAML text input
  const [yamlText, setYamlText] = useState("");
  const [isValidYaml, setIsValidYaml] = useState(false);

  // States for PreImport
  const [setAuthor, setSetAuthor] = useState<string | null>(null);
  const [setId, setSetId] = useState<string | null>(null);
  const [validPosterSet, setValidPosterSet] = useState<PosterSet | null>(null);
  const [numberOfPostersInSet, setNumberOfPostersInSet] = useState<number | null>(null);
  const [numberOfBackdropsInSet, setNumberOfBackdropsInSet] = useState<number | null>(null);
  const [numberOfSeasonPostersInSet, setNumberOfSeasonPostersInSet] = useState<number | null>(null);
  const [numberOfTitlecardsInSet, setNumberOfTitlecardsInSet] = useState<number | null>(null);

  // States for Import Progress
  const [importProgress, setImportProgress] = useState<ImportProgress>({
    currentText: "",
    totalPlanned: 0,
    tasks: [],
  });

  // States for Error Handling
  const [error, setError] = useState<string | null>(null);

  const resetImportProgress = () => {
    setImportProgress({ currentText: "", totalPlanned: 0, tasks: [] });
  };

  useEffect(() => {
    // When closing, clear progress so the next open starts clean
    if (!isOpen) {
      resetImportProgress();
    }
  }, [isOpen]);

  const setImportText = (text: string) => {
    setImportProgress((prev) => ({ ...prev, currentText: text }));
  };

  const updateImportTask = (taskId: string, updater: (t: ImportTask) => ImportTask) => {
    setImportProgress((prev) => ({
      ...prev,
      tasks: prev.tasks.map((t) => (t.id === taskId ? updater(t) : t)),
    }));
  };

  const handleValidateYaml = () => {
    try {
      if (!yamlText || yamlText.trim() === "") {
        throw new Error("YAML input is empty.");
      }

      // Parse YAML
      const parsedData: MediuxYaml = yaml.load(yamlText) as MediuxYaml;

      // Validate basic structure
      if (typeof parsedData !== "object" || parsedData === null) {
        throw new Error("YAML does not represent a valid object.");
      }

      // Extract author and setId from comments
      const authorMatch = yamlText.match(/Set by ([\w\d_-]+) on MediUX/i);
      const setIdMatch = yamlText.match(/mediux\.pro\/sets\/(\d+)/i);
      const author = authorMatch ? authorMatch[1] : null;
      const setId = setIdMatch ? setIdMatch[1] : null;

      setSetAuthor(author);
      setSetId(setId);

      // Build the poster set using your utility
      const posterSet = yamlToPosterSet(parsedData, mediaItem, mediaItem.type as "movie" | "show", setId, author);

      // Count posters, backdrops, etc. from the posterSet object
      const posterCount = posterSet.images?.filter((img) => img.type === "poster").length || 0;
      const backdropCount = posterSet.images?.filter((img) => img.type === "backdrop").length || 0;
      const seasonPosterCount = posterSet.images?.filter((img) => img.type === "season_poster").length || 0;
      const titlecardCount = posterSet.images?.filter((img) => img.type === "titlecard").length || 0;

      setValidPosterSet(posterSet);
      setNumberOfPostersInSet(posterCount);
      setNumberOfBackdropsInSet(backdropCount);
      setNumberOfSeasonPostersInSet(seasonPosterCount);
      setNumberOfTitlecardsInSet(titlecardCount);

      setIsValidYaml(true);
    } catch (error) {
      setIsValidYaml(false);
      setError("Error: " + (error instanceof Error ? error.message : String(error)));
    }
  };

  const getFileDownloadName = (imageFile: ImageFile, mediaItem: MediaItem) => {
    if (imageFile.type === "poster") {
      return `Poster for ${mediaItem.title}`;
    } else if (imageFile.type === "backdrop") {
      return `Backdrop for ${mediaItem.title}`;
    } else if (imageFile.type === "season_poster") {
      return `Season ${imageFile.season_number} Poster for ${mediaItem.title}`;
    } else if (imageFile.type === "titlecard") {
      const season = imageFile.season_number ?? "";
      const episode =
        imageFile.episode_number != null ? String(imageFile.episode_number).padStart(2, "0") : (imageFile.title ?? "");

      return `S${season}E${episode} Titlecard for ${mediaItem.title}`;
    } else {
      return `Image for ${mediaItem.title}`;
    }
  };

  const buildImportTasks = (posterSet: PosterSet): ImportTask[] => {
    const tasks: ImportTask[] = [];
    for (const image of posterSet.images || []) {
      const label = getFileDownloadName(image, mediaItem);
      tasks.push({
        id: newId(),
        status: "pending",
        label,
        imageFile: image,
      });
    }
    return tasks;
  };

  const runImportTask = async (t: ImportTask) => {
    updateImportTask(t.id, (task) => ({ ...task, status: "in-progress" }));
    try {
      setImportText(t.label);
      const response = await downloadImageFileForMediaItem(t.imageFile, mediaItem, t.label);
      if (response.status === "error") {
        throw new Error(response.error?.message || "Unknown error");
      }

      updateImportTask(t.id, (task) => ({ ...task, status: "completed" }));
    } catch (error) {
      updateImportTask(t.id, (task) => ({
        ...task,
        status: "failed",
        error: error instanceof Error ? error.message : String(error),
      }));
    }
  };

  const handleImport = async () => {
    const tasks = buildImportTasks(validPosterSet!);
    setImportProgress({ currentText: "Starting import...", totalPlanned: tasks.length, tasks });
    for (const task of tasks) {
      await runImportTask(task);
    }
    setImportText("Completed!");
  };

  const percent = getImportPercentComplete(importProgress);
  const { done, total } = getRefreshCounts(importProgress);
  const hasProgress = importProgress.totalPlanned > 0;
  const hasErrors = importProgress.tasks.some((t) => t.status === "failed");

  return (
    <Dialog open={isOpen} onOpenChange={onClose}>
      <DialogContent className={cn("max-h-[80vh] overflow-y-auto sm:max-w-[700px]", "border border-primary")}>
        <DialogHeader>
          <DialogTitle>Manual YAML Import for '{mediaItem.title}'</DialogTitle>
          {setAuthor && (
            <div className="flex items-center justify-center sm:justify-start">
              <Avatar className="rounded-lg mr-1 w-4 h-4">
                <AvatarImage src={`/api/images/mediux/avatar?username=${setAuthor}`} className="w-4 h-4" />
                <AvatarFallback className="">
                  <User className="w-4 h-4" />
                </AvatarFallback>
              </Avatar>
              <Link href={`/user/${setAuthor}`} className="hover:underline">
                {setAuthor}
              </Link>
            </div>
          )}
          {setId && <DialogDescription>Set ID: {setId}</DialogDescription>}
        </DialogHeader>
        {!isValidYaml ? (
          <div>
            <label className="block mb-2 font-medium">MediUX YAML:</label>
            <Textarea
              value={yamlText}
              onChange={(e) => setYamlText(e.target.value)}
              rows={5}
              placeholder="Paste MediUX YAML here..."
            />
            {error && <div className="text-destructive mt-2">{error}</div>}
          </div>
        ) : (
          <div className="flex flex-wrap gap-2 mt-2">
            {numberOfPostersInSet !== null && (
              <Badge>
                {makePlural(numberOfPostersInSet, "Poster")}: {numberOfPostersInSet}
              </Badge>
            )}
            {numberOfBackdropsInSet !== null && (
              <Badge>
                {makePlural(numberOfBackdropsInSet, "Backdrop")}: {numberOfBackdropsInSet}
              </Badge>
            )}
            {numberOfSeasonPostersInSet !== null && (
              <Badge>
                {makePlural(numberOfSeasonPostersInSet, "Season Poster")}: {numberOfSeasonPostersInSet}
              </Badge>
            )}
            {numberOfTitlecardsInSet !== null && (
              <Badge>
                {makePlural(numberOfTitlecardsInSet, "Titlecard")}: {numberOfTitlecardsInSet}
              </Badge>
            )}
          </div>
        )}

        {/* Progress Bar */}
        {hasProgress ? (
          <div className="mt-3 min-h-[24px]">
            <div className="flex items-center justify-between gap-2">
              <div className="relative w-full min-w-0">
                <Progress
                  value={percent}
                  className={cn(
                    "w-full rounded-md overflow-hidden",
                    percent < 100 ? "h-5" : "h-3",
                    percent === 100 && !hasErrors && "[&>div]:bg-green-500",
                    percent === 100 && hasErrors && "[&>div]:bg-destructive",
                    percent < 100 && "[&>div]:bg-primary"
                  )}
                />
                {percent < 100 && (
                  <span
                    className={cn(
                      "absolute inset-0 flex items-center justify-center",
                      "text-xs text-white pointer-events-none",
                      "px-2 min-w-0"
                    )}
                    title={importProgress.currentText}
                  >
                    <span className="w-full min-w-0 truncate text-center">{importProgress.currentText}</span>
                  </span>
                )}
              </div>

              <span className="text-sm text-muted-foreground min-w-[56px] text-right tabular-nums">{percent}%</span>
            </div>

            {done !== total && (
              <div className="mt-1 text-xs text-muted-foreground tabular-nums text-right">
                {done}/{total}
              </div>
            )}
          </div>
        ) : null}

        <DialogFooter>
          <Button variant="outline" onClick={onClose}>
            Cancel
          </Button>
          {!isValidYaml && <Button onClick={handleValidateYaml}>Validate YAML</Button>}
          {isValidYaml && (
            <>
              <Button
                variant="destructive"
                onClick={() => {
                  setIsValidYaml(false);
                  setYamlText("");
                  setSetAuthor(null);
                  setSetId(null);
                  setNumberOfPostersInSet(null);
                  setNumberOfBackdropsInSet(null);
                  setNumberOfSeasonPostersInSet(null);
                  setNumberOfTitlecardsInSet(null);
                  setError(null);
                }}
              >
                Clear
              </Button>
              <Button onClick={handleImport}>Import</Button>
            </>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

export function yamlToPosterSet(
  yamlData: MediuxYaml,
  mediaItem: MediaItem,
  type: "movie" | "show",
  setId: string | null,
  setAuthor: string | null
): PosterSet {
  const posterSet: PosterSet = {
    id: setId || "",
    title: mediaItem.title,
    type: type,
    user_created: setAuthor || "",
    date_created: new Date().toISOString(),
    date_updated: new Date().toISOString(),
    popularity: 0,
    popularity_global: 0,
    images: [],
  };

  function extractAssetId(url?: string): string {
    if (!url) return "";
    const parts = url.split("/");
    return parts[parts.length - 1];
  }

  // Helper to create PosterFile
  const makePosterFile = (
    src: string,
    fileType: TYPE_DOWNLOAD_IMAGE_TYPE_OPTIONS,
    seasonNumber?: number,
    episodeNumber?: number
  ): ImageFile => {
    const assetId = extractAssetId(src);
    return {
      id: "---" + assetId,
      type: fileType,
      modified: new Date().toISOString(),
      file_size: 0,
      src: "---" + assetId,
      blurhash: "",
      season_number: seasonNumber,
      episode_number: episodeNumber,
      item_tmdb_id: mediaItem.tmdb_id,
      language: "",
    };
  };

  if (type === "movie") {
    // Only keep the item that matches the mediaItem (e.g., by TMDB ID)
    const matchKey = mediaItem.tmdb_id;
    const item = yamlData[matchKey];
    if (!item) throw new Error("No matching item found in YAML for this media item.");
    if (item.url_poster) posterSet.images.push(makePosterFile(item.url_poster, "poster"));
    if (item.url_background) posterSet.images.push(makePosterFile(item.url_background, "backdrop"));
  } else if (type === "show") {
    // For shows, just use the first item in the YAML
    const firstKey = Object.keys(yamlData)[0];
    const item = yamlData[firstKey];

    if (item.url_poster) posterSet.images.push(makePosterFile(item.url_poster, "poster"));
    if (item.url_background) posterSet.images.push(makePosterFile(item.url_background, "backdrop"));
    if (item.seasons) {
      for (const [seasonNum, seasonData] of Object.entries(item.seasons)) {
        if (seasonData.url_poster && mediaItem.series?.seasons.find((s) => s.season_number.toString() === seasonNum)) {
          posterSet.images.push(makePosterFile(seasonData.url_poster, "season_poster", parseInt(seasonNum)));
        }
        for (const [episodeNum, episodeData] of Object.entries(seasonData.episodes || {})) {
          if (
            episodeData.url_poster &&
            mediaItem.series?.seasons.find((s) =>
              s.episodes.find(
                (e) => e.season_number.toString() === seasonNum && e.episode_number.toString() === episodeNum
              )
            )
          ) {
            posterSet.images.push(
              makePosterFile(episodeData.url_poster, "titlecard", parseInt(seasonNum), parseInt(episodeNum))
            );
          }
        }
      }
    }
  }

  return posterSet;
}

export type MediuxYaml = {
  [key: string]: MediuxYamlEntry;
};

export type MediuxYamlEntry = {
  url_poster?: string;
  url_background?: string;
  seasons?: {
    [seasonNumber: string]: MediuxYamlSeasonEntry;
  };
};

export type MediuxYamlSeasonEntry = {
  url_poster?: string;
  episodes?: {
    [episodeNumber: string]: MediuxYamlEpisodeEntry;
  };
};

export type MediuxYamlEpisodeEntry = {
  url_poster?: string;
};
