// Helpers for saving settings as they change.
// Objects are compared field by field, everything else as a whole, the same as the backend.

type PlainObject = Record<string, unknown>;

const isPlainObject = (value: unknown): value is PlainObject =>
  typeof value === "object" && value !== null && !Array.isArray(value);

// undefined and null are the same as an empty value
const isEmptyValue = (value: unknown): boolean =>
  value === undefined ||
  value === null ||
  value === "" ||
  value === false ||
  (Array.isArray(value) && value.length === 0) ||
  (isPlainObject(value) && Object.keys(value).every((key) => isEmptyValue(value[key])));

export const configValuesEqual = (a: unknown, b: unknown): boolean => {
  if (a === b) return true;
  if (isEmptyValue(a) && isEmptyValue(b)) return true;
  if (Array.isArray(a) && Array.isArray(b)) {
    return a.length === b.length && a.every((item, index) => configValuesEqual(item, b[index]));
  }
  if (isPlainObject(a) && isPlainObject(b)) {
    const keys = new Set([...Object.keys(a), ...Object.keys(b)]);
    return [...keys].every((key) => configValuesEqual(a[key], b[key]));
  }
  return false;
};

// configChanges returns the settings in draft that differ from saved.
// Changed objects only include their changed fields; lists and values are included as a whole.
export const configChanges = (saved: unknown, draft: unknown): PlainObject => {
  const changes: PlainObject = {};
  if (!isPlainObject(draft)) return changes;
  const savedObject = isPlainObject(saved) ? saved : {};

  for (const key of Object.keys(draft)) {
    const draftValue = draft[key];
    const savedValue = savedObject[key];
    if (configValuesEqual(savedValue, draftValue)) continue;

    if (isPlainObject(draftValue) && isPlainObject(savedValue)) {
      changes[key] = configChanges(savedValue, draftValue);
    } else {
      changes[key] = draftValue;
    }
  }
  return changes;
};

// Dirty fields are the changed fields, in the shape the settings cards use:
// - a changed value is true
// - a changed object has its changed fields
// - a changed list of objects has the changed fields of each item; a changed list of values is true
export type DirtyFields = { [key: string]: boolean | DirtyFields | DirtyFields[] };

export const configDirtyFields = (saved: unknown, draft: unknown): DirtyFields => {
  const dirty: DirtyFields = {};
  if (!isPlainObject(draft)) return dirty;
  const savedObject = isPlainObject(saved) ? saved : {};

  for (const key of Object.keys(draft)) {
    const draftValue = draft[key];
    const savedValue = savedObject[key];
    if (configValuesEqual(savedValue, draftValue)) continue;

    if (isPlainObject(draftValue)) {
      dirty[key] = configDirtyFields(savedValue, draftValue);
    } else if (Array.isArray(draftValue) && draftValue.some(isPlainObject)) {
      const savedList = Array.isArray(savedValue) ? savedValue : [];
      dirty[key] = draftValue.map((item, index) => configDirtyFields(savedList[index], item));
    } else {
      dirty[key] = true;
    }
  }
  return dirty;
};

// splitChanges splits changes into the ones that save automatically and the ones in groupKeys
// that are saved together with Test & Save
export const splitChanges = (
  changes: PlainObject,
  groupKeys: readonly string[]
): { autoChanges: PlainObject; groupChanges: PlainObject } => {
  const autoChanges: PlainObject = {};
  const groupChanges: PlainObject = {};
  for (const [key, value] of Object.entries(changes)) {
    if (groupKeys.includes(key)) groupChanges[key] = value;
    else autoChanges[key] = value;
  }
  return { autoChanges, groupChanges };
};
