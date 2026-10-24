"use client";

import { useEffect, useState } from "react";

import { Badge } from "@/components/ui/badge";

interface LoadTimingBadgeProps {
  label: string;
  startedAt: number | null; // performance.now() when loading started
  finishedAt: number | null; // performance.now() when loading finished
  urlMatch: string; // Requests whose URL contains this are counted
}

const formatSize = (bytes: number) =>
  bytes >= 1024 * 1024 ? `${(bytes / 1024 / 1024).toFixed(1)} MB` : `${Math.round(bytes / 1024)} KB`;

// Shows a page's load time and data downloaded, in Debug Mode only
export function LoadTimingBadge({ label, startedAt, finishedAt, urlMatch }: LoadTimingBadgeProps) {
  const [summary, setSummary] = useState<string | null>(null);
  const [debugMode, setDebugMode] = useState(false);

  useEffect(() => {
    try {
      setDebugMode(localStorage.getItem("debugMode") === "true");
      // The browser stops recording requests after 250 by default
      performance.setResourceTimingBufferSize(10000);
    } catch {
      setDebugMode(false);
    }
  }, []);

  useEffect(() => {
    if (startedAt === null || finishedAt === null) return;
    const requests = performance
      .getEntriesByType("resource")
      .filter(
        (entry): entry is PerformanceResourceTiming =>
          entry.name.includes(urlMatch) && entry.startTime >= startedAt - 1 && entry.startTime <= finishedAt
      );
    const bytes = requests.reduce((total, entry) => total + entry.decodedBodySize, 0);
    const time = Math.round(finishedAt - startedAt);
    setSummary(
      requests.length === 0
        ? `${time} ms · already loaded`
        : `${time} ms · ${formatSize(bytes)} · ${requests.length} ${requests.length === 1 ? "request" : "requests"}`
    );
  }, [startedAt, finishedAt, urlMatch]);

  if (!debugMode || !summary) return null;
  return (
    <Badge variant="outline" className="font-mono text-xs">
      {label}: {summary}
    </Badge>
  );
}
