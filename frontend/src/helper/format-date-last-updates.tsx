import { makePlural } from "@/helper/make_plural";

export const formatLastUpdatedDate = (lastUpdateString: string, dateCreatedString: string) => {
  try {
    if (!lastUpdateString && !dateCreatedString) {
      return "Invalid Date";
    }
    let useDateString = lastUpdateString;
    if (lastUpdateString === "0001-01-01T00:00:00Z") {
      useDateString = dateCreatedString;
    }
    const date = new Date(useDateString);
    if (isNaN(date.getTime())) throw new Error();

    const now = new Date();
    const diffMs = now.getTime() - date.getTime();
    const diffDays = Math.floor(diffMs / (1000 * 60 * 60 * 24));
    const diffWeeks = Math.floor(diffDays / 7);
    const diffMonths = Math.floor(diffDays / 30);
    const diffYears = Math.floor(diffDays / 365);

    if (diffYears >= 1) {
      return `Over ${diffYears} ${makePlural(diffYears, "year")} ago`;
    } else if (diffMonths >= 1) {
      return `${diffMonths} ${makePlural(diffMonths, "month")} ago`;
    } else if (diffWeeks >= 1) {
      return `${diffWeeks} ${makePlural(diffWeeks, "week")} ago`;
    } else if (diffDays >= 1) {
      return `${diffDays} ${makePlural(diffDays, "day")} ago`;
    } else {
      const diffHours = Math.floor(diffMs / (1000 * 60 * 60));
      const diffMinutes = Math.floor(diffMs / (1000 * 60));
      if (diffHours >= 1) {
        return `${diffHours} ${makePlural(diffHours, "hour")} ago`;
      } else if (diffMinutes < 5) {
        return "Just a moment ago";
      } else {
        return `${diffMinutes} ${makePlural(diffMinutes, "minute")} ago`;
      }
    }
  } catch {
    return "Invalid Date";
  }
};

export const formatExactDateTime = (dateString: string) => {
  try {
    if (!dateString) {
      return "Invalid Date";
    }
    const date = new Date(dateString);
    if (isNaN(date.getTime())) throw new Error();

    const pad = (n: number) => n.toString().padStart(2, "0");
    return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`;
  } catch {
    return "Invalid Date";
  }
};

export const formatUnixSeconds = (unixSeconds: number) => {
  if (!unixSeconds) return "";
  return new Date(unixSeconds * 1000).toLocaleString("en-US", {
    month: "2-digit",
    day: "2-digit",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hour12: false,
  });
};

// A date like 10/18/2026 at 1:00:07 AM, empty when there's no date
export const formatDateAndTime = (value: string | number | Date | undefined | null) => {
  if (!value) return "";
  const date = new Date(value);
  if (isNaN(date.getTime()) || date.getTime() <= 0) return "";
  return `${date.toLocaleDateString("en-US")} at ${date.toLocaleTimeString("en-US", {
    hour: "numeric",
    minute: "numeric",
    second: "numeric",
    hour12: true,
  })}`;
};
