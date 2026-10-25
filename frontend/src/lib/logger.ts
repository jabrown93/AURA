/* eslint-disable @typescript-eslint/no-unused-expressions */
import { IS_DEV_BUILD } from "@/lib/build-info";
import { redactLogData } from "@/lib/redact";

// Log to the browser console. data is redacted first, so secrets are never logged.
export function log(level: "INFO" | "WARN" | "ERROR", page: string, action: string, message: string, data?: unknown) {
  const debugMode = (typeof window !== "undefined" && localStorage.getItem("debugMode") === "true") || IS_DEV_BUILD;

  if (!debugMode && level !== "ERROR" && level !== "WARN") return;

  const now = new Date();
  const pad = (n: number, len = 2) => n.toString().padStart(len, "0");
  const formattedTime = `${now.getFullYear()}/${pad(now.getMonth() + 1)}/${pad(now.getDate())} ${pad(now.getHours())}:${pad(now.getMinutes())}:${pad(now.getSeconds())}:${pad(now.getMilliseconds(), 3)}`;

  const base = `[${formattedTime}] [${level}] [${page}] [${action}] - ${message}`;
  if (data !== undefined) data = redactLogData(data);
  if (level === "ERROR") {
    data !== undefined ? console.error(base, data) : console.error(base);
  } else if (level === "WARN") {
    data !== undefined ? console.warn(base, data) : console.warn(base);
  } else if (level === "INFO") {
    data !== undefined ? console.info(base, data) : console.info(base);
  } else {
    data !== undefined ? console.log(base, data) : console.log(base);
  }
}
