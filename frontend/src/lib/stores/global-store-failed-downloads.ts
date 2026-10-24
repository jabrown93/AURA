import { GetDownloadHistory } from "@/services/downloads/history-get";
import { create } from "zustand";

// The number of failed downloads, shown in the navbar
interface FailedDownloadsStore {
  failedCount: number;
  setFailedCount: (count: number) => void;
  refreshFailedCount: () => Promise<void>;
}

export const useFailedDownloadsStore = create<FailedDownloadsStore>()((set) => ({
  failedCount: 0,
  setFailedCount: (count) => set({ failedCount: count }),
  refreshFailedCount: async () => {
    // Only the total is needed
    const response = await GetDownloadHistory(1, 0);
    if (response.status === "success") {
      set({ failedCount: response.data?.total ?? 0 });
    }
  },
}));
