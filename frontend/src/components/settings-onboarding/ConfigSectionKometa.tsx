"use client";

import { GetKometaImportStatus, type KometaImportResult, TriggerKometaImport } from "@/services/kometa/import";
import { toast } from "sonner";

import { useEffect, useMemo, useState } from "react";

import { PopoverHelp } from "@/components/shared/popover-help";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";

import { cn } from "@/lib/cn";

import type { AppConfigImagesKometa, AppConfigMediaServerLibrary } from "@/types/config/config";

// Fork: Kometa asset-directory mode (Plex only), rendered inside the Images section.
// The import schedule is the "Kometa Asset Import Job" on the Jobs page.

interface ConfigSectionKometaProps {
  kometa: AppConfigImagesKometa;
  editing: boolean;
  showActions: boolean;
  mediaServerType?: string;
  libraries: AppConfigMediaServerLibrary[];
  dirtyFields?: Partial<Record<keyof AppConfigImagesKometa, boolean>>;
  onChange: <F extends keyof AppConfigImagesKometa>(field: F, value: AppConfigImagesKometa[F]) => void;
}

export function kometaError(kometa: AppConfigImagesKometa | undefined, mediaServerType?: string): string | undefined {
  if (!kometa?.enabled) return undefined;
  if (mediaServerType !== "Plex") {
    return "Kometa mode only supports Plex. Disable it or switch the media server type back to Plex.";
  }
  if (!kometa.asset_directory) return "Kometa asset directory is required when Kometa mode is enabled.";
  return undefined;
}

export function ConfigSectionKometa({
  kometa,
  editing,
  showActions,
  mediaServerType,
  libraries,
  dirtyFields = {},
  onChange,
}: ConfigSectionKometaProps) {
  const [kometaImporting, setKometaImporting] = useState(false);
  const [kometaResult, setKometaResult] = useState<KometaImportResult | null>(null);
  const error = useMemo(() => kometaError(kometa, mediaServerType), [kometa, mediaServerType]);

  // Kometa asset subfolders can only be assigned to Plex movie/show libraries.
  const kometaLibraries = libraries.filter((lib) => lib.type === "movie" || lib.type === "show");

  // An empty value removes the entry, so that library writes flat under the asset directory.
  const setLibraryAssetFolder = (libraryTitle: string, folder: string) => {
    const next: Record<string, string> = { ...(kometa.library_asset_folders ?? {}) };
    if (folder.trim() === "") {
      delete next[libraryTitle];
    } else {
      next[libraryTitle] = folder;
    }
    onChange("library_asset_folders", next);
  };

  // Sync with an import started elsewhere (the job or the API), so the button state is accurate.
  useEffect(() => {
    if (!showActions) return;
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
  }, [showActions]);

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

  return (
    <>
      {/* Kometa Mode (Plex only). Stays visible while enabled on a non-Plex server so the
          user can still turn it off — otherwise the save is rejected (Kometa is Plex-only)
          with no visible switch to clear it. */}
      {(mediaServerType === "Plex" || kometa.enabled) && (
        <div
          className={cn("border rounded-md p-3 transition", "border-muted", dirtyFields.enabled && "border-amber-500")}
        >
          <div className="flex items-center justify-between mb-2">
            <Label className="mr-2">Kometa Mode</Label>
            <div className="flex items-center gap-2">
              <Switch disabled={!editing} checked={!!kometa.enabled} onCheckedChange={(v) => onChange("enabled", v)} />
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

          {kometa.enabled && (
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
                  value={kometa.asset_directory || ""}
                  onChange={(e) => onChange("asset_directory", e.target.value)}
                  className={cn(
                    "w-full px-3 py-2 border rounded-md focus:outline-none focus:ring-2 focus:ring-primary disabled:opacity-50 transition",
                    (dirtyFields.asset_directory || error) && "border-amber-500"
                  )}
                  placeholder="/assets"
                />
                {error && <p className="text-sm text-destructive mt-1">{error}</p>}
              </div>

              {kometaLibraries.length > 0 && (
                <div
                  className={cn(
                    "rounded-md border border-muted p-3",
                    dirtyFields.library_asset_folders && "border-amber-500"
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
                          value={kometa.library_asset_folders?.[lib.title] ?? ""}
                          onChange={(e) => setLibraryAssetFolder(lib.title, e.target.value)}
                          className="flex-1 px-3 py-2 border rounded-md focus:outline-none focus:ring-2 focus:ring-primary disabled:opacity-50 transition"
                          placeholder={`e.g. ${lib.type === "movie" ? "movies" : "tv"} (blank = asset directory root)`}
                        />
                      </div>
                    ))}
                  </div>
                </div>
              )}

              <div
                className={cn(
                  "flex items-center justify-between border rounded-md p-3 transition",
                  "border-muted",
                  dirtyFields.sonarr_radarr_fallback && "border-amber-500"
                )}
              >
                <Label className="mr-2">Sonarr/Radarr Fallback</Label>
                <div className="flex items-center gap-2">
                  <Switch
                    disabled={!editing}
                    checked={!!kometa.sonarr_radarr_fallback}
                    onCheckedChange={(v) => onChange("sonarr_radarr_fallback", v)}
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

              {showActions && (
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
    </>
  );
}
