"use client";

import { ClearLogFiles } from "@/services/logs/clear";
import { toast } from "sonner";

import React, { useEffect, useRef } from "react";

import { useRouter } from "next/navigation";

import { ConfirmDestructiveDialogActionButton } from "@/components/shared/dialog-destructive-action";
import { NumberInput } from "@/components/shared/number-input";
import { PopoverHelp } from "@/components/shared/popover-help";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";

import { cn } from "@/lib/cn";

import type { AppConfigLogging } from "@/types/config/config";

interface ConfigSectionLoggingProps {
  value: AppConfigLogging;
  editing: boolean;
  showActions?: boolean; // Show actions hidden while editing, as Settings is always editable
  dirtyFields?: Partial<Record<keyof AppConfigLogging, boolean>>;
  onChange: <K extends keyof AppConfigLogging>(field: K, value: AppConfigLogging[K]) => void;
  errorsUpdate?: (errors: Partial<Record<keyof AppConfigLogging, string>>) => void;
}

const LOG_LEVEL_OPTIONS = ["TRACE", "DEBUG", "INFO", "WARN", "ERROR"] as const;

export const ConfigSectionLogging: React.FC<ConfigSectionLoggingProps> = ({
  value,
  editing,
  showActions = false,
  dirtyFields = {},
  onChange,
  errorsUpdate,
}) => {
  const router = useRouter();
  const prevErrorsRef = useRef<string>("");

  const errors = React.useMemo(() => {
    const errs: Partial<Record<keyof AppConfigLogging, string>> = {};
    if (!value.level?.trim()) errs.level = "Level is required.";
    else if (!LOG_LEVEL_OPTIONS.includes(value.level as (typeof LOG_LEVEL_OPTIONS)[number]))
      errs.level = "Invalid log level.";
    return errs;
  }, [value.level]);

  useEffect(() => {
    if (!errorsUpdate) return;
    const ser = JSON.stringify(errors);
    if (ser === prevErrorsRef.current) return;
    prevErrorsRef.current = ser;
    errorsUpdate(errors);
  }, [errors, errorsUpdate]);

  const handleViewLogs = () => {
    router.push("/logs");
  };

  const handleClearOldLogs = async () => {
    try {
      const response = await ClearLogFiles();
      if (response.status === "error") {
        toast.error(response.error?.message || "Failed to clear old logs");
        return;
      }
      toast.success(response.data?.message || "Old logs cleared successfully");
    } catch {
      toast.error("An unexpected error occurred");
    }
  };

  // If no level is set, default to INFO
  useEffect(() => {
    if (!value.level) {
      onChange("level", "INFO");
    }
  }, [value.level, onChange]);

  return (
    <Card className={`gap-4 p-4 sm:p-5 ${Object.values(errors).some(Boolean) ? "border-red-500" : "border-muted"}`}>
      <h2 className="text-xl font-semibold text-blue-500">Logging</h2>

      {/* Level */}
      <div className={cn("space-y-1 border rounded-md p-3 transition")}>
        <div className="flex items-center justify-between">
          <Label>Level</Label>
          {editing && (
            <PopoverHelp ariaLabel="help-logging-level">
              <p className="mb-2">Select verbosity of application logs.</p>
              <ul className="space-y-2 text-xs">
                <li>
                  <span className="font-mono">TRACE</span> – detailed tracing information
                </li>
                <li>
                  <span className="font-mono">DEBUG</span> – detailed development info
                </li>
                <li>
                  <span className="font-mono">INFO</span> – high-level operational events
                </li>
                <li>
                  <span className="font-mono">WARN</span> – unexpected but non-fatal issues
                </li>
                <li>
                  <span className="font-mono">ERROR</span> – failures requiring attention
                </li>
              </ul>
            </PopoverHelp>
          )}
        </div>
        <Select
          disabled={!editing}
          value={value.level}
          onValueChange={(v) => onChange("level", v as AppConfigLogging["level"])}
        >
          <SelectTrigger id="logging-level-trigger" className={cn("w-full", dirtyFields.level && "border-amber-500")}>
            <SelectValue placeholder="Select level..." />
          </SelectTrigger>
          <SelectContent>
            {LOG_LEVEL_OPTIONS.map((lvl) => (
              <SelectItem key={lvl} value={lvl}>
                {lvl}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        {errors.level && <p className="text-xs text-red-500">{errors.level}</p>}
      </div>

      {/* File (read-only) */}
      <div className="space-y-1 border rounded-md p-3">
        <div className="flex items-center justify-between mb-1">
          <Label>Log File</Label>
          <div className="flex items-center gap-2">
            <Button
              hidden={editing && !showActions}
              variant="outline"
              onClick={handleViewLogs}
              className="cursor-pointer hover:text-primary"
            >
              View
            </Button>
            <ConfirmDestructiveDialogActionButton
              hidden={editing && !showActions}
              onConfirm={handleClearOldLogs}
              title="Clear Old Logs?"
              description="This will permanently delete all old log files. Are you sure you want to continue?"
              confirmText="Yes, Clear Logs"
              cancelText="Cancel"
              className="text-destructive border-1 shadow-none hover:text-red-500 cursor-pointer"
            >
              Clear Old Logs
            </ConfirmDestructiveDialogActionButton>
          </div>
        </div>
        <Input disabled value={value.file || ""} placeholder="aura.log" />
      </div>

      {/* Keep Old Logs */}
      <div className="grid gap-3 sm:grid-cols-2">
        <div className={cn("space-y-1 border rounded-md p-3", dirtyFields.keep_days && "border-amber-500")}>
          <div className="flex items-center justify-between mb-1">
            <Label htmlFor="logging-keep-days">Keep Old Logs (days)</Label>
            {editing && (
              <PopoverHelp ariaLabel="help-logging-keep-days">
                <p>Old log files are deleted after this many days.</p>
              </PopoverHelp>
            )}
          </div>
          <NumberInput
            id="logging-keep-days"
            disabled={!editing}
            value={value.keep_days}
            onChange={(days) => onChange("keep_days", days)}
            placeholder="7"
            min={1}
            max={365}
          />
        </div>

        <div className={cn("space-y-1 border rounded-md p-3", dirtyFields.keep_files && "border-amber-500")}>
          <div className="flex items-center justify-between mb-1">
            <Label htmlFor="logging-keep-files">Keep Old Logs (files)</Label>
            {editing && (
              <PopoverHelp ariaLabel="help-logging-keep-files">
                <p className="mb-2">Only this many old log files are kept. The oldest are deleted first.</p>
                <p className="text-xs text-muted-foreground">Old files are deleted when either limit is reached.</p>
              </PopoverHelp>
            )}
          </div>
          <NumberInput
            id="logging-keep-files"
            disabled={!editing}
            value={value.keep_files}
            onChange={(files) => onChange("keep_files", files)}
            placeholder="7"
            min={1}
            max={100}
          />
        </div>
      </div>
    </Card>
  );
};
