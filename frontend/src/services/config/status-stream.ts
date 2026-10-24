import { log } from "@/lib/logger";

import type { AppStatusResponse } from "@/types/config/response-status";

// Subscribes to the app status over Server-Sent Events. It is sent on connect and whenever it changes.
export const subscribeToAppStatus = (
  onStatus: (status: AppStatusResponse) => void,
  onConnectionChange?: (connected: boolean) => void
): (() => void) => {
  const es = new EventSource(`/api/config/stream`, { withCredentials: true });

  es.onopen = () => onConnectionChange?.(true);

  es.onmessage = (event: MessageEvent<string>) => {
    try {
      const data: { status?: AppStatusResponse } = JSON.parse(event.data);
      if (data.status) onStatus(data.status);
    } catch (error) {
      log("ERROR", "API - Config", "Status Stream", "Failed to parse the app status event", { error });
    }
  };

  es.onerror = () => {
    // EventSource reconnects on its own, like while aura restarts
    onConnectionChange?.(false);
  };

  return () => es.close();
};
