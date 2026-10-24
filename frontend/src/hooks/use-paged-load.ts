import { ReturnErrorMessage } from "@/services/api-error-return";

import { useCallback, useEffect, useRef, useState } from "react";

import type { APIResponse } from "@/types/api/api-response";

// A default that stays the same between renders
const alwaysHasPages = () => true;

// Loads a page for a query, only the newest request updates the page
export function usePagedLoad<Q extends { page: number }, D extends { page: number }>(
  fetchPage: (q: Q, refresh: boolean) => Promise<APIResponse<D>>,
  errorMessage: string,
  setCurrentPage: (page: number) => void,
  hasPages: (q: Q) => boolean = alwaysHasPages
) {
  const [shown, setShown] = useState<{ query: Q; data: D } | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<APIResponse<unknown> | null>(null);
  const [timing, setTiming] = useState<{ startedAt: number | null; finishedAt: number | null }>({
    startedAt: null,
    finishedAt: null,
  });
  const latestRequest = useRef(0);

  const load = useCallback(
    async (q: Q, refresh = false) => {
      const startedAt = performance.now();
      const request = ++latestRequest.current;
      setLoading(true);
      const response = await fetchPage(q, refresh);
      if (request !== latestRequest.current) return;
      setLoading(false);
      if (response.status === "error" || !response.data) {
        setError(response.status === "error" ? response : ReturnErrorMessage<unknown>(errorMessage));
        return;
      }

      setShown({ query: q, data: response.data });
      setError(null);
      setTiming({ startedAt, finishedAt: performance.now() });

      // A page past the end shows the last page
      if (hasPages(q) && response.data.page !== q.page) setCurrentPage(response.data.page);
    },
    [fetchPage, errorMessage, setCurrentPage, hasPages]
  );

  return { shown, loading, error, setError, timing, load };
}

// A new search starts at the first page
export function useFirstPageOnSearch(searchQuery: string, setCurrentPage: (page: number) => void) {
  const previousSearch = useRef(searchQuery);
  useEffect(() => {
    if (searchQuery !== previousSearch.current) {
      previousSearch.current = searchQuery;
      setCurrentPage(1);
    }
  }, [searchQuery, setCurrentPage]);
}
