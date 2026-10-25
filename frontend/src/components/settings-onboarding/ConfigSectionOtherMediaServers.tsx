"use client";

import { UpdateAppConfigSection } from "@/services/config/update-section";
import {
  GetMediaServerSync,
  type MediaServerSync,
  StartMediaServerSync,
  StopMediaServerSync,
} from "@/services/mediaserver/sync";
import { Loader2, Plus } from "lucide-react";
import { toast } from "sonner";

import React, { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { MakeMainMediaServerDialog } from "@/components/settings-onboarding/MakeMainMediaServerDialog";
import { MediaServerSyncPanel } from "@/components/settings-onboarding/MediaServerSyncPanel";
import {
  NEW_SERVER_PREFIX,
  OtherMediaServerFields,
  newMediaServer,
} from "@/components/settings-onboarding/OtherMediaServerFields";
import { ConfirmDestructiveDialogActionButton } from "@/components/shared/dialog-destructive-action";
import { Button } from "@/components/ui/button";

import type { AppConfigMediaServer, AppConfigMediaServerLibrary } from "@/types/config/config";
import type { AppStatusResponse } from "@/types/config/response-status";

// How often the progress is checked while the saved sets are being applied
const SYNC_CHECK_INTERVAL = 2000; // milliseconds

interface ConfigSectionOtherMediaServersProps {
  servers: AppConfigMediaServer[]; // Every saved media server, including the main one
  mainMediaServer: string; // ID of the main media server
  mainLibraries: AppConfigMediaServerLibrary[]; // Libraries of the main media server
  onSaved: (status: AppStatusResponse) => void;
}

export const ConfigSectionOtherMediaServers: React.FC<ConfigSectionOtherMediaServersProps> = ({
  servers,
  mainMediaServer,
  mainLibraries,
  onSaved,
}) => {
  const [newServerKeys, setNewServerKeys] = useState<string[]>([]);
  const [makeMain, setMakeMain] = useState<AppConfigMediaServer | null>(null);
  const nextNewServer = useRef(1);

  // Applying the saved sets runs in the background. Its progress is checked while it runs
  const [sync, setSync] = useState<MediaServerSync | null>(null);
  const syncRunning = !!sync?.running;
  useEffect(() => {
    let cancelled = false;
    const check = async () => {
      const response = await GetMediaServerSync();
      if (!cancelled && response.status !== "error") setSync(response.data?.sync ?? null);
    };
    void check();
    const interval = syncRunning ? setInterval(check, SYNC_CHECK_INTERVAL) : undefined;
    return () => {
      cancelled = true;
      clearInterval(interval);
    };
  }, [syncRunning]);

  const startSync = async (serverId: string) => {
    const response = await StartMediaServerSync(serverId);
    if (response.status === "error" || !response.data?.sync) {
      toast.error(response.error?.message || "Could not apply the saved sets");
      return;
    }
    setSync(response.data.sync);
  };

  const stopSync = async () => {
    const response = await StopMediaServerSync();
    if (response.status === "error") {
      toast.error(response.error?.message || "Could not stop applying the saved sets");
      return;
    }
    setSync(response.data?.sync ?? null);
  };

  const otherServers = useMemo(
    () => servers.filter((server) => server.id !== mainMediaServer),
    [servers, mainMediaServer]
  );

  // Save the media servers, with one of them changed, added or removed
  const saveMediaServers = useCallback(
    async (mediaServers: AppConfigMediaServer[], successMessage: string): Promise<boolean> => {
      const response = await UpdateAppConfigSection("media_servers", { media_servers: mediaServers });
      if (response.status === "error" || !response.data) {
        toast.error(response.error?.message || "Failed to save the media servers");
        return false;
      }
      onSaved(response.data.status);
      toast.success(successMessage);
      return true;
    },
    [onSaved]
  );

  const addMediaServer = () => {
    setNewServerKeys((keys) => [...keys, `${NEW_SERVER_PREFIX}${nextNewServer.current++}`]);
  };

  const removeNewServer = (key: string) => {
    setNewServerKeys((keys) => keys.filter((k) => k !== key));
  };

  return (
    <div className="mt-6 space-y-4">
      <div>
        <h2 className="text-xl font-semibold">Other Media Servers</h2>
        <p className="text-sm text-muted-foreground">
          Images are also applied to these media servers. Only the main media server saves images locally.
        </p>
      </div>

      {otherServers.length === 0 && newServerKeys.length === 0 && (
        <p className="text-sm text-muted-foreground">No other media servers yet.</p>
      )}

      {otherServers.map((server) => (
        <OtherMediaServer
          key={server.id}
          serverKey={server.id ?? ""}
          saved={server}
          onMakeMain={() => setMakeMain(server)}
          mainLibraries={mainLibraries}
          sync={sync?.server_id === server.id ? sync : null}
          otherSyncRunning={syncRunning && sync?.server_id !== server.id ? sync?.server_name : undefined}
          onStartSync={() => startSync(server.id ?? "")}
          onStopSync={stopSync}
          onSave={(draft) =>
            saveMediaServers(
              servers.map((s) => (s.id === server.id ? draft : s)),
              `${draft.name || draft.type} saved`
            )
          }
          onRemove={() =>
            saveMediaServers(
              servers.filter((s) => s.id !== server.id),
              `${server.name || server.type} removed`
            )
          }
        />
      ))}

      {newServerKeys.map((key) => (
        <OtherMediaServer
          key={key}
          serverKey={key}
          mainLibraries={mainLibraries}
          onSave={async (draft) => {
            const saved = await saveMediaServers(
              [...servers, { ...draft, id: undefined }],
              `${draft.name || draft.type} added`
            );
            if (saved) removeNewServer(key);
            return saved;
          }}
          onRemove={async () => {
            removeNewServer(key);
            return true;
          }}
        />
      ))}

      <MakeMainMediaServerDialog server={makeMain} onClose={() => setMakeMain(null)} />

      <Button variant="outline" onClick={addMediaServer} className="w-full cursor-pointer">
        <Plus className="h-4 w-4" />
        Add Media Server
      </Button>
    </div>
  );
};

interface OtherMediaServerProps {
  serverKey: string;
  saved?: AppConfigMediaServer; // Not set for a media server that hasn't been saved yet
  mainLibraries: AppConfigMediaServerLibrary[];
  onSave: (draft: AppConfigMediaServer) => Promise<boolean>;
  onRemove: () => Promise<boolean>;
  onMakeMain?: () => void; // Not set for a media server that hasn't been saved yet
  sync?: MediaServerSync | null; // The latest time the saved sets were applied to this media server
  otherSyncRunning?: string; // Name of another media server the saved sets are being applied to
  onStartSync?: () => Promise<void>; // Not set for a media server that hasn't been saved yet
  onStopSync?: () => Promise<void>;
}

const OtherMediaServer: React.FC<OtherMediaServerProps> = ({
  serverKey,
  saved,
  mainLibraries,
  onSave,
  onRemove,
  onMakeMain,
  sync = null,
  otherSyncRunning,
  onStartSync,
  onStopSync,
}) => {
  const [draft, setDraft] = useState<AppConfigMediaServer>(() => saved ?? newMediaServer());
  const [hasErrors, setHasErrors] = useState(false);
  const [saving, setSaving] = useState(false);
  const isNew = serverKey.startsWith(NEW_SERVER_PREFIX);

  // Show the saved settings again after saving, as aura cleans some of them up
  const savedJSON = JSON.stringify(saved ?? null);
  const justSaved = useRef(false);
  useEffect(() => {
    if (justSaved.current && saved) {
      justSaved.current = false;
      setDraft(saved);
    }
  }, [savedJSON, saved]);

  const onChange = useCallback(<K extends keyof AppConfigMediaServer>(field: K, value: AppConfigMediaServer[K]) => {
    setDraft((prev) => ({ ...prev, [field]: value }));
  }, []);

  const errorsUpdate = useCallback((errors: Partial<Record<keyof AppConfigMediaServer, string>>) => {
    setHasErrors(Object.values(errors).some(Boolean));
  }, []);

  // Changed fields are highlighted until they are saved
  const dirtyFields = useMemo(() => {
    const fields: Partial<Record<keyof AppConfigMediaServer, boolean>> = {};
    for (const key of Object.keys(draft) as (keyof AppConfigMediaServer)[]) {
      if (JSON.stringify(draft[key] ?? null) !== JSON.stringify(saved?.[key] ?? null)) fields[key] = true;
    }
    return fields;
  }, [draft, saved]);
  const isDirty = isNew || Object.keys(dirtyFields).length > 0;

  const handleSave = async () => {
    setSaving(true);
    justSaved.current = true;
    const success = await onSave(draft);
    if (!success) justSaved.current = false;
    setSaving(false);
  };

  return (
    <OtherMediaServerFields
      serverKey={serverKey}
      server={draft}
      mainLibraries={mainLibraries}
      isNew={isNew}
      highlighted={isDirty}
      dirtyFields={dirtyFields}
      onChange={onChange}
      errorsUpdate={errorsUpdate}
      actions={
        <>
          {onMakeMain && (
            <Button
              variant="outline"
              size="sm"
              onClick={onMakeMain}
              disabled={isDirty || !(saved?.enabled ?? true)}
              title={isDirty ? "Save or discard your changes first" : undefined}
              className="cursor-pointer"
            >
              Make Main
            </Button>
          )}
          {isNew ? (
            <Button variant="ghost" size="sm" onClick={() => void onRemove()} className="cursor-pointer">
              Cancel
            </Button>
          ) : (
            <ConfirmDestructiveDialogActionButton
              onConfirm={() => void onRemove()}
              title={`Remove ${saved?.name || saved?.type || "this media server"}?`}
              description="aura stops applying images to this media server. Images already applied stay on it."
              confirmText="Remove"
              variant="outline"
            >
              Remove
            </ConfirmDestructiveDialogActionButton>
          )}
        </>
      }
      footer={
        <>
          {onStartSync && onStopSync && (
            <MediaServerSyncPanel
              serverName={saved?.name || saved?.type || "this media server"}
              sync={sync}
              disabledReason={
                isDirty
                  ? "Save or discard your changes first"
                  : !(saved?.enabled ?? true)
                    ? "Turn the media server on first"
                    : otherSyncRunning
                      ? `Saved sets are being applied to ${otherSyncRunning}`
                      : undefined
              }
              onStart={onStartSync}
              onStop={onStopSync}
            />
          )}
          {isDirty && (
            <div className="flex justify-end gap-2">
              {!isNew && (
                <Button
                  variant="ghost"
                  onClick={() => saved && setDraft(saved)}
                  disabled={saving}
                  className="cursor-pointer"
                >
                  Discard
                </Button>
              )}
              <Button
                onClick={handleSave}
                disabled={saving || hasErrors || !draft.url.trim() || !draft.api_token.trim()}
                className="cursor-pointer"
              >
                {saving && <Loader2 className="h-4 w-4 animate-spin" />}
                {saving ? "Testing..." : "Test & Save"}
              </Button>
            </div>
          )}
        </>
      }
    />
  );
};
