"use client";

import { formatExactDateTime } from "@/helper/format-date-last-updates";
import { makePlural } from "@/helper/make_plural";
import { ReturnErrorMessage } from "@/services/api-error-return";
import { GetAllDownloadQueueItems } from "@/services/downloads/queue-get";
import { RemoveItemFromQueue } from "@/services/downloads/queue-remove";
import { RetryItemInQueue } from "@/services/downloads/queue-retry";
import type { GetDownloadQueueStatus_Response } from "@/services/downloads/queue-status";
import { GetDownloadQueueStatus } from "@/services/downloads/queue-status";
import { Globe, RefreshCcw, Trash2 } from "lucide-react";
import { toast } from "sonner";

import React, { useCallback, useEffect, useRef, useState } from "react";

import { ConfirmDestructiveDialogActionButton } from "@/components/shared/dialog-destructive-action";
import DownloadQueueEntry from "@/components/shared/download-queue-entry";
import { ErrorMessage } from "@/components/shared/error-message";
import Loader from "@/components/shared/loader";
import { RefreshButton } from "@/components/shared/refresh-button";
import { ResponsiveGrid } from "@/components/shared/responsive-grid";
import { Accordion, AccordionContent, AccordionItem, AccordionTrigger } from "@/components/ui/accordion";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { H2, H3 } from "@/components/ui/typography";

import { cn } from "@/lib/cn";

import type { APIResponse } from "@/types/api/api-response";
import type { DBSavedItem } from "@/types/database/db-poster-set";

// Stable key for a queue entry, matching the backend's file identity (LibraryTitle + TMDB ID).
const errorEntryKey = (entry: DBSavedItem) => `${entry.media_item.tmdb_id}|||${entry.media_item.library_title}`;

