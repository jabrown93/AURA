"use client";

import { ReturnErrorMessage } from "@/services/api-error-return";
import { DeleteTempImages } from "@/services/images/api-images-actions";
import { GetKometaImportStatus, type KometaImportResult, TriggerKometaImport } from "@/services/kometa/import";
import { toast } from "sonner";

import React, { useEffect, useRef, useState } from "react";

import { ConfirmDestructiveDialogActionButton } from "@/components/shared/dialog-destructive-action";
import { PopoverHelp } from "@/components/shared/popover-help";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectScrollDownButton,
  SelectScrollUpButton,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";

import { cn } from "@/lib/cn";

import type { AppConfigImages, AppConfigMediaServerLibrary } from "@/types/config/config";

const EPISODE_NAMING_CONVENTION_OPTIONS = ["match", "static"];

interface ConfigSectionImagesProps {
  value: AppConfigImages;
  editing: boolean;
  dirtyFields?: {
    cache_images?: { enabled?: boolean };
    save_images_locally?: {
      enabled?: boolean;
      path?: boolean;
      episode_naming_convention?: boolean;
    };
    kometa?: {
      enabled?: boolean;
      asset_directory?: boolean;
      library_asset_folders?: boolean;
      import_cron?: boolean;
      sonarr_radarr_fallback?: boolean;
    };
  };
  onChange: <K extends keyof AppConfigImages, F extends keyof AppConfigImages[K]>(
    group: K,
    field: F,
    value: AppConfigImages[K][F]
  ) => void;
  errorsUpdate?: (errors: Partial<Record<keyof AppConfigImages, string>>) => void;
  mediaServerType?: string;
  libraries?: AppConfigMediaServerLibrary[];
}

