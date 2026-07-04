"use client";

import { Suspense, useEffect, useState } from "react";

import { useSearchParams } from "next/navigation";

import { ChangelogMarkdown } from "@/components/shared/changelog-markdown";

export default function Changelog() {
  return (
    <Suspense fallback={<div>Loading...</div>}>
      <ChangelogContent />
    </Suspense>
  );
}

function ChangelogContent() {
  const [content, setContent] = useState("");
  const searchParams = useSearchParams();
  const [currentVersion, setCurrentVersion] = useState<string | null>(null);
  const [latestVersion, setLatestVersion] = useState<string | null>(null);

  useEffect(() => {
    const currentVersion = searchParams.get("currentVersion");
    setCurrentVersion(currentVersion);
    const updates = searchParams.get("updates");
    const latestVersion = searchParams.get("latestVersion");
    setLatestVersion(latestVersion);
    if (updates === "true") {
      // Fetch latest changelog from GitHub (raw URL)
      fetch("https://raw.githubusercontent.com/jabrown93/aura/main/frontend/public/CHANGELOG.md")
        .then((res) => res.text())
        .then(setContent)
        .catch(() => setContent("Failed to fetch latest changelog from GitHub."));
    } else {
      const currentVersion = searchParams.get("currentVersion");
      setCurrentVersion(currentVersion);
      const latestVersion = searchParams.get("latestVersion");
      setLatestVersion(latestVersion);
      // Fetch local changelog
      fetch("/CHANGELOG.md")
        .then((res) => res.text())
        .then(setContent)
        .catch(() => setContent("Failed to fetch local changelog."));
    }
  }, [searchParams]);

  return (
    <div className="min-h-screen py-2 px-10 flex justify-center">
      <div className="w-full max-w-4xl rounded-lg shadow-md p-2">
        <h1 className="text-3xl font-bold mb-6 text-center">Change Log</h1>
        <ChangelogMarkdown currentVersion={currentVersion} latestVersion={latestVersion}>
          {content}
        </ChangelogMarkdown>
      </div>
    </div>
  );
}
