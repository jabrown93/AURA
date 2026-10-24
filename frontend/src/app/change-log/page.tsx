"use client";

import { Suspense, useEffect, useMemo, useState } from "react";

import { useSearchParams } from "next/navigation";

import { ChangelogMarkdown } from "@/components/shared/changelog-markdown";
import { Accordion, AccordionContent, AccordionItem, AccordionTrigger } from "@/components/ui/accordion";

import { BUILD_COMMIT_DIRTY, BUILD_COMMIT_SHORT, BUILD_COMMIT_URL, BUILD_INFO, IS_DEV_BUILD } from "@/lib/build-info";

export default function Changelog() {
  return (
    <Suspense fallback={<div>Loading...</div>}>
      <ChangelogContent />
    </Suspense>
  );
}

interface ChangelogSection {
  version: string | null;
  date: string | null;
  content: string;
}

function splitChangelogSections(markdown: string): ChangelogSection[] {
  if (!markdown) return [];
  return markdown
    .split(/(?=^##\s*\[)/m)
    .filter((block) => block.trim().length > 0)
    .map((block) => {
      const trimmed = block.replace(/\n-{3,}\s*$/, "").trim();
      const match = /##\s*\[([^\]]+)\]\s*-\s*(\d{4}-\d{2}-\d{2})/.exec(trimmed);
      return {
        version: match ? match[1] : null,
        date: match ? match[2] : null,
        content: trimmed,
      };
    });
}

const normalizeVersion = (v: string) => v.replace(/^v/, "").replace(/-.*$/, "");

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

  const sections = useMemo(() => splitChangelogSections(content), [content]);
  const [latestSection, ...olderSections] = sections;

  // Auto-expand the section matching currentVersion if it isn't the latest one already shown open.
  const currentSectionValue = useMemo(() => {
    if (!currentVersion) return undefined;
    const idx = olderSections.findIndex(
      (section) => section.version && normalizeVersion(section.version) === normalizeVersion(currentVersion)
    );
    return idx >= 0 ? `version-${idx}` : undefined;
  }, [olderSections, currentVersion]);

  return (
    <div className="min-h-screen py-2 px-10 flex justify-center">
      <div className="w-full max-w-4xl rounded-lg shadow-md p-2">
        <h1 className="text-3xl font-bold mb-6 text-center">Change Log</h1>

        <BuildDetails />

        {latestVersion && currentVersion && (
          <>
            <div className="my-6 flex items-center gap-3">
              <hr className="flex-grow border-amber-400 border-t-2" />
            </div>
            <h2 className="text-xl font-bold mb-4 text-amber-700 text-center">Updates since {currentVersion}</h2>
          </>
        )}

        {latestSection && (
          <ChangelogMarkdown currentVersion={currentVersion}>{latestSection.content}</ChangelogMarkdown>
        )}

        {olderSections.length > 0 && (
          <>
            <div className="my-8 flex items-center gap-3">
              <hr className="flex-grow border-muted-foreground/20" />
              <span className="text-muted-foreground text-sm font-semibold uppercase tracking-wide">
                Previous Updates
              </span>
              <hr className="flex-grow border-muted-foreground/20" />
            </div>
            <Accordion type="multiple" defaultValue={currentSectionValue ? [currentSectionValue] : []}>
              {olderSections.map((section, index) => (
                <AccordionItem key={section.version ?? index} value={`version-${index}`}>
                  <AccordionTrigger className="text-lg font-bold">
                    <span className="flex items-center gap-3">
                      {section.version ?? "Unknown version"}
                      {section.date && (
                        <span className="text-muted-foreground text-sm font-normal">{section.date}</span>
                      )}
                      {`version-${index}` === currentSectionValue && (
                        <span className="bg-yellow-100 text-yellow-800 px-2 py-0.5 rounded text-xs font-semibold border border-yellow-300">
                          You are here
                        </span>
                      )}
                    </span>
                  </AccordionTrigger>
                  <AccordionContent>
                    <ChangelogMarkdown currentVersion={currentVersion}>
                      {section.content.replace(/^##\s*\[[^\]]+\]\s*-\s*\d{4}-\d{2}-\d{2}\s*/, "")}
                    </ChangelogMarkdown>
                  </AccordionContent>
                </AccordionItem>
              ))}
            </Accordion>
          </>
        )}
      </div>
    </div>
  );
}

// The build details, to check this is an official image
function BuildDetails() {
  const builtAt = BUILD_INFO.buildDate ? new Date(BUILD_INFO.buildDate) : null;
  const details = [
    { label: "Version", value: BUILD_INFO.version },
    { label: "Build", value: BUILD_INFO.buildNumber },
    {
      label: "Commit",
      value: BUILD_COMMIT_SHORT && (
        <span className="flex items-center gap-2">
          <a
            href={BUILD_COMMIT_URL}
            target="_blank"
            rel="noopener noreferrer"
            className="font-mono underline underline-offset-2 hover:text-primary"
            title={BUILD_INFO.commit}
          >
            {BUILD_COMMIT_SHORT}
          </a>
          {BUILD_COMMIT_DIRTY && <span className="text-xs text-muted-foreground">(with uncommitted changes)</span>}
        </span>
      ),
    },
    {
      label: "Built",
      value: builtAt && !isNaN(builtAt.getTime()) ? builtAt.toLocaleString() : BUILD_INFO.buildDate,
    },
  ];

  return (
    <div className="mb-6 rounded-md border p-3 sm:p-4">
      <h2 className="mb-2 font-semibold">Build Details</h2>
      {BUILD_INFO.commit ? (
        <dl className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1 text-sm">
          {details.map(
            (detail) =>
              detail.value && (
                <div key={detail.label} className="contents">
                  <dt className="text-muted-foreground">{detail.label}</dt>
                  <dd className="min-w-0 break-all">{detail.value}</dd>
                </div>
              )
          )}
        </dl>
      ) : IS_DEV_BUILD ? (
        <p className="text-sm text-muted-foreground">Version {BUILD_INFO.version}, development build.</p>
      ) : (
        <p className="text-sm text-muted-foreground">Version {BUILD_INFO.version}, built without build details.</p>
      )}
      <p className="mt-2 text-xs text-muted-foreground">
        {/* Fork: images are published from jabrown93/AURA */}
        Images from ghcr.io/jabrown93/aura show a build number and a commit from the jabrown93/AURA repository on GitHub.
      </p>
    </div>
  );
}
