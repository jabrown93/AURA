import { isAxiosError } from "axios";

// Helpers to hide secrets before data is logged or shown

const SECRET_KEY_PATTERN =
  /(password|passwd|secret|token|api_?key|apikey|authorization|cookie|webhook|user_?key|hash|session|credential)/i;
const SECRET_QUERY_PARAM_PATTERN =
  /(token|password|secret|api_?key|apikey|auth|signature|session|^key$|^code$|^state$)/i;
const MAX_DEPTH = 8;

// maskSecret keeps the last 4 characters, like the backend
export const maskSecret = (value: string): string => (value ? `***${value.slice(-4)}` : value);

// redactURL hides sensitive query parameters like ?token= in a URL
export const redactURL = (rawURL: string): string => {
  const [base, query] = rawURL.split("?", 2);
  if (!query) return rawURL;
  const params = new URLSearchParams(query);
  for (const name of Array.from(params.keys())) {
    if (SECRET_QUERY_PARAM_PATTERN.test(name)) params.set(name, "REDACTED");
  }
  return `${base}?${params.toString()}`;
};

// redactSecrets returns a copy of value with the values of secret-looking keys masked
export const redactSecrets = (value: unknown, depth = 0): unknown => {
  if (depth > MAX_DEPTH) return "[...]";
  if (Array.isArray(value)) return value.map((item) => redactSecrets(item, depth + 1));
  if (typeof value !== "object" || value === null) return value;

  const result: Record<string, unknown> = {};
  for (const [key, item] of Object.entries(value)) {
    if (key.toLowerCase() === "headers" && typeof item === "object" && item !== null) {
      // Header values often hold credentials
      result[key] = Object.fromEntries(
        Object.entries(item).map(([name, headerValue]) => [
          name,
          typeof headerValue === "string" ? maskSecret(headerValue) : headerValue,
        ])
      );
    } else if (SECRET_KEY_PATTERN.test(key) && typeof item === "string") {
      result[key] = maskSecret(item);
    } else {
      result[key] = redactSecrets(item, depth + 1);
    }
  }
  return result;
};

// redactLogData makes data safe to log: errors are reduced to their details and secrets are masked
export const redactLogData = (data: unknown): unknown => {
  if (isAxiosError(data)) {
    return {
      name: data.name,
      message: data.message,
      code: data.code,
      status: data.response?.status,
      method: data.config?.method?.toUpperCase(),
      url: data.config?.url ? redactURL(data.config.url) : undefined,
      response: redactSecrets(data.response?.data),
    };
  }
  if (data instanceof Error) {
    return { name: data.name, message: data.message, stack: data.stack };
  }
  return redactSecrets(data);
};
