"use client";

import { formatDownloadSize } from "@/helper/format-download-size";
import { CreateDatabaseBackup, type DatabaseBackups_Response, GetDatabaseBackups } from "@/services/database/backups";
import { DatabaseBackup, Loader2 } from "lucide-react";
import { toast } from "sonner";

import React, { useCallback, useEffect, useRef, useState } from "react";

import Link from "next/link";

import { NumberInput } from "@/components/shared/number-input";
import { PopoverHelp } from "@/components/shared/popover-help";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Label } from "@/components/ui/label";

import { cn } from "@/lib/cn";

import type { AppConfigBackups } from "@/types/config/config";

interface ConfigSectionBackupsProps {
  value: AppConfigBackups;
  dirtyFields?: Partial<Record<keyof AppConfigBackups, boolean>>;
  onChange: <K extends keyof AppConfigBackups>(field: K, value: AppConfigBackups[K]) => void;
  errorsUpdate?: (errors: Partial<Record<keyof AppConfigBackups, string>>) => void;
}

export const ConfigSectionBackups: React.FC<ConfigSectionBackupsProps> = ({
  value,
  dirtyFields = {},
  onChange,
  errorsUpdate,
}) => {
  const prevErrorsRef = useRef<string>("");
  const [backups, setBackups] = useState<DatabaseBackups_Response | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [backingUp, setBackingUp] = useState(false);

  const errors = React.useMemo(() => {
    const errs: Partial<Record<keyof AppConfigBackups, string>> = {};
    if (!Number.isInteger(value.keep) || value.keep < 1 || value.keep > 100)
      errs.keep = "Use a number between 1 and 100.";
    return errs;
  }, [value.keep]);

  useEffect(() => {
    if (!errorsUpdate) return;
    const ser = JSON.stringify(errors);
    if (ser === prevErrorsRef.current) return;
    prevErrorsRef.current = ser;
    errorsUpdate(errors);
  }, [errors, errorsUpdate]);

  const loadBackups = useCallback(async () => {
    const response = await GetDatabaseBackups();
    if (response.status === "error" || !response.data) {
      setLoadError(response.error?.message || "Could not load the backups");
      return;
    }
    setLoadError(null);
    setBackups(response.data);
  }, []);

  useEffect(() => {
    void loadBackups();
  }, [loadBackups]);

  const handleBackUpNow = async () => {
    setBackingUp(true);
    const response = await CreateDatabaseBackup();
    setBackingUp(false);
    if (response.status === "error" || !response.data) {
      toast.error(response.error?.message || "Could not back up the database");
      return;
    }
    toast.success(`Database backed up to ${response.data.backup.name}`);
    setBackups(response.data);
  };

  return (
    <Card className={cn("gap-4 p-4 sm:p-5", errors.keep ? "border-red-500" : "border-muted")}>
      {/* Keep Backups */}
      <div className={cn("space-y-1 border rounded-md p-3", dirtyFields.keep && "border-amber-500")}>
        <div className="flex items-center justify-between mb-1">
          <Label htmlFor="backups-keep">Keep Backups</Label>
          <PopoverHelp ariaLabel="help-backups-keep">
            <p>Only this many backups are kept. The oldest are deleted when a new backup is made.</p>
          </PopoverHelp>
        </div>
        <NumberInput
          id="backups-keep"
          value={value.keep}
          onChange={(keep) => onChange("keep", keep)}
          placeholder="7"
          min={1}
          max={100}
        />
        {errors.keep && <p className="text-xs text-red-500">{errors.keep}</p>}
        <p className="text-xs text-muted-foreground">
          aura backs up the database with the Backup Database job. Change when it runs on the{" "}
          <Link href="/jobs" className="text-primary hover:underline">
            Jobs
          </Link>{" "}
          page.
        </p>
      </div>

      {/* Backups */}
      <div className="space-y-3 border rounded-md p-3">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <div className="flex items-center gap-2">
            <Label>Backups</Label>
            <PopoverHelp ariaLabel="help-backups-restore">
              <p className="mb-2">Each backup has your settings and saved sets.</p>
              <p className="text-muted-foreground">
                To restore one, stop aura, replace AURA.db in your config folder with the backup file renamed to
                AURA.db, then start aura again.
              </p>
            </PopoverHelp>
          </div>
          <Button variant="outline" size="sm" onClick={handleBackUpNow} disabled={backingUp} className="cursor-pointer">
            {backingUp ? <Loader2 className="h-4 w-4 animate-spin" /> : <DatabaseBackup className="h-4 w-4" />}
            Back Up Now
          </Button>
        </div>

        {loadError ? (
          <p className="text-sm text-red-500">{loadError}</p>
        ) : !backups ? (
          <p className="flex items-center gap-2 text-sm text-muted-foreground">
            <Loader2 className="h-4 w-4 animate-spin" />
            Loading backups...
          </p>
        ) : (
          <>
            {backups.backups.length === 0 ? (
              <p className="text-sm text-muted-foreground">No backups yet.</p>
            ) : (
              <ul className="divide-y rounded-md border">
                {backups.backups.map((backup) => (
                  <li
                    key={backup.name}
                    className="flex flex-wrap items-center justify-between gap-x-4 gap-y-1 px-3 py-2"
                  >
                    <span className="break-all font-mono text-sm">{backup.name}</span>
                    <span className="text-xs text-muted-foreground">
                      {new Date(backup.created_at).toLocaleString()} · {formatDownloadSize(backup.size)}
                    </span>
                  </li>
                ))}
              </ul>
            )}
            <p className="break-all text-xs text-muted-foreground">Saved in {backups.folder}</p>
          </>
        )}
      </div>
    </Card>
  );
};
