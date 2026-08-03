"use client";

import { ReturnErrorMessage } from "@/services/api-error-return";
import { GetAllJobs, type JobInfo } from "@/services/jobs/get";
import { RunJob } from "@/services/jobs/run";
import { toast } from "sonner";

import { useEffect, useState } from "react";

import { ErrorMessage } from "@/components/shared/error-message";
import Loader from "@/components/shared/loader";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader } from "@/components/ui/card";
import { Small } from "@/components/ui/typography";

import { cn } from "@/lib/cn";

import type { APIResponse } from "@/types/api/api-response";

export default function JobsPage() {
  // States - Loading & Error
  const [loading, setLoading] = useState<boolean>(true);
  const [error, setError] = useState<APIResponse<JobInfo[]> | null>(null);

  // States - Data
  const [jobs, setJobs] = useState<JobInfo[]>([]);

  const fetchJobs = async () => {
    try {
      setLoading(true);

      const response = await GetAllJobs();
      if (response.status === "error") {
        setError(ReturnErrorMessage<JobInfo[]>(response.error?.message || "Failed to fetch jobs"));
        setJobs([]);
        return;
      }

      const items = response.data?.jobs || [];

      // Sort jobs by next_run date ascending
      items.sort((a, b) => {
        const dateA = new Date(a.next_run).getTime() || Infinity;
        const dateB = new Date(b.next_run).getTime() || Infinity;
        return dateA - dateB;
      });

      setJobs(items);
      setError(null);
    } catch (e) {
      setError(ReturnErrorMessage<JobInfo[]>(e));
      setJobs([]);
    } finally {
      setLoading(false);
    }
  };

  const handleRunJobNowClick = async (job: JobInfo) => {
    const response = await RunJob(job.job_name, job.id);
    if (response.status === "error") {
      setError(ReturnErrorMessage<JobInfo[]>(response.error?.message || "Failed to trigger job"));
    } else {
      toast.success(`Job "${job.job_name}" triggered successfully`);
      void fetchJobs();
    }
  };

  useEffect(() => {
    void fetchJobs();
  }, []);

  return (
    <div className="container mx-auto px-2 py-4 min-h-screen flex flex-col items-center">
      <Card className="w-full">
        <CardHeader>
          <div className="flex flex-col md:flex-row md:items-center md:justify-between w-full space-y-2 md:space-y-0">
            <h2 className="text-lg font-medium text-center md:text-left whitespace-nowrap">Scheduled Jobs</h2>

            <div className="flex flex-row flex-wrap justify-center md:justify-end gap-2 px-2 w-full max-w-full">
              <Button variant="outline" onClick={fetchJobs} disabled={loading}>
                Refresh
              </Button>
            </div>
          </div>
        </CardHeader>

        <CardContent>
          {loading ? (
            <Loader />
          ) : error ? (
            <ErrorMessage error={error} />
          ) : (!jobs || jobs.length === 0) && !error && !loading ? (
            <div className="w-full">
              <ErrorMessage error={ReturnErrorMessage<string>("No jobs found")} />
            </div>
          ) : (
            <div className="space-y-2">
              {jobs.map((job) => (
                <Card key={job.id} className="w-full">
                  <CardHeader className={cn("gap-0 mb-0")}>
                    <div className="flex flex-col">
                      <div className="flex flex-row items-start justify-between mb-1 w-full gap-2">
                        <div className="flex flex-col">
                          <div className="text-base font-medium">{job.job_name || "Unknown Job"}</div>
                        </div>

                        <div className="flex flex-col items-end gap-2 max-w-[60%]">
                          {job.spec ? (
                            <code className="px-2 py-1 rounded bg-muted text-xs break-all text-right w-full">
                              {job.spec}
                            </code>
                          ) : (
                            <code className="px-2 py-1 rounded bg-muted text-xs">—</code>
                          )}

                          <Button
                            variant="secondary"
                            size="sm"
                            onClick={() => handleRunJobNowClick(job)}
                            disabled={loading}
                            className="w-fit"
                          >
                            Run Now
                          </Button>
                        </div>
                      </div>

                      <div className="flex flex-col gap-1">
                        <Small className="text-sm text-muted-foreground">
                          Next Run: <span className="font-mono">{job.next_run || "—"}</span>
                        </Small>
                        {job.prev_run && (
                          <Small className="text-sm text-muted-foreground">
                            Prev Run: <span className="font-mono">{job.prev_run || "—"}</span>
                          </Small>
                        )}
                      </div>
                    </div>
                  </CardHeader>
                </Card>
              ))}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
