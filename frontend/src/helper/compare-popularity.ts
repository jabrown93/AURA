import type { TYPE_SORT_ORDER_OPTIONS } from "@/types/ui-options";

// Sorts by MediUX rank, 1 is best and unranked 0 always goes last
export const comparePopularity = (
  rankA: number | undefined,
  rankB: number | undefined,
  order: TYPE_SORT_ORDER_OPTIONS
): number => {
  const a = rankA || Infinity;
  const b = rankB || Infinity;
  if (a === b) return 0;
  if (a === Infinity) return 1;
  if (b === Infinity) return -1;
  return order === "desc" ? a - b : b - a;
};
