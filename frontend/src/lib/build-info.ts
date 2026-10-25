// Build details set by the image build, empty for local builds
export const BUILD_INFO = {
  version: process.env.NEXT_PUBLIC_APP_VERSION || "dev",
  commit: process.env.NEXT_PUBLIC_APP_COMMIT || "",
  buildNumber: process.env.NEXT_PUBLIC_APP_BUILD_NUMBER || "",
  buildDate: process.env.NEXT_PUBLIC_APP_BUILD_DATE || "",
};

// npm run dev sets the build number to dev, older dev builds end the version with dev
export const IS_DEV_BUILD = BUILD_INFO.version.endsWith("dev") || BUILD_INFO.buildNumber === "dev";

// Local builds add -dirty to the commit when there were uncommitted changes
const commitSHA = BUILD_INFO.commit.replace(/-dirty$/, "");

export const BUILD_COMMIT_SHORT = BUILD_INFO.commit ? BUILD_INFO.commit.slice(0, 7) : "";
export const BUILD_COMMIT_DIRTY = BUILD_INFO.commit.endsWith("-dirty");
// Fork: commits link to this fork
export const BUILD_COMMIT_URL = commitSHA ? `https://github.com/jabrown93/AURA/commit/${commitSHA}` : "";
