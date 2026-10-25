// Errors for empty fields can be hidden until a section has been left,
// so a new setup section doesn't start out red.

const isEmptyField = (value: unknown): boolean =>
  value === undefined ||
  value === null ||
  (typeof value === "string" && value.trim() === "") ||
  (Array.isArray(value) && value.length === 0);

// withoutEmptyFieldErrors returns the errors for fields that are not empty
export const withoutEmptyFieldErrors = <E extends Partial<Record<string, string>>>(
  errors: E,
  values: Record<string, unknown>
): E => Object.fromEntries(Object.entries(errors).filter(([field]) => !isEmptyField(values[field]))) as E;
