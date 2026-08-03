import { type AppConfigStatus_Response, GetAppConfigStatus } from "@/services/config/status";
import { create } from "zustand";
import { persist } from "zustand/middleware";

import { GlobalStore } from "@/lib/stores/stores";

import type { APIResponse } from "@/types/api/api-response";
import type { AppStatusResponse } from "@/types/config/response-status";

interface OnboardingStore {
  status: AppStatusResponse | null;
  loading: boolean;
  error: string | null;

  setStatus: (status: AppStatusResponse | null) => void;
  setLoading: (loading: boolean) => void;
  setError: (error: string | null) => void;

  fetchStatus: () => Promise<void>;
  clear: () => void;

  hasHydrated: boolean;
  hydrate: () => void;
}

export const useOnboardingStore = create<OnboardingStore>()(
  persist(
    (set) => ({
      status: null,
      loading: false,
      error: null,

      setStatus: (status) => set({ status }),
      setLoading: (loading) => set({ loading }),
      setError: (error) => set({ error }),

      fetchStatus: async () => {
        set({ loading: true, error: null });
        try {
          const response: APIResponse<AppConfigStatus_Response> = await GetAppConfigStatus(false);
          set({ status: response.data?.status, loading: false });
        } catch (err: unknown) {
          const message = err instanceof Error ? err.message : "Failed to fetch onboarding status";
          set({ error: message, loading: false });
        }
      },

      clear: () => set({ status: null, loading: false, error: null }),

      hasHydrated: false,
      hydrate: () => set({ hasHydrated: true }),
    }),
    {
      name: "Onboarding",
      storage: GlobalStore,
      partialize: (state) => ({
        status: state.status,
        loading: state.loading,
        error: state.error,
      }),
      onRehydrateStorage: () => (state) => {
        state?.hydrate();
      },
    }
  )
);
