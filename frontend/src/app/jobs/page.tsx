"use client";

import { ReturnErrorMessage } from "@/services/api-error-return";
import { GetAppConfigStatus } from "@/services/config/status";
import type { AppConfigSection } from "@/services/config/update-section";
import { GetAllJobs, type JobInfo } from "@/services/jobs/get";
import { RunJob } from "@/services/jobs/run";
import cronstrue from "cronstrue";
import { CalendarClock, History, Loader2, Play, RefreshCcw } from "lucide-react";
import { toast } from "sonner";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import Link from "next/link";

import { ConfigSaveStatus } from "@/components/settings-onboarding/ConfigSaveStatus";
import { ErrorMessage } from "@/components/shared/error-message";
import Loader from "@/components/shared/loader";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";

import { cn } from "@/lib/cn";

import { useConfigAutosave } from "@/hooks/use-config-autosave";

import type { APIResponse } from "@/types/api/api-response";
import type { AppConfig, AppConfigJobSetting, AppConfigJobs } from "@/types/config/config";
import { defaultAppConfig } from "@/types/config/config-default-app";

const AUTOSAVE_SECTIONS: readonly AppConfigSection[] = ["jobs"];

const validateCron = (expr: string): string | null => {
  const trimmed = expr.trim();
  if (!trimmed) return "A schedule is needed while the job is on.";
  try {
    cronstrue.toString(trimmed);
    return null;
  } catch {
    return "Not a valid cron expression.";
  }
};

// describeCron returns the schedule in words, e.g. "At 04:00 AM"
const describeCron = (expr: string): string | null => {
  try {
    return cronstrue.toString(expr.trim());
  } catch {
    return null;
  }
};

// withJobValues fills in the schedules a job uses when its settings are left at the default
const withJobValues = (cfg: AppConfig, jobs: JobInfo[]): AppConfig => {
  const filled = { ...cfg.jobs } as Record<string, AppConfigJobSetting>;
  for (const job of jobs) {
    const setting = filled[job.id] ?? { cron: "" };
    filled[job.id] = { enabled: setting.enabled ?? job.enabled, cron: setting.cron || job.spec };
  }
  return { ...cfg, jobs: filled as unknown as AppConfigJobs };
};

// The backend formats timestamps as "YYYY-MM-DD HH:MM:SS" with no timezone
// suffix. Normalizing to an ISO-ish "T" separator makes Date parsing reliable
// across browsers (notably Safari, which can otherwise reject the space form).
const parseServerTimestamp = (value: string): number => {
  if (!value) return NaN;
  return new Date(value.replace(" ", "T")).getTime();
};

// Renders a millisecond duration as an at-a-glance label: "45s", "5 min", "6
// hours", "2 days".
const formatDuration = (ms: number): string => {
  const seconds = Math.floor(ms / 1000);
  if (seconds < 60) return `${seconds}s`;

  const minutes = Math.floor(ms / (60 * 1000));
  if (minutes < 60) return `${minutes} min`;

  const hours = Math.floor(ms / (60 * 60 * 1000));
  if (hours < 24) return `${hours} ${hours === 1 ? "hour" : "hours"}`;

  const days = Math.floor(ms / (24 * 60 * 60 * 1000));
  return `${days} ${days === 1 ? "day" : "days"}`;
};

const formatNextRun = (targetMs: number, nowMs: number): string => {
  const diffMs = targetMs - nowMs;
  if (diffMs <= 0) return "due now";
  return `in ${formatDuration(diffMs)}`;
};

const formatPrevRun = (targetMs: number, nowMs: number): string => {
  const diffMs = nowMs - targetMs;
  if (diffMs < 60 * 1000) return "just now";
  return `${formatDuration(diffMs)} ago`;
};

