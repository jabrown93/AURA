"use client";

import { Plus } from "lucide-react";

import React, { useCallback, useEffect, useRef, useState } from "react";

import {
  NEW_SERVER_PREFIX,
  OtherMediaServerFields,
  newMediaServer,
} from "@/components/settings-onboarding/OtherMediaServerFields";
import { Button } from "@/components/ui/button";

import type { AppConfigMediaServer, AppConfigMediaServerLibrary } from "@/types/config/config";

type ConnectionCheck = { verified: boolean; checking: boolean };
type ServerErrors = Partial<Record<keyof AppConfigMediaServer, string>>;

interface ConfigSectionOtherMediaServersSetupProps {
  servers: AppConfigMediaServer[]; // The media servers that aren't the main one
  mainLibraries: AppConfigMediaServerLibrary[]; // Libraries of the main media server
  onChange: (update: (servers: AppConfigMediaServer[]) => AppConfigMediaServer[]) => void;
  onConnectionVerifiedChange: (verified: boolean, checking: boolean) => void; // Whether every enabled media server works
  errorsUpdate: (errors: Record<string, string>) => void;
}

// The other media servers during setup. They are saved with the rest of the setup.
export const ConfigSectionOtherMediaServersSetup: React.FC<ConfigSectionOtherMediaServersSetupProps> = ({
  servers,
  mainLibraries,
  onChange,
  onConnectionVerifiedChange,
  errorsUpdate,
}) => {
  const nextNewServer = useRef(1);
  const [connectionChecks, setConnectionChecks] = useState<Record<string, ConnectionCheck>>({});
  const [serverErrors, setServerErrors] = useState<Record<string, ServerErrors>>({});

  const addMediaServer = () => {
    const id = `${NEW_SERVER_PREFIX}${nextNewServer.current++}`;
    onChange((list) => [...list, { ...newMediaServer(), id }]);
  };

  const removeMediaServer = (id: string) => {
    onChange((list) => list.filter((server) => server.id !== id));
    setConnectionChecks(({ [id]: _, ...rest }) => rest);
    setServerErrors(({ [id]: _, ...rest }) => rest);
  };

  const updateServer = useCallback(
    (id: string, field: keyof AppConfigMediaServer, value: unknown) => {
      onChange((list) => list.map((server) => (server.id === id ? { ...server, [field]: value } : server)));
    },
    [onChange]
  );

  const reportConnection = useCallback((id: string, check: ConnectionCheck) => {
    setConnectionChecks((prev) =>
      prev[id]?.verified === check.verified && prev[id]?.checking === check.checking ? prev : { ...prev, [id]: check }
    );
  }, []);

  const reportErrors = useCallback((id: string, errors: ServerErrors) => {
    setServerErrors((prev) => {
      const cleaned = Object.fromEntries(Object.entries(errors).filter(([, message]) => !!message)) as ServerErrors;
      return JSON.stringify(prev[id] ?? {}) === JSON.stringify(cleaned) ? prev : { ...prev, [id]: cleaned };
    });
  }, []);

  // Every enabled media server must have a working connection
  useEffect(() => {
    const enabled = servers.filter((server) => server.enabled ?? true);
    const verified = enabled.every((server) => connectionChecks[server.id ?? ""]?.verified);
    const checking = enabled.some((server) => connectionChecks[server.id ?? ""]?.checking);
    onConnectionVerifiedChange(verified, checking);
  }, [servers, connectionChecks, onConnectionVerifiedChange]);

  // Errors are shown with the media server's name
  useEffect(() => {
    const errors: Record<string, string> = {};
    for (const server of servers) {
      const name = server.name || server.type || "Media Server";
      for (const [field, message] of Object.entries(serverErrors[server.id ?? ""] ?? {})) {
        if (message) errors[`${server.id}.${field}`] = `${name}: ${message}`;
      }
    }
    errorsUpdate(errors);
  }, [servers, serverErrors, errorsUpdate]);

  return (
    <div className="space-y-4">
      {servers.length === 0 && (
        <p className="text-sm text-muted-foreground">
          No other media servers yet. Add one to apply images to it as well, or skip this step.
        </p>
      )}

      {servers.map((server) => (
        <SetupMediaServer
          key={server.id}
          server={server}
          mainLibraries={mainLibraries}
          onUpdate={updateServer}
          onRemove={removeMediaServer}
          onConnection={reportConnection}
          onErrors={reportErrors}
        />
      ))}

      <Button variant="outline" onClick={addMediaServer} className="w-full cursor-pointer">
        <Plus className="h-4 w-4" />
        Add Media Server
      </Button>
    </div>
  );
};

interface SetupMediaServerProps {
  server: AppConfigMediaServer;
  mainLibraries: AppConfigMediaServerLibrary[];
  onUpdate: (id: string, field: keyof AppConfigMediaServer, value: unknown) => void;
  onRemove: (id: string) => void;
  onConnection: (id: string, check: ConnectionCheck) => void;
  onErrors: (id: string, errors: ServerErrors) => void;
}

const SetupMediaServer: React.FC<SetupMediaServerProps> = ({
  server,
  mainLibraries,
  onUpdate,
  onRemove,
  onConnection,
  onErrors,
}) => {
  const id = server.id ?? "";

  const onChange = useCallback(
    <K extends keyof AppConfigMediaServer>(field: K, value: AppConfigMediaServer[K]) => onUpdate(id, field, value),
    [id, onUpdate]
  );
  const errorsUpdate = useCallback((errors: ServerErrors) => onErrors(id, errors), [id, onErrors]);
  const onConnectionVerifiedChange = useCallback(
    (verified: boolean, checking: boolean) => onConnection(id, { verified, checking }),
    [id, onConnection]
  );

  return (
    <OtherMediaServerFields
      serverKey={id}
      server={server}
      mainLibraries={mainLibraries}
      isNew={id.startsWith(NEW_SERVER_PREFIX)}
      onChange={onChange}
      errorsUpdate={errorsUpdate}
      onConnectionVerifiedChange={onConnectionVerifiedChange}
      actions={
        <Button variant="ghost" size="sm" onClick={() => onRemove(id)} className="cursor-pointer">
          Remove
        </Button>
      }
    />
  );
};
