"use client";

import { ValidateURL } from "@/helper/validation/validate-url";
import { GetLibrarySectionOptions } from "@/services/mediaserver/get-library-section-options";
import { CheckAuthStatusWithPlex, GetPlexPinAndID, type PlexServersResponse } from "@/services/mediaserver/plex";
import { ValidateMediaServerInfo } from "@/services/validation/mediaserver";
import { RefreshCcw } from "lucide-react";

import React, { useCallback, useEffect, useRef, useState } from "react";

import {
  CONNECTION_STATUS_COLORS_BG,
  type ConfigConnectionStatus,
} from "@/components/settings-onboarding/ConfigSectionSonarrRadarr";
import { PopoverHelp } from "@/components/shared/popover-help";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectScrollDownButton,
  SelectScrollUpButton,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";

import { cn } from "@/lib/cn";

import type { AppConfigMediaServer, AppConfigMediaServerLibrary } from "@/types/config/config";

interface ConfigSectionMediaServerProps {
  value: AppConfigMediaServer;
  editing: boolean;
  configAlreadyLoaded: boolean;
  dirtyFields?: Partial<Record<keyof AppConfigMediaServer, boolean>>;
  onChange: <K extends keyof AppConfigMediaServer>(field: K, value: AppConfigMediaServer[K]) => void;
  errorsUpdate?: (errors: Partial<Record<keyof AppConfigMediaServer, string>>) => void;
}

const SERVER_TYPES = ["Plex", "Emby", "Jellyfin"];
const USER_ID_REQUIRED_TYPES = new Set<string>(["Emby", "Jellyfin"]);

export function GetConnectionColor(status: "unknown" | "ok" | "error"): "green-500" | "red-500" | "gray-500" {
  switch (status) {
    case "ok":
      return "green-500";
    case "error":
      return "red-500";
    default:
      return "gray-500";
  }
}