export default function JobsPage() {
  // States - Loading & Error
  const [loading, setLoading] = useState<boolean>(true);
  const [error, setError] = useState<APIResponse<JobInfo[]> | null>(null);
  const [runningJobId, setRunningJobId] = useState<string | null>(null);

  // States - Data
  const [jobs, setJobs] = useState<JobInfo[]>([]);
  const [savedConfig, setSavedConfig] = useState<AppConfig>(() => defaultAppConfig());
  const [draftConfig, setDraftConfig] = useState<AppConfig>(() => defaultAppConfig());

  // Ticks once a second so "Next Run" labels (and the under-a-minute countdown)
  // stay live without needing to refetch from the server.
  const [now, setNow] = useState<number>(() => Date.now());
  useEffect(() => {
    const interval = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(interval);
  }, []);

  // The saved jobs the run times were last loaded for
  const lastSavedJobs = useRef("");

  // Get the jobs again, keeping their order so a card doesn't move while it is being edited
  const jobOrder = useRef<string[]>([]);
  const fetchJobs = useCallback(async (): Promise<JobInfo[] | null> => {
    const response = await GetAllJobs();
    if (response.status === "error") {
      setError(ReturnErrorMessage<JobInfo[]>(response.error?.message || "Failed to fetch jobs"));
      return null;
    }
    const items = response.data?.jobs || [];
    if (jobOrder.current.length === 0) {
      // Soonest next run first
      items.sort((a, b) => {
        const dateA = parseServerTimestamp(a.next_run) || Infinity;
        const dateB = parseServerTimestamp(b.next_run) || Infinity;
        return dateA - dateB;
      });
      jobOrder.current = items.map((job) => job.id);
    } else {
      const position = (id: string) => {
        const index = jobOrder.current.indexOf(id);
        return index === -1 ? Infinity : index;
      };
      items.sort((a, b) => position(a.id) - position(b.id));
    }
    setJobs(items);
    setError(null);
    return items;
  }, []);

  const loadPage = useCallback(async () => {
    setLoading(true);
    const configResponse = await GetAppConfigStatus();
    if (configResponse.status === "error") {
      setError(ReturnErrorMessage<JobInfo[]>(configResponse.error?.message || "Failed to fetch configuration"));
      setLoading(false);
      return;
    }
    const items = await fetchJobs();
    const cfg = withJobValues(configResponse.data?.status.current_setup ?? defaultAppConfig(), items ?? []);
    lastSavedJobs.current = JSON.stringify(cfg.jobs);
    setSavedConfig(cfg);
    setDraftConfig(cfg);
    setLoading(false);
  }, [fetchJobs]);

  useEffect(() => {
    void loadPage();
  }, [loadPage]);

  // A job that isn't on has no schedule to check
  const validationErrors = useMemo(() => {
    const errors: Record<string, string> = {};
    for (const [id, setting] of Object.entries(draftConfig.jobs ?? {}) as [string, AppConfigJobSetting][]) {
      const cronErr = setting.enabled ? validateCron(setting.cron) : null;
      if (cronErr) errors[id] = cronErr;
    }
    return { jobs: errors };
  }, [draftConfig.jobs]);

  // Saved messages name the job, e.g. "AutoDownload Job turned off"
  const jobSettingLabel = useCallback(
    (section: AppConfigSection, path: string) => {
      if (section !== "jobs") return undefined;
      const [id, field] = path.split(".");
      const name = jobs.find((job) => job.id === id)?.job_name;
      if (!name) return undefined;
      return field === "cron" ? `${name} schedule` : name;
    },
    [jobs]
  );

  // The saved jobs section leaves out settings at their default, so fill them in again
  const jobsRef = useRef<JobInfo[]>([]);
  jobsRef.current = jobs;
  const fillJobValues = useCallback(
    (set: React.Dispatch<React.SetStateAction<AppConfig>>): React.Dispatch<React.SetStateAction<AppConfig>> =>
      (update) =>
        set((prev) => withJobValues(typeof update === "function" ? update(prev) : update, jobsRef.current)),
    []
  );
  const setSavedJobsConfig = useMemo(() => fillJobValues(setSavedConfig), [fillJobValues]);
  const setDraftJobsConfig = useMemo(() => fillJobValues(setDraftConfig), [fillJobValues]);

  const { saveState, isBlockedByErrors, retry } = useConfigAutosave({
    sections: AUTOSAVE_SECTIONS,
    savedConfig,
    draftConfig,
    setSavedConfig: setSavedJobsConfig,
    setDraftConfig: setDraftJobsConfig,
    validationErrors,
    settingLabel: jobSettingLabel,
  });

  // Get the next run times again once a schedule is saved
  const savedJobs = JSON.stringify(savedConfig.jobs ?? {});
  useEffect(() => {
    if (loading || lastSavedJobs.current === savedJobs) return;
    lastSavedJobs.current = savedJobs;
    void fetchJobs();
  }, [savedJobs, loading, fetchJobs]);

  const updateJob = <K extends keyof AppConfigJobSetting>(jobId: string, field: K, value: AppConfigJobSetting[K]) => {
    setDraftConfig((prev) => ({
      ...prev,
      jobs: {
        ...prev.jobs,
        [jobId]: { ...prev.jobs[jobId as keyof AppConfigJobs], [field]: value },
      },
    }));
  };

  const handleRunJobNowClick = async (job: JobInfo) => {
    setRunningJobId(job.id);
    const response = await RunJob(job.id);
    setRunningJobId(null);
    if (response.status === "error") {
      toast.error(response.error?.message || `Failed to run "${job.job_name}"`);
    } else {
      toast.success(`"${job.job_name}" started`);
      void fetchJobs();
    }
  };

  const handleRefreshClick = async () => {
    setLoading(true);
    await fetchJobs();
    setLoading(false);
  };

  return (
    <div className="container mx-auto max-w-6xl p-4 sm:p-6 lg:p-8">
      <div className="mb-6 flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div className="space-y-1">
          <h1 className="text-3xl font-bold">Jobs</h1>
          <p className="text-sm text-muted-foreground">
            Background jobs that keep aura up to date. Each job runs on its own schedule, written as a cron expression.
            You can use{" "}
            <Link
              className="text-primary hover:underline"
              href="https://crontab.guru/"
              target="_blank"
              rel="noopener noreferrer"
            >
              crontab.guru
            </Link>{" "}
            to create and test them. Changes save automatically.
          </p>
        </div>
        <div className="flex shrink-0 items-center gap-3 self-start">
          <ConfigSaveStatus
            state={saveState.jobs}
            blockedByErrors={isBlockedByErrors("jobs")}
            onRetry={() => void retry("jobs")}
          />
          <Button
            variant="outline"
            size="sm"
            onClick={handleRefreshClick}
            disabled={loading}
            className="cursor-pointer"
          >
            <RefreshCcw className={cn("h-4 w-4", loading && "animate-spin")} />
            Refresh
          </Button>
        </div>
      </div>

      {loading && jobs.length === 0 ? (
        <Loader />
      ) : error ? (
        <ErrorMessage error={error} />
      ) : jobs.length === 0 ? (
        <ErrorMessage error={ReturnErrorMessage<string>("No jobs found")} />
      ) : (
        <div className="grid gap-4 md:grid-cols-2">
          {jobs.map((job) => {
            const draft = draftConfig.jobs?.[job.id as keyof AppConfigJobs] ?? { enabled: job.enabled, cron: job.spec };
            const saved = savedConfig.jobs?.[job.id as keyof AppConfigJobs];
            const enabled = draft.enabled ?? false;
            const changed = !!saved && (enabled !== (saved.enabled ?? false) || draft.cron !== saved.cron);
            const cronErr = validationErrors.jobs[job.id] ?? null;
            const schedule = !cronErr && enabled ? describeCron(draft.cron) : null;

            const nextRunLabel = job.enabled
              ? job.next_run
                ? formatNextRun(parseServerTimestamp(job.next_run), now)
                : "—"
              : "Off";
            const prevRunLabel = job.prev_run ? formatPrevRun(parseServerTimestamp(job.prev_run), now) : null;

            return (
              <Card key={job.id} className={cn("gap-4 p-4", changed && "border-amber-500")}>
                {/* Name, description and on/off */}
                <div className="flex items-start justify-between gap-3">
                  <div className="min-w-0 space-y-1">
                    <div className="flex items-center gap-2">
                      <h2 className="font-semibold leading-tight">{job.job_name || "Unknown Job"}</h2>
                      {!enabled && <Badge variant="secondary">Off</Badge>}
                    </div>
                    {job.description && <p className="text-sm text-muted-foreground">{job.description}</p>}
                  </div>
                  <Switch
                    checked={enabled}
                    onCheckedChange={(v) => updateJob(job.id, "enabled", v)}
                    aria-label={`Turn ${job.job_name} on or off`}
                    className="mt-0.5 cursor-pointer"
                  />
                </div>

                {/* Schedule */}
                <div className="space-y-1">
                  <Label htmlFor={`job-cron-${job.id}`} className="text-xs text-muted-foreground">
                    Schedule
                  </Label>
                  <Input
                    id={`job-cron-${job.id}`}
                    disabled={!enabled}
                    placeholder="0 3 * * *"
                    value={draft.cron}
                    onChange={(e) => updateJob(job.id, "cron", e.target.value)}
                    className={cn("font-mono", cronErr ? "border-red-500" : changed && "border-amber-500")}
                  />
                  {cronErr ? (
                    <p className="text-xs text-red-500">{cronErr}</p>
                  ) : (
                    schedule && <p className="text-xs text-muted-foreground">{schedule}</p>
                  )}
                </div>

                {/* Next and last run, and Run Now */}
                <div className="mt-auto flex flex-wrap items-center justify-between gap-3 border-t pt-3">
                  <div className="flex flex-wrap gap-x-4 gap-y-1 text-xs text-muted-foreground">
                    <span className="flex items-center gap-1" title={job.next_run || undefined}>
                      <CalendarClock className="h-3.5 w-3.5" />
                      Next: {nextRunLabel}
                    </span>
                    {prevRunLabel && (
                      <span className="flex items-center gap-1" title={job.prev_run || undefined}>
                        <History className="h-3.5 w-3.5" />
                        Last: {prevRunLabel}
                      </span>
                    )}
                  </div>

                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => handleRunJobNowClick(job)}
                    disabled={runningJobId === job.id}
                    className="ml-auto cursor-pointer"
                  >
                    {runningJobId === job.id ? (
                      <Loader2 className="h-4 w-4 animate-spin" />
                    ) : (
                      <Play className="h-4 w-4" />
                    )}
                    Run Now
                  </Button>
                </div>
              </Card>
            );
          })}
        </div>
      )}
    </div>
  );
}