export const ConfigSectionImages: React.FC<ConfigSectionImagesProps> = ({
  value,
  editing,
  dirtyFields = {},
  onChange,
  errorsUpdate,
  mediaServerType,
  libraries = [],
}) => {
  const prevErrorsRef = useRef<string>("{}");

  const [kometaImporting, setKometaImporting] = useState(false);
  const [kometaResult, setKometaResult] = useState<KometaImportResult | null>(null);

  // Kometa asset subfolders can only be assigned to Plex movie/show libraries.
  const kometaLibraries = libraries.filter((lib) => lib.type === "movie" || lib.type === "show");

  // Update the per-library Kometa subfolder map. An empty value removes the entry, so that
  // library falls back to writing flat under the asset directory.
  const setLibraryAssetFolder = (libraryTitle: string, folder: string) => {
    const next: Record<string, string> = { ...(value.kometa.library_asset_folders ?? {}) };
    if (folder.trim() === "") {
      delete next[libraryTitle];
    } else {
      next[libraryTitle] = folder;
    }
    onChange("kometa", "library_asset_folders", next);
  };

  const clearTempImagesFolder = async () => {
    try {
      const response = await DeleteTempImages();
      if (response.status === "error") {
        toast.error(response.error?.message || "Failed to clear temp images folder");
        return;
      }
      toast.success(response.data?.message || "Temp images folder cleared successfully");
    } catch (error) {
      const errorResponse = ReturnErrorMessage<void>(error);
      toast.error(errorResponse.error?.message || "An unexpected error occurred");
    }
  };

  // On mount, sync with any import that was started elsewhere (cron or direct API call)
  // so the button state and last result are accurate before the user interacts.
  useEffect(() => {
    let cancelled = false;
    (async () => {
      const response = await GetKometaImportStatus();
      if (cancelled || response.status === "error" || !response.data) return;
      if (response.data.result) setKometaResult(response.data.result);
      if (response.data.running) setKometaImporting(true);
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  // Poll import status while an import is running so the UI reflects progress and results.
  useEffect(() => {
    if (!kometaImporting) return;
    let cancelled = false;
    const interval = setInterval(async () => {
      const response = await GetKometaImportStatus();
      if (cancelled || response.status === "error" || !response.data) return;
      if (response.data.result) setKometaResult(response.data.result);
      if (!response.data.running) {
        setKometaImporting(false);
        toast.success("Kometa asset import finished");
      }
    }, 2000);
    return () => {
      cancelled = true;
      clearInterval(interval);
    };
  }, [kometaImporting]);

  const runKometaImport = async () => {
    const response = await TriggerKometaImport();
    if (response.status === "error") {
      toast.error(response.error?.message || "Failed to start Kometa import");
      return;
    }
    setKometaImporting(true);
    toast.info("Kometa asset import started");
  };

  const errors = React.useMemo<Partial<Record<keyof AppConfigImages, string>>>(() => {
    const errs: Partial<Record<keyof AppConfigImages, string>> = {};

    // If Media Server Type is Plex, validate SaveImagesLocally.EpisodeNamingConvention
    if (mediaServerType && mediaServerType === "Plex" && value.save_images_locally.enabled) {
      if (!value.save_images_locally.episode_naming_convention) {
        errs.save_images_locally = "Episode naming convention is required.";
      } else {
        if (!EPISODE_NAMING_CONVENTION_OPTIONS.includes(value.save_images_locally.episode_naming_convention)) {
          errs.save_images_locally = `Episode naming convention must be one of: ${EPISODE_NAMING_CONVENTION_OPTIONS.join(", ")}.`;
        }
      }
    }

    // Kometa mode is Plex-only; if it stays enabled after a server-type change the backend
    // rejects the save, so tell the user how to resolve it.
    if (mediaServerType !== "Plex" && value.kometa.enabled) {
      errs.kometa = "Kometa mode only supports Plex. Disable it or switch the media server type back to Plex.";
    }

    // If Kometa mode is enabled (Plex only), an asset directory is required.
    if (mediaServerType === "Plex" && value.kometa.enabled && !value.kometa.asset_directory) {
      errs.kometa = "Kometa asset directory is required when Kometa mode is enabled.";
    }

    return errs;
  }, [
    mediaServerType,
    value.save_images_locally.enabled,
    value.save_images_locally.episode_naming_convention,
    value.kometa.enabled,
    value.kometa.asset_directory,
  ]);

  // Emit errors upward
  useEffect(() => {
    if (!errorsUpdate) return;
    const serialized = JSON.stringify(errors);
    if (serialized === prevErrorsRef.current) return;
    prevErrorsRef.current = serialized;
    errorsUpdate(errors);
  }, [errors, errorsUpdate]);

  return (
    <Card className="p-5">
      <div className="flex items-center justify-between">
        <h2 className="text-xl font-semibold text-blue-500">Images</h2>
        <ConfirmDestructiveDialogActionButton
          hidden={editing}
          onConfirm={clearTempImagesFolder}
          title="Clear Temp Images Folder?"
          description="This will permanently delete all temporary images. Are you sure you want to continue?"
          confirmText="Yes, Clear Images"
          cancelText="Cancel"
          className="text-destructive border-1 shadow-none hover:text-red-500 cursor-pointer"
          variant="ghost"
        >
          Clear Temp Images
        </ConfirmDestructiveDialogActionButton>
      </div>
      {/* Cache Images */}
      <div
        className={cn(
          "flex items-center justify-between border rounded-md p-3 transition",
          "border-muted",
          dirtyFields.cache_images?.enabled && "border-amber-500"
        )}
      >
        <Label className="mr-2">Cache Images</Label>
        <div className="flex items-center gap-2">
          <Switch
            disabled={!editing}
            checked={value.cache_images.enabled}
            onCheckedChange={(v) => onChange("cache_images", "enabled", v)}
          />
          {editing && (
            <PopoverHelp ariaLabel="help-images-cache">
              <p>Store downloaded artwork locally to reduce external requests and speed repeat access.</p>
            </PopoverHelp>
          )}
        </div>
      </div>

      {/* Save Images Locally */}
      {mediaServerType === "Plex" && (
        <div
          className={cn(
            "border rounded-md p-3 transition",
            "border-muted",
            dirtyFields.save_images_locally?.enabled && "border-amber-500"
          )}
        >
          <div className="flex items-center justify-between mb-2">
            <Label className="mr-2">Save Images Locally</Label>
            <div className="flex items-center gap-2">
              <Switch
                disabled={!editing}
                checked={!!value.save_images_locally.enabled}
                onCheckedChange={(v) => onChange("save_images_locally", "enabled", v)}
              />
              {editing && (
                <PopoverHelp ariaLabel="help-images-save-next-to-content">
                  <p>
                    Save images to a local folder on the server. This is useful for not relying on your Media Server
                    database. Make sure the path is accessible by the Aura server.
                  </p>
                </PopoverHelp>
              )}
            </div>
          </div>

          {value.save_images_locally.enabled && (
            <div
              className={cn(
                "mt-2",
                dirtyFields.save_images_locally?.enabled && "border border-amber-500 rounded-md p-2"
              )}
            >
              <div className="flex items-center justify-between mb-2">
                <Label className="mr-2">Path</Label>
                {editing && (
                  <PopoverHelp ariaLabel="help-images-save-path">
                    <p>
                      Enter the local folder path where images should be saved. This must be accessible by the Aura
                      server. Leave this blank if you want to save images next to the content.
                    </p>
                  </PopoverHelp>
                )}
              </div>
              <Input
                type="text"
                disabled={!editing}
                value={value.save_images_locally.path || ""}
                onChange={(e) => onChange("save_images_locally", "path", e.target.value)}
                className={cn(
                  "w-full px-3 py-2 border rounded-md focus:outline-none focus:ring-2 focus:ring-primary disabled:opacity-50 transition",
                  dirtyFields.save_images_locally?.path && "border-amber-500"
                )}
                placeholder="/path/to/images"
              />
            </div>
          )}

          {mediaServerType === "Plex" && value.save_images_locally.enabled && (
            <div className={cn("space-y-1 mt-4")}>
              <div className="flex items-center justify-between">
                <Label>Episode Naming Convention</Label>
                {editing && (
                  <PopoverHelp ariaLabel="help-media-server-episode-naming-convention">
                    <div className="space-y-3">
                      <div>
                        <p className="font-medium mb-1">Episode Naming Convention</p>
                        <p className="text-muted-foreground">How Plex episode files are named.</p>
                      </div>
                      <ul className="space-y-1">
                        <li className="flex items-center gap-2">
                          <span className="inline-flex h-5 items-center rounded-sm bg-muted px-2 font-mono ">
                            match
                          </span>
                          <span>Some Episode Title S01E01.jpg</span>
                        </li>
                        <li className="flex items-center gap-2">
                          <span className="inline-flex h-5 items-center rounded-sm bg-muted px-2 font-mono">
                            static
                          </span>
                          <span>S01E01.jpg</span>
                        </li>
                      </ul>
                      <p className="text-muted-foreground">Used for file naming logic.</p>
                    </div>
                  </PopoverHelp>
                )}
              </div>
              <Select
                disabled={!editing}
                value={value.save_images_locally.episode_naming_convention || ""}
                onValueChange={(v) => onChange("save_images_locally", "episode_naming_convention", v)}
              >
                <SelectTrigger
                  id="media-server-episode-naming-convention-trigger"
                  className={cn(
                    "w-full",
                    dirtyFields.save_images_locally?.episode_naming_convention && "border-amber-500"
                  )}
                >
                  <SelectValue placeholder="Select convention..." />
                </SelectTrigger>
                <SelectContent>
                  {EPISODE_NAMING_CONVENTION_OPTIONS.map((o) => (
                    <SelectItem key={o} value={o}>
                      {o}
                    </SelectItem>
                  ))}
                  <SelectScrollUpButton />
                  <SelectScrollDownButton />
                </SelectContent>
              </Select>
            </div>
          )}
        </div>
      )}

      {/* Kometa Mode (Plex only). Stays visible while enabled on a non-Plex server so the
          user can still turn it off — otherwise the save is rejected (Kometa is Plex-only)
          with no visible switch to clear it. */}
      {(mediaServerType === "Plex" || value.kometa.enabled) && (
        <div
          className={cn(
            "border rounded-md p-3 transition",
            "border-muted",
            dirtyFields.kometa?.enabled && "border-amber-500"
          )}
        >
          <div className="flex items-center justify-between mb-2">
            <Label className="mr-2">Kometa Mode</Label>
            <div className="flex items-center gap-2">
              <Switch
                disabled={!editing}
                checked={!!value.kometa.enabled}
                onCheckedChange={(v) => onChange("kometa", "enabled", v)}
              />
              {editing && (
                <PopoverHelp ariaLabel="help-images-kometa">
                  <p>
                    Write downloaded images into your Kometa asset directory using Kometa&apos;s folder-per-item naming
                    (<span className="font-mono">poster.jpg</span>, <span className="font-mono">background.jpg</span>,{" "}
                    <span className="font-mono">Season01.jpg</span>, <span className="font-mono">S01E01.jpg</span>).
                    Images are still applied to Plex immediately. Plex only.
                  </p>
                </PopoverHelp>
              )}
            </div>
          </div>

          {value.kometa.enabled && (
            <div className="mt-2 space-y-4">
              <div>
                <div className="flex items-center justify-between mb-2">
                  <Label className="mr-2">Asset Directory</Label>
                  {editing && (
                    <PopoverHelp ariaLabel="help-images-kometa-asset-dir">
                      <p>
                        The directory Kometa reads assets from (its <span className="font-mono">asset_directory</span>).
                        Must be mounted into the Aura container at this exact path.
                      </p>
                    </PopoverHelp>
                  )}
                </div>
                <Input
                  type="text"
                  disabled={!editing}
                  value={value.kometa.asset_directory || ""}
                  onChange={(e) => onChange("kometa", "asset_directory", e.target.value)}
                  className={cn(
                    "w-full px-3 py-2 border rounded-md focus:outline-none focus:ring-2 focus:ring-primary disabled:opacity-50 transition",
                    (dirtyFields.kometa?.asset_directory || errors.kometa) && "border-amber-500"
                  )}
                  placeholder="/assets"
                />
                {errors.kometa && <p className="text-sm text-destructive mt-1">{errors.kometa}</p>}
              </div>

              {kometaLibraries.length > 0 && (
                <div
                  className={cn(
                    "rounded-md border border-muted p-3",
                    dirtyFields.kometa?.library_asset_folders && "border-amber-500"
                  )}
                >
                  <div className="flex items-center justify-between mb-2">
                    <Label className="mr-2">Per-Library Subfolders (optional)</Label>
                    {editing && (
                      <PopoverHelp ariaLabel="help-images-kometa-library-folders">
                        <p>
                          Optionally write each library&apos;s assets into its own subfolder of the asset directory (for
                          example <span className="font-mono">movies</span>, <span className="font-mono">tv</span>,{" "}
                          <span className="font-mono">anime</span>). Leave a library blank to write directly under the
                          asset directory. The subfolder must match the per-library{" "}
                          <span className="font-mono">asset_directory</span> you configured in Kometa — Aura writes
                          there, Kometa reads from there.
                        </p>
                      </PopoverHelp>
                    )}
                  </div>
                  <div className="space-y-2">
                    {kometaLibraries.map((lib) => (
                      <div key={lib.title} className="flex items-center gap-2">
                        <span className="w-2/5 shrink-0 truncate text-sm text-muted-foreground" title={lib.title}>
                          {lib.title}
                        </span>
                        <Input
                          type="text"
                          disabled={!editing}
                          value={value.kometa.library_asset_folders?.[lib.title] ?? ""}
                          onChange={(e) => setLibraryAssetFolder(lib.title, e.target.value)}
                          className="flex-1 px-3 py-2 border rounded-md focus:outline-none focus:ring-2 focus:ring-primary disabled:opacity-50 transition"
                          placeholder={`e.g. ${lib.type === "movie" ? "movies" : "tv"} (blank = asset directory root)`}
                        />
                      </div>
                    ))}
                  </div>
                </div>
              )}

              <div>
                <div className="flex items-center justify-between mb-2">
                  <Label className="mr-2">Import Schedule (Cron)</Label>
                  {editing && (
                    <PopoverHelp ariaLabel="help-images-kometa-cron">
                      <p>
                        Optional cron expression to periodically import existing Kometa assets. Leave blank to only
                        import manually with the button below.
                      </p>
                    </PopoverHelp>
                  )}
                </div>
                <Input
                  type="text"
                  disabled={!editing}
                  value={value.kometa.import_cron || ""}
                  onChange={(e) => onChange("kometa", "import_cron", e.target.value)}
                  className={cn(
                    "w-full px-3 py-2 border rounded-md focus:outline-none focus:ring-2 focus:ring-primary disabled:opacity-50 transition",
                    dirtyFields.kometa?.import_cron && "border-amber-500"
                  )}
                  placeholder="0 3 * * * (optional)"
                />
              </div>

              <div
                className={cn(
                  "flex items-center justify-between border rounded-md p-3 transition",
                  "border-muted",
                  dirtyFields.kometa?.sonarr_radarr_fallback && "border-amber-500"
                )}
              >
                <Label className="mr-2">Sonarr/Radarr Fallback</Label>
                <div className="flex items-center gap-2">
                  <Switch
                    disabled={!editing}
                    checked={!!value.kometa.sonarr_radarr_fallback}
                    onCheckedChange={(v) => onChange("kometa", "sonarr_radarr_fallback", v)}
                  />
                  {editing && (
                    <PopoverHelp ariaLabel="help-images-kometa-sonarr-radarr-fallback">
                      <p>
                        When a media-server lookup fails (for example Plex returns a 404 for an item it can no longer
                        find) but the show or movie still exists in Sonarr/Radarr, save the downloaded images into the
                        Kometa asset folder anyway. The asset folder name is taken from the Sonarr/Radarr path, and the
                        images are recorded as a Kometa set rather than applied to Plex. Requires a Sonarr/Radarr
                        instance configured for this library.
                      </p>
                    </PopoverHelp>
                  )}
                </div>
              </div>

              {!editing && (
                <div className="flex flex-col gap-2">
                  <Button type="button" variant="outline" onClick={runKometaImport} disabled={kometaImporting}>
                    {kometaImporting ? "Importing…" : "Import Existing Kometa Assets"}
                  </Button>
                  {kometaResult && (
                    <p className="text-sm text-muted-foreground">
                      Last import: {kometaResult.images_uploaded} images uploaded, {kometaResult.items_registered} items
                      tracked, {kometaResult.unmatched_folders} unmatched
                      {kometaResult.images_skipped_owned > 0 &&
                        `, ${kometaResult.images_skipped_owned} skipped (AURA-managed)`}
                      {kometaResult.error && ` — error: ${kometaResult.error}`}
                    </p>
                  )}
                </div>
              )}
            </div>
          )}
        </div>
      )}
    </Card>
  );
};
