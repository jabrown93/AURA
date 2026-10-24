import { useState } from "react";

// Runs a refresh, and says whether one is running
export function useRefreshing(refresh: () => Promise<unknown>): [boolean, () => Promise<void>] {
  const [refreshing, setRefreshing] = useState(false);
  const run = async () => {
    setRefreshing(true);
    try {
      await refresh();
    } finally {
      setRefreshing(false);
    }
  };
  return [refreshing, run];
}