export const ConfigSectionMediaServer: React.FC<ConfigSectionMediaServerProps> = ({
  value,
  editing,
  configAlreadyLoaded,
  dirtyFields = {},
  onChange,
  errorsUpdate,
}) => {
  const prevErrorsRef = useRef<string>("");

  // Normalize libraries to array (avoid null errors)
  const libraries: AppConfigMediaServerLibrary[] = React.useMemo(
    () => (Array.isArray(value.libraries) ? value.libraries : []),
    [value.libraries]
  );

  useEffect(() => {
    if (!Array.isArray(value.libraries)) {
      onChange("libraries", [] as AppConfigMediaServerLibrary[]);
    }
  }, [onChange, value.libraries]);

  const [remoteTokenError, setRemoteTokenError] = useState<string | null>(null);
  const [testingToken, setTestingToken] = useState(false);
  const [connectionStatus, setConnectionStatus] = useState<ConfigConnectionStatus>({
    status: "unknown",
    color: GetConnectionColor("unknown"),
  });

  // Plex Signin States
  const PLEX_CLIENT_ID = "aura";
  const PLEX_PRODUCT = "AURA";
  const [plexID, setPlexID] = useState("");
  const [plexPIN, setPlexPIN] = useState("");
  const [plexConnectionsAvailable, setPlexConnectionsAvailable] = useState<PlexServersResponse[]>([]);
  const [plexOAuthWindow, setPlexOAuthWindow] = useState<Window | null>(null);
  const [signInWithPlex, setSignInWithPlex] = useState(false);

  const valueRef = React.useRef(value);
  React.useEffect(() => {
    valueRef.current = value;
  }, [value]);

  const [libraryFetchLoading, setLibraryFetchLoading] = useState(false);

  const typeNormalized = value.type.trim();
  const hasRunInitialValidation = useRef(false);

  const errors = React.useMemo<Partial<Record<keyof AppConfigMediaServer, string>>>(() => {
    const errs: Partial<Record<keyof AppConfigMediaServer, string>> = {};

    // Type
    if (!typeNormalized) errs.type = "Type is required.";
    else if (!SERVER_TYPES.includes(typeNormalized)) errs.type = "Unsupported type.";

    // URL
    if (!value.url.trim()) errs.url = "URL is required.";
    else {
      const urlErr = ValidateURL(value.url.trim());
      if (urlErr) errs.url = urlErr;
    }

    // Token
    if (!value.api_token.trim()) errs.api_token = "Token is required.";
    if (remoteTokenError) errs.api_token = remoteTokenError;

    // User ID requirement
    if (USER_ID_REQUIRED_TYPES.has(typeNormalized) && !value.user_id?.trim()) {
      errs.user_id = "User ID should be set automatically after URL & Token are valid.";
    }

    // Libraries
    if (libraries.length === 0) {
      errs.libraries = "Add at least one library.";
    } else {
      if (libraries.some((l) => !l.title?.trim())) errs.libraries = "Library names cannot be empty.";
      if (!errs.libraries) {
        const seen = new Set<string>();
        for (const l of libraries) {
          const n = (l.title || "").trim().toLowerCase();
          if (!n) continue;
          if (seen.has(n)) {
            errs.libraries = "Duplicate library names are not allowed.";
            break;
          }
          seen.add(n);
        }
      }
    }

    return errs;
  }, [typeNormalized, value.url, value.api_token, value.user_id, libraries, remoteTokenError]);

  // Emit errors upward
  useEffect(() => {
    if (!errorsUpdate) return;
    const serialized = JSON.stringify(errors);
    if (serialized === prevErrorsRef.current) return;
    prevErrorsRef.current = serialized;
    errorsUpdate(errors);
  }, [errors, errorsUpdate]);

  // Reset remote token error when URL or Token changes
  useEffect(() => {
    setRemoteTokenError(null);
  }, [value.api_token, value.url]);

  useEffect(() => {
    if (editing && value.url) {
      runRemoteValidation();
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [value.url]);

  const runRemoteValidation = useCallback(
    async (showToast = true) => {
      setConnectionStatus({ status: "unknown", color: GetConnectionColor("unknown") });
      const current = valueRef.current;
      if (!current.api_token.trim()) {
        setRemoteTokenError("Token is required.");
        setConnectionStatus({ status: "error", color: GetConnectionColor("error") });
        return;
      }
      if (!current.url.trim()) {
        setRemoteTokenError("URL is required.");
        setConnectionStatus({ status: "error", color: GetConnectionColor("error") });
        return;
      }

      setTestingToken(true);
      const start = Date.now();
      const { valid, message, media_server } = await ValidateMediaServerInfo(current, showToast);
      const elapsed = Date.now() - start;
      const minDelay = 400; // milliseconds

      if (elapsed < minDelay) {
        await new Promise((resolve) => setTimeout(resolve, minDelay - elapsed));
      }
      setTestingToken(false);

      if (valid) {
        setRemoteTokenError(null);
        setConnectionStatus({ status: "ok", color: GetConnectionColor("ok") });
        if (media_server && media_server.user_id && media_server.user_id !== current.user_id) {
          onChange("user_id", media_server.user_id);
        }
      } else {
        setRemoteTokenError(message || "Token invalid");
        setConnectionStatus({ status: "error", color: GetConnectionColor("error") });
      }
    },
    [onChange]
  );

  useEffect(() => {
    if (configAlreadyLoaded && !hasRunInitialValidation.current) {
      runRemoteValidation(false); // No toast on initial load
      hasRunInitialValidation.current = true;
    }
  }, [configAlreadyLoaded, runRemoteValidation]);

  const removeLibraryByIndex = (index: number) => {
    onChange(
      "libraries",
      libraries.filter((_, i) => i !== index)
    );
  };

  const replaceLibraries = (names: string[]) => {
    onChange(
      "libraries",
      names.map((n) => ({ title: n, id: "", type: "" }))
    );
  };

  const fetchServerLibraries = async () => {
    if (!editing || libraryFetchLoading) return;
    setLibraryFetchLoading(true);
    const resp = await GetLibrarySectionOptions(value);
    setLibraryFetchLoading(false);
    if (resp.status !== "success" || !Array.isArray(resp.data?.library_sections)) {
      return;
    }
    replaceLibraries(resp.data.library_sections.map((lib) => lib.title));
  };

  // Plex OAuth Flow (Get PIN and ID)
  const handleGetPlexPinAndID = async () => {
    const resp = await GetPlexPinAndID();
    if (resp.status !== "success" || !resp.data?.plex_id || !resp.data?.plex_pin) {
      return;
    }
    const { plex_id, plex_pin } = resp.data;
    setPlexPIN(plex_pin);
    setPlexID(plex_id);
  };

  // Plex OAuth Flow (Open Plex OAuth Window)
  const handleOpenPlexOAuthWindow = () => {
    if (plexID === "" || plexPIN === "") return;
    const plexOAuthURL = `https://app.plex.tv/auth/#?clientID=${PLEX_CLIENT_ID}&code=${plexPIN}&context%5Bdevice%5D%5Bproduct%5D=${PLEX_PRODUCT}`;
    const width = 500;
    const height = 700;
    const left = window.screenX + (window.outerWidth - width) / 2;
    const top = window.screenY + (window.outerHeight - height) / 2.5;

    const oauthWindow = window.open(
      plexOAuthURL,
      "Plex OAuth",
      `width=${width},height=${height},left=${left},top=${top},resizable=yes,scrollbars=yes,status=yes`
    );
    if (oauthWindow) {
      setPlexOAuthWindow(oauthWindow);
    }
  };

  // Plex OAuth Flow (Poll for OAuth Completion)
  useEffect(() => {
    if (plexID === "" || plexPIN === "" || !plexOAuthWindow) return;

    const interval = setInterval(async () => {
      const resp = await CheckAuthStatusWithPlex(plexID);
      if (resp.status !== "success" || !resp.data) {
        return;
      }
      const { authenticated, auth_token, connections_available } = resp.data;
      if (authenticated) {
        // Set the token and available connections
        onChange("api_token", auth_token || "");
        setPlexConnectionsAvailable(connections_available || []);
        setSignInWithPlex(true);
        // Close the OAuth window
        if (plexOAuthWindow && !plexOAuthWindow.closed) {
          plexOAuthWindow.close();
        }
        setPlexOAuthWindow(null);
      }
    }, 3000);

    return () => clearInterval(interval);
  }, [plexID, plexPIN, plexOAuthWindow, onChange]);

  let displayValue = "Select server connection...";
  if (value.url) {
    try {
      const url = new URL(value.url);
      const showPort = url.port && url.port !== "443";
      displayValue = showPort ? `${url.hostname}:${url.port}` : url.hostname;
    } catch {
      displayValue = value.url;
    }
  }

  return (
    <Card className={`p-5 ${Object.values(errors).some(Boolean) ? "border-red-500" : "border-muted"}`}>
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <h2 className={`text-xl font-semibold text-${connectionStatus.color}`}>Media Server</h2>
          <span
            className={`h-2 w-2 rounded-full ${CONNECTION_STATUS_COLORS_BG[connectionStatus.status]} animate-pulse`}
            title={`Connection status: ${connectionStatus.status}`}
          />
        </div>
        <Button
          variant="outline"
          size="sm"
          hidden={editing}
          disabled={editing || testingToken}
          onClick={() => runRemoteValidation()}
          className="cursor-pointer hover:text-primary"
        >
          {testingToken ? "Testing..." : "Test Connection"}
        </Button>
      </div>

      {/* Type */}
      <div className={cn("space-y-1")}>
        <div className="flex items-center justify-between">
          <Label>Type</Label>
          {editing && (
            <PopoverHelp ariaLabel="help-media-server-type">
              <p>The type of Media Server </p>
              <ul className="list-disc list-inside mt-1">
                <li>Plex</li>
                <li>Emby</li>
                <li>Jellyfin</li>
              </ul>
            </PopoverHelp>
          )}
        </div>
        <Select
          disabled={!editing}
          value={value.type}
          onValueChange={(v) => {
            value.url = "";
            value.api_token = "";
            value.user_id = "";
            value.libraries = [];
            setPlexID("");
            setPlexPIN("");
            setPlexConnectionsAvailable([]);
            setSignInWithPlex(false);
            onChange("type", v);
            if (v === "Plex") {
              handleGetPlexPinAndID();
            }
          }}
        >
          <SelectTrigger
            id="media-server-type-trigger"
            className={cn("w-full", dirtyFields.type && "border-amber-500")}
          >
            <SelectValue placeholder="Select type..." />
          </SelectTrigger>
          <SelectContent>
            {SERVER_TYPES.map((t) => (
              <SelectItem className="cursor-pointer" key={t} value={t}>
                {t}
              </SelectItem>
            ))}
            <SelectScrollUpButton />
            <SelectScrollDownButton />
          </SelectContent>
        </Select>
        {errors.type && <p className="text-xs text-red-500">{errors.type}</p>}
      </div>

      {/* If the Type is Plex, then show a Sign in with Plex button here */}
      {editing && value.type === "Plex" && plexPIN && plexID && plexConnectionsAvailable.length === 0 && (
        <div className="flex flex-col items-center">
          <div
            className="flex flex-row items-center justify-center rounded-lg"
            style={{
              background: "#e5a00d 100%",
              padding: "0.2rem 0.5rem",
              margin: "1rem 0",
              maxWidth: "320px",
              marginLeft: "auto",
              marginRight: "auto",
            }}
            onClick={() => {
              handleOpenPlexOAuthWindow();
            }}
          >
            <img src="/plex-icon.png" alt="Plex" className="mr-3 w-8 h-8" />
            <Button
              variant="ghost"
              size="sm"
              disabled={!editing}
              onClick={() => {
                handleOpenPlexOAuthWindow();
              }}
              className="font-semibold text-black border-none hover:text-white transition-colors"
            >
              Sign in with Plex
            </Button>
          </div>
          {!signInWithPlex && (
            <span className="text-xs text-center text-muted-foreground">or Enter URL and Token manually</span>
          )}
        </div>
      )}

      {/* URL */}
      {signInWithPlex && value.type === "Plex" && plexConnectionsAvailable.length > 0 ? (
        <div className={cn("space-y-1")}>
          <div className="flex items-center justify-between">
            <Label>Server Connection</Label>
            {editing && (
              <PopoverHelp ariaLabel="help-media-server-connection">
                <p>Select the Plex server connection to use.</p>
              </PopoverHelp>
            )}
          </div>

          <Select
            disabled={!editing}
            value={value.url}
            onValueChange={(v) => {
              onChange("url", v);
            }}
          >
            <SelectTrigger
              id="media-server-connection-trigger"
              className={cn("w-full", dirtyFields.url && "border-amber-500")}
            >
              <SelectValue placeholder={displayValue} />
            </SelectTrigger>

            <SelectContent>
              <SelectScrollUpButton />
              {plexConnectionsAvailable.flatMap((server, serverIndex) =>
                (server.connections || []).map((c, connIndex) => {
                  const showPort = c.port && c.port !== 443;
                  const label = !showPort ? c.uri.replace(/:\d+$/, "") : c.uri;

                  return (
                    <SelectItem
                      className="cursor-pointer"
                      key={`${serverIndex}-${connIndex}-${label}`}
                      value={!showPort ? c.uri.replace(/:\d+$/, "") : c.uri}
                    >
                      <div className="flex items-center gap-2">
                        <span className="block text-xs ">{label}</span>{" "}
                        {c.local && (
                          <span className="ml-1 px-1.5 py-0.5 rounded bg-green-100 text-green-700 text-[10px] font-semibold">
                            Local
                          </span>
                        )}
                        {c.relay && (
                          <span className="ml-1 px-1.5 py-0.5 rounded bg-yellow-100 text-yellow-700 text-[10px] font-semibold">
                            Relay
                          </span>
                        )}
                      </div>
                    </SelectItem>
                  );
                })
              )}
              <SelectScrollDownButton />
            </SelectContent>
          </Select>

          {errors.url && <p className="text-xs text-red-500">{errors.url}</p>}
        </div>
      ) : (
        <div className={cn("space-y-1")}>
          <div className="flex items-center justify-between">
            <Label>URL</Label>
            {editing && (
              <PopoverHelp ariaLabel="help-media-server-url">
                <p className="font-medium mb-1">Base server URL</p>
                <p>Examples:</p>
                <ul className="list-disc list-inside mb-1">
                  <li>https://{value.type.toLowerCase()}.domain.com</li>
                  <li>http://192.168.1.10:{value.type === "Plex" ? "32400" : "8096"}</li>
                  <li>
                    http://{value.type.toLowerCase()}:{value.type === "Plex" ? "32400" : "8096"}
                  </li>
                </ul>
                <p>Rules:</p>
                <ul className="list-disc list-inside">
                  <li>Must be a valid URL</li>
                  <li>Must include http:// or https://</li>
                  <li>IPv4 addresses must include a port</li>
                </ul>
              </PopoverHelp>
            )}
          </div>

          <Input
            disabled={!editing}
            placeholder="https://server.example.com"
            value={value.url}
            onChange={(e) => onChange("url", e.target.value)}
            onBlur={() => runRemoteValidation()}
            className={cn("w-full", dirtyFields.url && "border-amber-500")}
          />

          {errors.url && <p className="text-xs text-red-500">{errors.url}</p>}
        </div>
      )}

      {/* Token */}

      <div className={cn("space-y-1")}>
        <div className="flex items-center justify-between">
          <Label>Token</Label>
          {editing && (
            <PopoverHelp ariaLabel="help-media-server-token">
              <p>API token used to authenticate requests to the media server.</p>
            </PopoverHelp>
          )}
        </div>
        <Input
          disabled={!editing || (value.type === "Plex" && signInWithPlex)}
          type="text"
          placeholder="API token"
          value={value.api_token}
          onChange={(e) => onChange("api_token", e.target.value)}
          onBlur={() => runRemoteValidation()}
          className={cn("w-full", dirtyFields.api_token && "border-amber-500")}
        />
        {errors.api_token && <p className="text-xs text-red-500">{errors.api_token}</p>}
      </div>

      {/* User ID (Emby / Jellyfin) */}
      {USER_ID_REQUIRED_TYPES.has(value.type) && (
        <div className={cn("space-y-1")}>
          <div className="flex items-center justify-between">
            <Label>User ID</Label>
            {editing && (
              <PopoverHelp ariaLabel="help-media-server-user-id">
                <p>
                  User ID is required for Emby/Jellyfin. It should be set automatically after URL & Token are valid.
                </p>
              </PopoverHelp>
            )}
          </div>
          <Input
            disabled={true}
            value={value.user_id ?? ""}
            placeholder="Emby/Jellyfin user id"
            className={cn("w-full", dirtyFields.user_id && "border-amber-500")}
          />
          {errors.user_id && <p className="text-xs text-red-500">{errors.user_id}</p>}
        </div>
      )}

      {/* Libraries */}
      <div className={cn("space-y-3")}>
        <div className="flex items-center">
          <Label>Libraries</Label>
          {editing && (
            <span className="flex items-center gap-2 ml-3">
              <Button
                onClick={fetchServerLibraries}
                variant="outline"
                size="icon"
                className="h-7 w-7"
                title="Refresh libraries from server"
                disabled={libraryFetchLoading}
              >
                {libraryFetchLoading ? (
                  <RefreshCcw className="h-4 w-4 animate-spin" />
                ) : (
                  <RefreshCcw className="h-4 w-4" />
                )}
              </Button>
            </span>
          )}
        </div>

        <div className="flex flex-wrap gap-2">
          {libraries.length === 0 && (
            <span className="inline-flex h-7 items-center rounded-full border border-dashed px-3 text-xs text-muted-foreground">
              No libraries added
            </span>
          )}
          {libraries.map((lib, i) => (
            <Badge
              key={i}
              className={cn(
                "cursor-pointer select-none transition duration-200 px-3 py-1 text-xs font-normal",
                editing
                  ? "bg-secondary text-secondary-foreground hover:bg-red-500 hover:text-white"
                  : "bg-muted text-muted-foreground",
                dirtyFields.libraries && "border-amber-500"
              )}
              onClick={() => {
                if (!editing) return;
                removeLibraryByIndex(i);
              }}
              title={editing ? "Remove library" : lib.title}
            >
              {lib.title}
            </Badge>
          ))}
        </div>
        {errors.libraries && <p className="text-xs text-red-500">{errors.libraries}</p>}
      </div>

      {/* Enable Sort by Episode Added Date */}
      {value.type === "Plex" && (
        <div
          className={cn(
            "flex items-center justify-between border rounded-md p-3 transition",
            "border-muted",
            dirtyFields.enable_sort_by_episode_added_date && "border-amber-500"
          )}
        >
          <Label>Enable "Sort by Episode Added Date"</Label>
          <div className="flex items-center gap-2">
            <Switch
              disabled={!editing}
              checked={value.enable_sort_by_episode_added_date}
              onCheckedChange={(c) => onChange("enable_sort_by_episode_added_date", c)}
            />
            {editing && (
              <PopoverHelp ariaLabel="help-sort-new-episode">
                <p className="mb-2">
                  When enabled, the <b>New Episode Added</b> sort option will be available on the Home page.
                </p>
                <p className="text-muted-foreground">
                  This option requires an extra request to your media server to fetch the latest episode dates for all
                  shows. Disabling it can significantly speed up initial page loading for large libraries.
                </p>
              </PopoverHelp>
            )}
          </div>
        </div>
      )}

      {/* Plex Websocket Listener */}
      {value.type === "Plex" && (
        <div
          className={cn(
            "flex items-center justify-between border rounded-md p-3 transition",
            "border-muted",
            dirtyFields.enable_plex_event_listener && "border-amber-500"
          )}
        >
          <Label>Enable Plex Websocket Listener</Label>
          <div className="flex items-center gap-2">
            <Switch
              disabled={!editing}
              checked={value.enable_plex_event_listener}
              onCheckedChange={(c) => onChange("enable_plex_event_listener", c)}
            />
            {editing && (
              <PopoverHelp ariaLabel="help-plex-websocket-listener">
                <p className="mb-2">
                  When enabled, Aura will listen for Plex library refresh events and automatically reapply saved images
                  to refreshed items.
                </p>
                <p className="text-muted-foreground">
                  This feature is exclusive to Plex and requires an extra websocket connection from Aura to your Plex
                  server. It can be safely disabled if you don't want this functionality or if you experience any
                  issues.
                </p>
              </PopoverHelp>
            )}
          </div>
        </div>
      )}
    </Card>
  );
};
