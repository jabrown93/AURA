import { ValidateURL } from "@/helper/validation/validate-url";
import { ValidateSonarrRadarrInfo } from "@/services/validation/sonarr-radarr";
import { Plus, TestTube, Trash2 } from "lucide-react";

import React, { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { GetConnectionColor } from "@/components/settings-onboarding/ConfigSectionMediaServer";
import { PopoverHelp } from "@/components/shared/popover-help";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";

import { cn } from "@/lib/cn";

import type {
  AppConfigMediaServerLibrary,
  AppConfigSonarrRadarrApp,
  AppConfigSonarrRadarrApps,
} from "@/types/config/config";

interface ConfigSectionSonarrRadarrProps {
  value: AppConfigSonarrRadarrApps;
  editing: boolean;
  dirtyFields?: {
    applications?: Array<Partial<{ type: boolean; library: boolean; url: boolean; api_token: boolean }>>;
  };
  onChange: <K extends keyof AppConfigSonarrRadarrApps>(field: K, value: AppConfigSonarrRadarrApps[K]) => void;
  errorsUpdate?: (errors: Record<string, string>) => void;
  configAlreadyLoaded: boolean;
  onConnectionVerifiedChange?: (verified: boolean, checking: boolean) => void; // Reports whether the connection has been checked and works
  libraries: AppConfigMediaServerLibrary[];
}

const SR_TYPES = ["Sonarr", "Radarr"] as const;

export type ConfigConnectionStatus = {
  status: "ok" | "error" | "unknown";
  color: "green-500" | "red-500" | "gray-500";
};

export const CONNECTION_STATUS_COLORS_BG: Record<string, string> = {
  ok: "bg-green-500",
  error: "bg-red-500",
  unknown: "bg-gray-400",
};

export const ConfigSectionSonarrRadarr: React.FC<ConfigSectionSonarrRadarrProps> = ({
  value,
  editing,
  dirtyFields = {},
  onChange,
  errorsUpdate,
  configAlreadyLoaded,
  onConnectionVerifiedChange,
  libraries,
}) => {
  const prevErrorsRef = useRef<string>("");
  const hasRunInitialValidation = useRef(false);

  // Local select state for adding new application
  const [newAppType, setNewAppType] = useState<string>("Sonarr");

  const apps = useMemo(() => (Array.isArray(value.applications) ? value.applications : []), [value.applications]);

  // State to track app connection testing
  const [appConnectionStatus, setAppConnectionStatus] = useState<Record<number, ConfigConnectionStatus>>({});
  // Connections that have been checked and work, by URL and API key
  const [verifiedConnections, setVerifiedConnections] = useState<Set<string>>(() => new Set());
  const [checkingConnections, setCheckingConnections] = useState(0);
  const [remoteTokenErrors, setRemoteTokenErrors] = useState<Record<number, string | null>>({});

  // ----- Validation -----
  const errors = useMemo(() => {
    const errs: Record<string, string> = {};

    apps.forEach((app, index) => {
      if (!app.type) {
        errs[`Applications.[${index}].Type`] = "Type is required";
      } else {
        if (!SR_TYPES.includes(app.type as (typeof SR_TYPES)[number])) {
          errs[`Applications.[${index}].Type`] = "Invalid Type, must be Sonarr or Radarr";
        }
      }
      if (!app.library) {
        errs[`Applications.[${index}].Library`] = "Library is required";
      }
      if (!app.url) {
        const rawURL = (app.url || "").trim();
        if (!rawURL) errs[`Applications.[${index}].URL`] = "URL is required";
        else {
          const urlErr = ValidateURL(rawURL);
          if (urlErr) errs[`Applications.[${index}].URL`] = urlErr;
        }
      }
      if (!app.api_token) {
        errs[`Applications.[${index}].APIToken`] = "API Token is required";
      }
      // Remote error (overrides local message if present)
      if (remoteTokenErrors[index]) {
        errs[`Applications.[${index}].APIToken`] = remoteTokenErrors[index] || "Connection failed";
      }
    });

    return errs;
  }, [apps, remoteTokenErrors]);

  // Emit errors if changed
  useEffect(() => {
    if (!errorsUpdate) return;
    const serialized = JSON.stringify(errors);
    if (serialized === prevErrorsRef.current) return;
    prevErrorsRef.current = serialized;
    errorsUpdate(errors);
  }, [errors, errorsUpdate]);

  // ----- Mutators -----
  const setApps = (next: AppConfigSonarrRadarrApp[]) => onChange("applications", next);

  const addApp = () => {
    if (!editing) return;
    const type = newAppType as (typeof SR_TYPES)[number];
    let newEntry: AppConfigSonarrRadarrApp;
    if (type === "Sonarr") {
      newEntry = { type: "Sonarr", library: "", url: "", api_token: "" };
    } else {
      newEntry = { type: "Radarr", library: "", url: "", api_token: "" };
    }
    setApps([...apps, newEntry]);
  };

  const removeApp = (idx: number) => {
    if (!editing) return;
    const next = apps.slice();
    next.splice(idx, 1);
    setApps(next);
  };

  const updateApp = <K extends keyof AppConfigSonarrRadarrApp>(
    idx: number,
    field: K,
    value: AppConfigSonarrRadarrApp[K]
  ) => {
    if (!editing) return;
    const next = apps.slice();
    next[idx] = { ...next[idx], [field]: value };
    setApps(next);
  };

  const runRemoteValidation = useCallback(
    async (idx: number, showToast = true) => {
      const app = apps[idx];
      if (!app || !app.url || !app.api_token) return;

      // Set to unknown while testing
      setAppConnectionStatus((s) => ({
        ...s,
        [idx]: { status: "unknown", color: GetConnectionColor("unknown") },
      }));
      const connectionKey = `${app.url}|${app.api_token}`;
      setCheckingConnections((n) => n + 1);

      try {
        const start = Date.now();
        const { ok, message } = await ValidateSonarrRadarrInfo(app, showToast);
        const elapsed = Date.now() - start;
        const minDelay = 400; // milliseconds

        if (elapsed < minDelay) {
          await new Promise((resolve) => setTimeout(resolve, minDelay - elapsed));
        }

        if (ok) {
          setRemoteTokenErrors((s) => ({ ...s, [idx]: null }));
          setAppConnectionStatus((s) => ({ ...s, [idx]: { status: "ok", color: GetConnectionColor("ok") } }));
          setVerifiedConnections((prev) => new Set(prev).add(connectionKey));
        } else {
          setRemoteTokenErrors((s) => ({ ...s, [idx]: message || "Connection failed" }));
          setAppConnectionStatus((s) => ({
            ...s,
            [idx]: { status: "error", color: GetConnectionColor("error") },
          }));
        }
      } catch {
        setRemoteTokenErrors((s) => ({ ...s, [idx]: "Connection failed" }));
        setAppConnectionStatus((s) => ({
          ...s,
          [idx]: { status: "error", color: GetConnectionColor("error") },
        }));
      }
      setCheckingConnections((n) => n - 1);
    },
    [apps]
  );

  // Report whether every added app's connection has been checked and works
  useEffect(() => {
    const addedApps = apps.filter((app) => app.url?.trim() || app.api_token?.trim());
    const verified = addedApps.every((app) => verifiedConnections.has(`${app.url}|${app.api_token}`));
    onConnectionVerifiedChange?.(verified && checkingConnections === 0, checkingConnections > 0);
  }, [apps, verifiedConnections, checkingConnections, onConnectionVerifiedChange]);

  useEffect(() => {
    if (configAlreadyLoaded && !hasRunInitialValidation.current) {
      // Run remote validation for all apps that have URL and APIKey set
      apps.forEach((app, idx) => {
        if (app.url && app.api_token) {
          // Delay slightly to allow UI to settle
          setTimeout(() => {
            runRemoteValidation(idx, false);
          }, 200 * idx);
        }
      });
      hasRunInitialValidation.current = true;
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [configAlreadyLoaded, runRemoteValidation]);

  // ----- Render -----
  return (
    <Card className={`gap-4 p-4 sm:p-5 ${Object.values(errors).some(Boolean) ? "border-red-500" : "border-muted"}`}>
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h2 className="text-xl font-semibold text-blue-500 whitespace-nowrap">Sonarr & Radarr</h2>
        <div className="flex items-center">
          {editing && libraries.length > 0 && (
            <div className="flex items-center gap-2">
              <Select value={newAppType} onValueChange={(v) => setNewAppType(v)}>
                <SelectTrigger className="h-8 w-36">
                  <SelectValue placeholder="Type" />
                </SelectTrigger>
                <SelectContent>
                  {SR_TYPES.map((sr) => (
                    <SelectItem key={sr} value={sr}>
                      {sr}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <Button type="button" variant="outline" size="sm" onClick={addApp}>
                <Plus className="h-4 w-4 mr-1" />
                Add
              </Button>
            </div>
          )}
        </div>
      </div>

      {/* Applications */}
      <div className={cn("space-y-3", "rounded-md")}>
        {libraries.length === 0 ? (
          <p className="text-sm text-red-500">
            No Media Server libraries found. Please configure your Media Server first.
          </p>
        ) : apps.length === 0 ? (
          <p className="text-sm text-muted-foreground">No applications added yet.</p>
        ) : (
          <>
            {apps.map((app, idx) => {
              const appDirty = dirtyFields.applications?.[idx] as Partial<{
                type: string;
                library: string;
                url: string;
                api_token: string;
              }>;
              const appErrorEntries = Object.entries(errors).filter(([k]) => k.startsWith(`Applications.[${idx}]`));
              const appErrors = appErrorEntries.map(([, msg]) => msg);

              // Field-level helpers (key-based)
              const hasError = (suffix: string) => appErrorEntries.some(([k]) => k.endsWith(suffix));
              const statusObj = appConnectionStatus[idx] || {
                status: "unknown",
                color: "gray-500",
              };
              return (
                <div
                  key={idx}
                  className={cn(
                    "space-y-3 rounded-md border p-3 transition",
                    appErrors.length
                      ? "border-red-500"
                      : appDirty && Object.values(appDirty).some(Boolean)
                        ? "border-amber-500"
                        : "border-muted"
                  )}
                >
                  <div className="flex items-center justify-between gap-3">
                    <div className="flex items-center gap-2">
                      <h2 className={`text-xl font-semibold text-${statusObj.color}`}>{app.type}</h2>
                      <span
                        className={`h-2 w-2 rounded-full ${CONNECTION_STATUS_COLORS_BG[statusObj.status]} animate-pulse`}
                        title={`Connection status: ${statusObj.status}`}
                      />
                    </div>
                    <div className="flex items-center gap-2">
                      <Button
                        variant="outline"
                        size="sm"
                        disabled={!app.url || !app.api_token}
                        hidden={!app.api_token && !app.url}
                        onClick={() => {
                          runRemoteValidation(idx);
                        }}
                        aria-label="test-app-connection"
                      >
                        <TestTube className="h-4 w-4 mr-1" />{" "}
                      </Button>
                      {editing && (
                        <Button
                          variant="ghost"
                          size="icon"
                          onClick={() => removeApp(idx)}
                          aria-label="help-apps-remove-app"
                          className="bg-red-700"
                        >
                          <Trash2 className="h-4 w-4" />
                        </Button>
                      )}
                    </div>
                  </div>

                  {/* Library Field */}
                  <div className={cn("space-y-1", "rounded-md")}>
                    <div className="flex items-center justify-between">
                      <Label>Library</Label>
                      {editing && (
                        <PopoverHelp ariaLabel="help-apps-sr-library">
                          <p className="mb-2 font-medium">Library</p>
                          <p>The Media Server library that this {app.type} instance will manage.</p>
                        </PopoverHelp>
                      )}
                    </div>
                    <Select
                      disabled={!editing}
                      value={app.library || ""}
                      onValueChange={(v) => updateApp(idx, "library", v)}
                    >
                      <SelectTrigger
                        className={cn(
                          "h-8",
                          !app.library && "text-muted-foreground",
                          !hasError("library") && appDirty?.library && "rounded-md border border-amber-500 p-3"
                        )}
                      >
                        <SelectValue placeholder={libraries.length === 0 ? "No libraries found" : "Select library"} />
                      </SelectTrigger>
                      {libraries.length > 0 && (
                        <SelectContent>
                          {libraries.map((lib) => (
                            <SelectItem key={lib.title} value={lib.title}>
                              {lib.title}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      )}
                    </Select>
                    {appErrorEntries
                      .filter(([k]) => k.endsWith("Library"))
                      .map(([, msg], i) => (
                        <p key={i} className="text-xs text-red-500">
                          {msg}
                        </p>
                      ))}
                  </div>

                  {/* URL Field */}
                  <div className={cn("space-y-1", "rounded-md")}>
                    <div className="flex items-center justify-between">
                      <Label>URL</Label>
                      {editing && (
                        <PopoverHelp ariaLabel="help-apps-sr-url">
                          <p className="font-medium mb-1">App URL</p>
                          <p>Examples:</p>
                          <ul className="list-disc list-inside mb-1">
                            <li>https://{app.type.toLowerCase()}.domain.com</li>
                            <li>
                              http://192.168.1.10:
                              {app.type === "Sonarr" ? "8989" : "7878"}
                            </li>
                            <li>
                              http://{app.type.toLowerCase()}:{app.type === "Sonarr" ? "8989" : "7878"}
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
                      placeholder="http://app.domain.com"
                      value={app.url || ""}
                      onChange={(e) => updateApp(idx, "url", e.target.value)}
                      className={cn(appDirty?.url && "rounded-md border border-amber-500 p-3")}
                    />
                    {appErrorEntries
                      .filter(([k]) => k.endsWith("url"))
                      .map(([, msg], i) => (
                        <p key={i} className="text-xs text-red-500">
                          {msg}
                        </p>
                      ))}
                  </div>

                  {/* API Key Field */}
                  <div className={cn("space-y-1", "rounded-md")}>
                    <div className="flex items-center justify-between">
                      <Label>API Key</Label>
                      {editing && (
                        <PopoverHelp ariaLabel="help-apps-sr-apikey">
                          <p className="mb-2 font-medium">API Key</p>
                          <p>The API Key for your {app.type} instance.</p>
                          <p>This can usually be found in the settings of your application.</p>
                        </PopoverHelp>
                      )}
                    </div>
                    <Input
                      disabled={!editing}
                      placeholder="API Key"
                      value={app.api_token || ""}
                      onChange={(e) => updateApp(idx, "api_token", e.target.value)}
                      onBlur={() => {
                        runRemoteValidation(idx);
                      }}
                      className={cn(appDirty?.api_token && "rounded-md border border-amber-500 p-3")}
                    />
                    {appErrorEntries
                      .filter(([k]) => k.endsWith("api_token"))
                      .map(([, msg], i) => (
                        <p key={i} className="text-xs text-red-500">
                          {msg}
                        </p>
                      ))}
                  </div>
                </div>
              );
            })}{" "}
          </>
        )}
      </div>
    </Card>
  );
};
