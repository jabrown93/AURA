import { useEffect, useState } from "react";

import { log } from "@/lib/logger";

export function useAppVersion(currentVersion: string = "dev") {
  const [latestVersion, setLatestVersion] = useState<string | null>(null);
  const [showReleaseNotes, setShowReleaseNotes] = useState(false);
  const [changelog, setChangelog] = useState("");

  function isNewerVersion(latest: string, current: string): boolean {
    const parse = (v: string) => v.replace(/^v/, "").split("-")[0].split(".").map(Number);
    const [lMaj, lMin, lPatch] = parse(latest);
    const [cMaj, cMin, cPatch] = parse(current);

    if (lMaj > cMaj) return true;
    if (lMaj < cMaj) return false;
    if (lMin > cMin) return true;
    if (lMin < cMin) return false;
    if (lPatch > cPatch) return true;
    return false;
  }

  function normalizeVersion(version: string) {
    return version.replace(/^v/, "").replace(/-.*$/, "");
  }

  function getChangelogEntriesSince(changelog: string, lastVersion: string) {
    // Matches both the legacy "## [x.y.z] - YYYY-MM-DD" headings and the
    // semantic-release format "## [x.y.z](compare-url) (YYYY-MM-DD)" (1-3 #'s).
    const regex = /^#{1,3} \[([^\]]+)\](?:\([^)]*\))? (?:- )?\(?(\d{4}-\d{2}-\d{2})\)?/gm;
    const entries: { version: string; content: string }[] = [];
    let match;
    const indices: number[] = [];
    const versions: string[] = [];
    while ((match = regex.exec(changelog)) !== null) {
      indices.push(match.index);
      versions.push(match[1]);
    }
    indices.push(changelog.length);

    for (let i = 0; i < versions.length; i++) {
      entries.push({
        version: versions[i],
        content: changelog.slice(indices[i], indices[i + 1]),
      });
    }

    const normalizedLastVersion = normalizeVersion(lastVersion || "");
    const idx = entries.findIndex((e) => normalizeVersion(e.version) === normalizedLastVersion);

    return idx === -1 ? entries : entries.slice(0, idx);
  }

  useEffect(() => {
    const fetchLatestVersion = async () => {
      try {
        const res = await fetch("https://raw.githubusercontent.com/jabrown93/aura/main/VERSION.txt", {
          cache: "no-store",
        });
        if (!res.ok) throw new Error(`HTTP ${res.status}`);
        const txt = (await res.text()).trim();
        if (txt && txt.length < 50) {
          setLatestVersion(txt);
        }
      } catch (error) {
        log("ERROR", "App Version", "Fetch Latest Version", "Failed to fetch latest version:", error);
      }
    };
    fetchLatestVersion();
  }, []);

  useEffect(() => {
    const lastSeen = localStorage.getItem("lastSeenVersion");
    log("INFO", "App Version", "Release Notes", `Last seen version: ${lastSeen}, Current version: ${currentVersion}`);

    fetch("/CHANGELOG.md")
      .then((res) => res.text())
      .then((fullChangelog) => {
        const relevantEntries = getChangelogEntriesSince(fullChangelog, lastSeen || "");
        setChangelog(relevantEntries.map((e) => e.content).join("\n"));
        if (lastSeen !== currentVersion && relevantEntries.length > 0) {
          setShowReleaseNotes(true);
        }
      });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [currentVersion]);

  function handleCloseReleaseNotes() {
    localStorage.setItem("lastSeenVersion", currentVersion);
    log("INFO", "App Version", "Release Notes", `User closed release notes for version ${currentVersion}`);
    setShowReleaseNotes(false);
  }

  return {
    latestVersion,
    showReleaseNotes,
    changelog,
    isNewerVersion,
    handleCloseReleaseNotes,
  };
}
