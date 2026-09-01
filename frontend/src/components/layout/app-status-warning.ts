import type { AppStatusResponse } from "@/types/config/response-status";

type DependencyStatus = Pick<AppStatusResponse, "media_server_reachable" | "mediux_reachable" | "mediux_valid">;

export type DependencyWarning = {
  label: string;
  detail: string;
};

export const getDependencyWarning = (status: DependencyStatus | null): DependencyWarning | null => {
  if (!status) return null;
  if (!status.media_server_reachable) {
    return {
      label: "Media server is unreachable",
      detail:
        "Media server is unreachable. aura is running with reduced functionality and will reconnect automatically.",
    };
  }
  if (!status.mediux_reachable) {
    return {
      label: "MediUX is unreachable",
      detail: "MediUX is unreachable. aura is running with reduced functionality and will reconnect automatically.",
    };
  }
  if (!status.mediux_valid) {
    return {
      label: "MediUX token is invalid",
      detail: "MediUX rejected configured token. Update it in settings to restore MediUX functionality.",
    };
  }
  return null;
};