const DownloadQueuePage: React.FC = () => {
  // Refs - Fetching
  const isFetchingRef = useRef(false);

  // States - Queue Entries
  const [inProgressEntries, setInProgressEntries] = useState<DBSavedItem[]>([]);
  const [errorEntries, setErrorEntries] = useState<DBSavedItem[]>([]);
  const [warningEntries, setWarningEntries] = useState<DBSavedItem[]>([]);

  // States - Queue Status
  const [queueStatus, setQueueStatus] = useState<GetDownloadQueueStatus_Response>({
    time: "",
    status: "",
    message: "",
    warnings: [],
    errors: [],
  });
  const [secondsToNextRun, setSecondsToNextRun] = useState<number>(0);

  // States - Loading & Error
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<APIResponse<unknown> | null>(null);

  // States - Bulk edit (Error section only)
  const [bulkMode, setBulkMode] = useState(false);
  const [selectedKeys, setSelectedKeys] = useState<Set<string>>(new Set());
  const [bulkActionRunning, setBulkActionRunning] = useState(false);

  // Fetch Queue Entries
  const fetchQueueEntries = useCallback(async () => {
    if (isFetchingRef.current) return;
    isFetchingRef.current = true;

    try {
      setLoading(true);

      const response = await GetAllDownloadQueueItems();

      if (response.status === "error") {
        setError(response);
        return;
      }

      setInProgressEntries(response.data?.in_progress_entries || []);
      setErrorEntries(response.data?.error_entries || []);
      setWarningEntries(response.data?.warning_entries || []);
      setError(null);
    } catch (error) {
      setError(ReturnErrorMessage<unknown>(error));
    } finally {
      isFetchingRef.current = false;
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchQueueEntries();
  }, [fetchQueueEntries, queueStatus.status]);

  // --- Bulk edit helpers (Error section) ---

  // Exit bulk mode automatically once there are no error entries left to act on.
  useEffect(() => {
    if (errorEntries.length === 0 && bulkMode) {
      setBulkMode(false);
      setSelectedKeys(new Set());
    }
  }, [errorEntries.length, bulkMode]);

  const toggleBulkMode = () => {
    setSelectedKeys(new Set());
    setBulkMode((prev) => !prev);
  };

  const toggleSelected = (key: string, checked: boolean) => {
    setSelectedKeys((prev) => {
      const next = new Set(prev);
      if (checked) {
        next.add(key);
      } else {
        next.delete(key);
      }
      return next;
    });
  };

  // Deduplicate by backend identity so each media item is acted on once even if
  // multiple error files exist for it.
  const getSelectedErrorEntries = (): DBSavedItem[] => {
    const byKey = new Map<string, DBSavedItem>();
    for (const entry of errorEntries) {
      const key = errorEntryKey(entry);
      if (selectedKeys.has(key) && !byKey.has(key)) {
        byKey.set(key, entry);
      }
    }
    return Array.from(byKey.values());
  };

  const handleBulkRetry = async () => {
    const entries = getSelectedErrorEntries();
    if (entries.length === 0) return;

    setBulkActionRunning(true);
    const toastId = "bulk-retry-error-entries";
    toast.loading(`Retrying 0 of ${entries.length}...`, { id: toastId });

    let succeeded = 0;
    const failures: string[] = [];
    for (const [index, entry] of entries.entries()) {
      toast.loading(`Retrying ${index + 1} of ${entries.length} - ${entry.media_item.title}`, { id: toastId });
      const response = await RetryItemInQueue(entry);
      if (response.status === "error") {
        failures.push(entry.media_item.title);
      } else {
        succeeded++;
      }
    }

    if (failures.length === 0) {
      toast.success(`Retried ${succeeded} ${makePlural(succeeded, "item")}`, { id: toastId });
    } else {
      toast.error(`Retried ${succeeded} of ${entries.length}; failed: ${failures.join(", ")}`, { id: toastId });
    }

    setBulkMode(false);
    setSelectedKeys(new Set());
    setBulkActionRunning(false);
    await fetchQueueEntries();
  };

  const handleBulkDelete = async () => {
    const entries = getSelectedErrorEntries();
    if (entries.length === 0) return;

    setBulkActionRunning(true);
    const toastId = "bulk-delete-error-entries";
    toast.loading(`Deleting 0 of ${entries.length}...`, { id: toastId });

    let succeeded = 0;
    const failures: string[] = [];
    for (const [index, entry] of entries.entries()) {
      toast.loading(`Deleting ${index + 1} of ${entries.length} - ${entry.media_item.title}`, { id: toastId });
      const safeEntry: DBSavedItem = {
        ...entry,
        poster_sets: Array.isArray(entry.poster_sets) ? entry.poster_sets : [],
      };
      const response = await RemoveItemFromQueue(safeEntry);
      if (response.status === "error") {
        failures.push(entry.media_item.title);
      } else {
        succeeded++;
      }
    }

    if (failures.length === 0) {
      toast.success(`Deleted ${succeeded} ${makePlural(succeeded, "item")}`, { id: toastId });
    } else {
      toast.error(`Deleted ${succeeded} of ${entries.length}; failed: ${failures.join(", ")}`, { id: toastId });
    }

    setBulkMode(false);
    setSelectedKeys(new Set());
    setBulkActionRunning(false);
    await fetchQueueEntries();
  };

  useEffect(() => {
    const fetchStatus = async () => {
      try {
        const statusResponse = await GetDownloadQueueStatus();
        if (statusResponse.status === "error") {
          throw new Error("Error fetching status");
        }
        const status = statusResponse.data || {
          time: new Date().toISOString(),
          status: "Error",
          message: "Unable to get status from server",
          warnings: [],
          errors: ["No status data"],
        };
        setQueueStatus(status);
      } catch {
        const errorResponse: GetDownloadQueueStatus_Response = {
          time: new Date().toISOString(),
          status: "Error",
          message: "Failed to fetch status",
          warnings: [],
          errors: [],
        };
        setQueueStatus(errorResponse);
      }
    };

    fetchStatus();
    const interval = setInterval(fetchStatus, 1000 * 2); // Refresh every 2 seconds

    return () => clearInterval(interval);
  }, []);

  useEffect(() => {
    const updateNextRunTime = () => {
      const now = new Date();
      const next = new Date(now);
      next.setSeconds(0, 0);
      if (now.getSeconds() !== 0 || now.getMilliseconds() !== 0) {
        next.setMinutes(now.getMinutes() + 1);
      }

      const diff = Math.max(0, Math.floor((next.getTime() - now.getTime()) / 1000));
      setSecondsToNextRun(diff);
    };

    updateNextRunTime();
    const interval = setInterval(updateNextRunTime, 1000);

    return () => clearInterval(interval);
  }, []);

  if (loading) {
    return <Loader className="mt-10" message="Loading download queue entries..." />;
  }

  if (error) {
    return (
      <div className="flex flex-col items-center p-6 gap-4">
        <ErrorMessage error={error} />
      </div>
    );
  }

  const defaultAccordionValues = [
    inProgressEntries.length > 0 ? "in_progress" : null,
    errorEntries.length > 0 ? "error_entries" : null,
    warningEntries.length > 0 ? "warning_entries" : null,
  ].filter(Boolean) as string[];

  // Derived bulk-selection state for the Error section.
  const uniqueErrorKeys = Array.from(new Set(errorEntries.map(errorEntryKey)));
  const selectedErrorCount = uniqueErrorKeys.filter((key) => selectedKeys.has(key)).length;
  const allErrorsSelected = uniqueErrorKeys.length > 0 && selectedErrorCount === uniqueErrorKeys.length;

  return (
    <div className="container mx-auto p-4 min-h-screen flex flex-col items-center">
      <H2 className="text-3xl font-bold mb-4">Download Queue</H2>

      {typeof secondsToNextRun === "number" && (
        <div className="w-full max-w-4xl mb-2 text-xs text-muted-foreground text-right flex items-center justify-end gap-2">
          <span className="font-mono">Next Run: {secondsToNextRun}s</span>
          <span title="HTTP Polling">
            <Globe className="inline-block h-4 w-4 text-blue-500" />
          </span>
        </div>
      )}
      <pre
        className={cn(
          "w-full max-w-4xl mb-4 p-3 rounded text-xs whitespace-pre-wrap border",
          queueStatus.status === "Error"
            ? "border-red-400 text-red-500"
            : queueStatus.status === "Warning"
              ? "border-yellow-400 text-yellow-500"
              : queueStatus.status === "Success"
                ? "border-green-400 text-green-500"
                : queueStatus.status === "Idle - Queue Empty"
                  ? "border-gray-400 text-gray-500"
                  : "border-primary text-primary"
        )}
      >
        {queueStatus.time && (
          <div>
            <b>Last Run:</b> {formatExactDateTime(queueStatus.time)}
          </div>
        )}
        {queueStatus.status && (
          <div>
            <b>Status:</b> {queueStatus.status}
          </div>
        )}
        {queueStatus.message && <div className="mt-2 mb-2">{queueStatus.message}</div>}
        {queueStatus.warnings && queueStatus.warnings.length > 0 && (
          <div className="mt-1">
            <b className="text-yellow-500">Warnings:</b>
            <ul className="list-disc ml-5 text-yellow-500">
              {queueStatus.warnings.map((w, i) => (
                <li key={i}>{w}</li>
              ))}
            </ul>
          </div>
        )}
        {queueStatus.errors && queueStatus.errors.length > 0 && (
          <div className="mt-1">
            <b className="text-red-500">Errors:</b>
            <ul className="list-disc ml-5 text-red-500">
              {queueStatus.errors.map((e, i) => (
                <li key={i}>{e}</li>
              ))}
            </ul>
          </div>
        )}
      </pre>

      {inProgressEntries.length === 0 && errorEntries.length === 0 && warningEntries.length === 0 && (
        <p className="text-gray-500">No download queue entries found</p>
      )}

      <div className="w-full">
        <Accordion type="multiple" className="mb-4" defaultValue={defaultAccordionValues}>
          {inProgressEntries.length > 0 && (
            <AccordionItem value="in_progress">
              <AccordionTrigger
                className={cn(
                  "cursor-pointer",
                  "hover:underline-none focus:underline-none underline-none hover:no-underline focus:no-underline justify-center"
                )}
              >
                <H3>In Progress Entries</H3>
              </AccordionTrigger>
              <AccordionContent>
                {inProgressEntries.length === 0 ? (
                  <p className="text-gray-500">No entries in progress.</p>
                ) : (
                  <ResponsiveGrid size="regular">
                    {inProgressEntries.map((entry) => (
                      <DownloadQueueEntry
                        key={entry.media_item.tmdb_id}
                        entry={entry}
                        fetchQueueEntries={fetchQueueEntries}
                      />
                    ))}
                  </ResponsiveGrid>
                )}
              </AccordionContent>
            </AccordionItem>
          )}

          {errorEntries.length > 0 && (
            <AccordionItem value="error_entries">
              <AccordionTrigger
                className={cn(
                  "cursor-pointer",
                  "hover:underline-none focus:underline-none underline-none hover:no-underline focus:no-underline justify-center"
                )}
              >
                <H3>Error Entries</H3>
              </AccordionTrigger>
              <AccordionContent>
                {errorEntries.length === 0 ? (
                  <p className="text-gray-500">No error entries.</p>
                ) : (
                  <>
                    {/* Bulk edit toggle + action bar */}
                    <div className="mb-3 flex flex-col gap-2">
                      <div className="flex justify-end">
                        <Button
                          variant={bulkMode ? "destructive" : "secondary"}
                          onClick={toggleBulkMode}
                          disabled={bulkActionRunning}
                          className={cn(
                            "flex items-center gap-1 text-xs sm:text-sm cursor-pointer",
                            bulkMode
                              ? "border-red-600 bg-red-600/10 hover:bg-red-600/20"
                              : "border border-1 border-yellow-500 hover:bg-yellow-800"
                          )}
                        >
                          {bulkMode ? "Cancel Bulk Edit" : "Bulk Edit"}
                        </Button>
                      </div>

                      {bulkMode && (
                        <div className="flex flex-wrap items-center gap-3 rounded-md border p-3">
                          <div className="flex items-center gap-2 select-none">
                            <Checkbox
                              id="select-all-error-entries"
                              checked={allErrorsSelected}
                              onCheckedChange={(checked) =>
                                setSelectedKeys(checked ? new Set(uniqueErrorKeys) : new Set())
                              }
                              disabled={bulkActionRunning}
                              aria-label={allErrorsSelected ? "Deselect all error entries" : "Select all error entries"}
                              className="h-5 w-5 border-1 border-primary cursor-pointer"
                            />
                            <label
                              htmlFor="select-all-error-entries"
                              className={cn("text-sm", bulkActionRunning ? "cursor-not-allowed" : "cursor-pointer")}
                            >
                              {allErrorsSelected ? "Deselect All" : "Select All"}
                            </label>
                          </div>

                          <span className="text-sm text-muted-foreground">
                            {selectedErrorCount} of {uniqueErrorKeys.length} selected
                          </span>

                          <div className="ml-auto flex items-center gap-2">
                            <Button
                              variant="secondary"
                              onClick={handleBulkRetry}
                              disabled={selectedErrorCount === 0 || bulkActionRunning}
                              className="flex items-center gap-1 text-xs sm:text-sm cursor-pointer"
                            >
                              <RefreshCcw className="h-4 w-4" />
                              Retry Selected
                            </Button>
                            <ConfirmDestructiveDialogActionButton
                              variant="outline"
                              className="flex items-center gap-1 text-destructive border-1 shadow-none hover:text-red-500 cursor-pointer text-xs sm:text-sm disabled:opacity-50"
                              disabled={selectedErrorCount === 0 || bulkActionRunning}
                              confirmText="Delete Selected"
                              title={`Delete ${selectedErrorCount} selected error ${makePlural(selectedErrorCount, "item")}?`}
                              description="Are you sure you want to delete the selected error entries from the download queue? This action cannot be undone."
                              onConfirm={handleBulkDelete}
                            >
                              <Trash2 className="h-4 w-4" />
                              Delete Selected
                            </ConfirmDestructiveDialogActionButton>
                          </div>
                        </div>
                      )}
                    </div>

                    <ResponsiveGrid size="regular">
                      {errorEntries.map((entry, index) => {
                        const key = errorEntryKey(entry);
                        return (
                          <DownloadQueueEntry
                            key={`${key}-${index}`}
                            entry={entry}
                            fetchQueueEntries={fetchQueueEntries}
                            selectable={bulkMode}
                            selected={selectedKeys.has(key)}
                            onToggleSelected={(checked) => toggleSelected(key, checked)}
                          />
                        );
                      })}
                    </ResponsiveGrid>
                  </>
                )}
              </AccordionContent>
            </AccordionItem>
          )}

          {warningEntries.length > 0 && (
            <AccordionItem value="warning_entries">
              <AccordionTrigger
                className={cn(
                  "cursor-pointer",
                  "hover:underline-none focus:underline-none underline-none hover:no-underline focus:no-underline justify-center"
                )}
              >
                <H3>Warning Entries</H3>
              </AccordionTrigger>
              <AccordionContent>
                {warningEntries.length === 0 ? (
                  <p className="text-gray-500">No warning entries.</p>
                ) : (
                  <ResponsiveGrid size="larger">
                    {warningEntries.map((entry) => (
                      <DownloadQueueEntry
                        key={entry.media_item.tmdb_id}
                        entry={entry}
                        fetchQueueEntries={fetchQueueEntries}
                      />
                    ))}
                  </ResponsiveGrid>
                )}
              </AccordionContent>
            </AccordionItem>
          )}
        </Accordion>
      </div>

      {/* Refresh Button */}
      <RefreshButton onClick={() => fetchQueueEntries()} />
    </div>
  );
};

export default DownloadQueuePage;
