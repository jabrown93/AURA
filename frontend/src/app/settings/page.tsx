"use client";

import { ReturnErrorMessage } from "@/services/api-error-return";
import { GetAppConfigStatus } from "@/services/config/status";
import { GetNotificationTemplateVariables } from "@/services/config/template-variables";
import { UpdateAppConfig } from "@/services/config/update";
import { Edit, SaveIcon, X } from "lucide-react";
import { toast } from "sonner";

import { useEffect, useRef, useState } from "react";

import { useRouter } from "next/navigation";

import { ConfigSectionAuth } from "@/components/settings-onboarding/ConfigSectionAuth";
import { ConfigSectionAutoDownload } from "@/components/settings-onboarding/ConfigSectionAutoDownload";
import { ConfigSectionImages } from "@/components/settings-onboarding/ConfigSectionImages";
import { ConfigSectionLabelsAndTags } from "@/components/settings-onboarding/ConfigSectionLabelsAndTags";
import { ConfigSectionLogging } from "@/components/settings-onboarding/ConfigSectionLogging";
import { ConfigSectionMediaServer } from "@/components/settings-onboarding/ConfigSectionMediaServer";
import { ConfigSectionMediux } from "@/components/settings-onboarding/ConfigSectionMediux";
import { ConfigSectionNotifications } from "@/components/settings-onboarding/ConfigSectionNotifications";
import { ConfigSectionSonarrRadarr } from "@/components/settings-onboarding/ConfigSectionSonarrRadarr";
import { UserPreferencesCard } from "@/components/settings-onboarding/UserPreferences";
import { ConfirmDestructiveDialogActionButton } from "@/components/shared/dialog-destructive-action";
import { ErrorMessage } from "@/components/shared/error-message";
import Loader from "@/components/shared/loader";
import { Button } from "@/components/ui/button";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";

import { cn } from "@/lib/cn";
import { ClearAllStores } from "@/lib/stores/clear-all-stores";

import type { APIResponse } from "@/types/api/api-response";
import type {
  AppConfig,
  AppConfigNotificationDiscord,
  AppConfigNotificationGotify,
  AppConfigNotificationPushover,
  AppConfigNotificationWebhook,
  NotificationTemplateVariablesCatalog,
} from "@/types/config/config";
import { defaultAppConfig } from "@/types/config/config-default-app";

type ObjectSectionKeys = {
  [K in keyof AppConfig]-?: NonNullable<AppConfig[K]> extends object ? K : never;
}[keyof AppConfig];

type SectionDirty<S extends ObjectSectionKeys = ObjectSectionKeys> = S extends "sonarr_radarr"
  ? Partial<{ type: boolean; library: boolean; url: boolean; api_token: boolean }>
  : Partial<Record<keyof NonNullable<AppConfig[S]>, boolean>>;

interface ImagesDirty {
  cache_images?: { enabled?: boolean };
  save_images_locally?: { enabled?: boolean; path?: string };
  kometa?: { enabled?: boolean; asset_directory?: string; library_asset_folders?: boolean; import_cron?: string };
}

type NotificationsDirty = {
  enabled?: boolean;
  providers?: Array<
    Partial<
      Record<
        string,
        | boolean
        | {
            enabled?: boolean;
            webhook?: boolean;
            user_key?: boolean;
            api_token?: boolean;
            url?: boolean;
            headers?: Record<string, boolean>;
          }
      >
    >
  >;
};

type DirtyState = {
  [K in ObjectSectionKeys]?: K extends "images"
    ? ImagesDirty
    : K extends "sonarr_radarr"
      ? Array<SectionDirty<"sonarr_radarr">>
      : K extends "notifications"
        ? NotificationsDirty
        : SectionDirty<K>;
};

type ValidationErrors = {
  [K in ObjectSectionKeys]?: Record<string, string>; // field -> message
};

const SettingsPage: React.FC = () => {
  const router = useRouter();
  const isMounted = useRef(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<APIResponse<unknown> | null>(null);

  const [editing, setEditing] = useState(false);
  const [saving, setSaving] = useState(false);

  const [initialConfig, setInitialConfig] = useState<AppConfig>(() => defaultAppConfig());
  const [newConfig, setNewConfig] = useState<AppConfig>(() => defaultAppConfig());
  const [notificationTemplateVariables, setNotificationTemplateVariables] =
    useState<NotificationTemplateVariablesCatalog | null>(null);
  const [dirty, setDirty] = useState<DirtyState>({});

  const [validationErrors, setValidationErrors] = useState<ValidationErrors>({});

  const preferencesRef = useRef<HTMLDivElement>(null);

  const [activeTab, setActiveTab] = useState("app-settings");

  // State - Debug Mode
  const [debugEnabled, setDebugEnabled] = useState(false);

  useEffect(() => {
    const savedMode = localStorage.getItem("debugMode") === "true";
    setDebugEnabled(savedMode);
  }, []);

  useEffect(() => {
    const scrollToPreferences = () => {
      if (loading) return;
      if (window.location.hash === "#preferences-section") {
        preferencesRef.current?.scrollIntoView({ behavior: "smooth" });
      }
    };
    window.addEventListener("hashchange", scrollToPreferences);
    // Run on mount
    scrollToPreferences();
    return () => window.removeEventListener("hashchange", scrollToPreferences);
  }, [loading]);

  const toggleDebugMode = (checked: boolean) => {
    setDebugEnabled(checked);
    localStorage.setItem("debugMode", checked.toString());
  };

  const fetchAndSetConfig = async (reload: boolean = false) => {
    try {
      setLoading(true);
      const response = await GetAppConfigStatus(reload);
      const templateVariableResponse = await GetNotificationTemplateVariables();
      if (response.status === "error") {
        setError(response);
        setInitialConfig(defaultAppConfig());
        setNewConfig(defaultAppConfig());
        return;
      }

      if (templateVariableResponse.status === "success" && templateVariableResponse.data?.variables) {
        setNotificationTemplateVariables(templateVariableResponse.data.variables);
      }

      const cfg = response.data?.status.current_setup ?? defaultAppConfig();
      setInitialConfig(cfg);
      setNewConfig(cfg);
      setError(null);
    } catch (error) {
      setError(ReturnErrorMessage<AppConfig>(error));
      setInitialConfig(defaultAppConfig());
      setNewConfig(defaultAppConfig());
    } finally {
      setLoading(false);
    }
  };

  // Fetch configuration data on mount
  useEffect(() => {
    if (isMounted.current) return;
    isMounted.current = true;

    fetchAndSetConfig();
  }, []);

  const handleCancel = () => {
    setEditing(false);
    setSaving(false);
    setDirty({});
    setNewConfig(initialConfig); // <- reset edits
  };

  const handleSaveAll = async () => {
    if (!newConfig) return;

    setSaving(true);

    setEditing(false);

    try {
      const resp = await UpdateAppConfig(newConfig);

      if (resp.status === "error") {
        setError(resp);
      } else if (resp.status === "warn") {
        toast.warning("No changes detected.");
      } else {
        toast.success("Configuration updated successfully.");
        window.location.reload();
      }
    } catch (error) {
      toast.error(
        typeof error === "object" && error !== null && "message" in error
          ? (error as { message?: string }).message || "Unknown error"
          : "Unknown error"
      );
    }
    setSaving(false);
    setDirty({});
  };

  const structuralEqual = (a: unknown, b: unknown): boolean => {
    if (a === b) return true;
    if (Array.isArray(a) && Array.isArray(b)) {
      if (a.length !== b.length) return false;
      for (let i = 0; i < a.length; i++) {
        if (!structuralEqual(a[i], b[i])) return false;
      }
      return true;
    }
    if (a && b && typeof a === "object" && typeof b === "object") {
      const keysA = Object.keys(a as object);
      const keysB = Object.keys(b as object);
      if (keysA.length !== keysB.length) return false;
      for (const k of keysA) {
        if (!structuralEqual((a as Record<string, unknown>)[k], (b as Record<string, unknown>)[k])) return false;
      }
      return true;
    }
    return false;
  };

  const updateConfigField = <S extends ObjectSectionKeys, F extends keyof AppConfig[S]>(
    section: S,
    field: F,
    value: AppConfig[S][F]
  ) => {
    setNewConfig((prev) => {
      const prevSection = (prev[section] ?? {}) as NonNullable<AppConfig[S]>;
      return {
        ...prev,
        [section]: {
          ...prevSection,
          [field]: value,
        },
      };
    });

    setDirty((prev) => {
      // --- SonarrRadarr dirty tracking ---
      if (section === "sonarr_radarr" && field === "applications") {
        const originalApps = initialConfig.sonarr_radarr.applications ?? [];
        const newApps = (value as AppConfig["sonarr_radarr"]["applications"]) ?? [];
        const dirtyArr = newApps.map((app, idx) => {
          const orig = originalApps[idx];
          const dirtyObj: SectionDirty<"sonarr_radarr"> = {};
          for (const key of ["type", "library", "url", "api_token"] as const) {
            if (app[key] !== orig?.[key]) dirtyObj[key] = true;
          }
          return dirtyObj;
        });
        return { ...prev, sonarr_radarr: dirtyArr };
      }

      // --- Notifications dirty tracking ---
      if (section === "notifications" && field === "providers") {
        const originalProviders = initialConfig.notifications?.providers ?? [];
        const newProviders = value as AppConfig["notifications"]["providers"];
        const dirtyProviders = newProviders.map((prov, idx) => {
          const orig = originalProviders[idx] ?? {};
          const dirtyObj: Partial<
            Record<
              string,
              {
                enabled?: boolean;
                webhook?: boolean;
                user_key?: boolean;
                api_token?: boolean;
                url?: boolean;
                headers?: Record<string, boolean>;
              }
            >
          > = {};

          for (const key of ["discord", "pushover", "gotify", "webhook"] as const) {
            if (prov[key] && orig?.[key]) {
              const fieldDirty: {
                enabled?: boolean;
                webhook?: boolean;
                user_key?: boolean;
                url?: boolean;
                api_token?: boolean;
                headers?: Record<string, boolean>;
              } = {};
              if (key === "discord") {
                const discordKeys = ["enabled", "webhook"] as const;
                for (const subKey of discordKeys) {
                  if (
                    (prov[key] as AppConfigNotificationDiscord)[subKey] !==
                    (orig[key] as AppConfigNotificationDiscord)[subKey]
                  ) {
                    fieldDirty[subKey] = true;
                  }
                }
              } else if (key === "pushover") {
                const pushoverKeys = ["enabled", "user_key", "api_token"] as const;
                for (const subKey of pushoverKeys) {
                  if (
                    (prov[key] as AppConfigNotificationPushover)[subKey] !==
                    (orig[key] as AppConfigNotificationPushover)[subKey]
                  ) {
                    fieldDirty[subKey] = true;
                  }
                }
              } else if (key === "gotify") {
                const gotifyKeys = ["enabled", "api_token", "url"] as const;
                for (const subKey of gotifyKeys) {
                  if (
                    (prov[key] as AppConfigNotificationGotify)[subKey] !==
                    (orig[key] as AppConfigNotificationGotify)[subKey]
                  ) {
                    fieldDirty[subKey] = true;
                  }
                }
              } else if (key === "webhook") {
                const webhookKeys = ["enabled", "url"] as const;
                for (const subKey of webhookKeys) {
                  if (
                    (prov[key] as AppConfigNotificationWebhook)[subKey] !==
                    (orig[key] as AppConfigNotificationWebhook)[subKey]
                  ) {
                    fieldDirty[subKey] = true;
                  }
                }
                // Deep compare Headers
                const newHeaders = (prov[key] as AppConfigNotificationWebhook).headers ?? {};
                const origHeaders = (orig[key] as AppConfigNotificationWebhook).headers ?? {};
                const headersDirty: Record<string, boolean> = {};
                const allHeaderKeys = Array.from(new Set([...Object.keys(newHeaders), ...Object.keys(origHeaders)]));
                for (const hKey of allHeaderKeys) {
                  if (newHeaders[hKey] !== origHeaders[hKey]) {
                    headersDirty[hKey] = true;
                  }
                }
                fieldDirty.headers = Object.keys(headersDirty).length > 0 ? headersDirty : undefined;
              }
              if (Object.keys(fieldDirty).length > 0) {
                dirtyObj[key] = fieldDirty;
              }
            }
          }
          return dirtyObj;
        });
        return {
          ...prev,
          notifications: {
            ...prev.notifications,
            providers: dirtyProviders,
          },
        };
      }

      const originalValue = initialConfig[section]?.[field];
      const reverted: boolean = structuralEqual(originalValue, value);

      const prevSectionDirty = (prev[section] ?? {}) as SectionDirty<S>;
      let nextSectionDirty = prevSectionDirty;

      // Only run for non-SonarrRadarr sections
      if (section !== "sonarr_radarr") {
        // Explicit cast via unknown to satisfy TypeScript
        const dirtyKey = field as unknown as keyof SectionDirty<S>;
        if (reverted) {
          if (prevSectionDirty[dirtyKey]) {
            const clone = { ...prevSectionDirty };
            delete clone[dirtyKey];
            nextSectionDirty = clone;
          }
        } else if (!prevSectionDirty[dirtyKey]) {
          nextSectionDirty = { ...prevSectionDirty, [dirtyKey]: true };
        }

        const nextState: DirtyState = { ...prev };
        if (Object.keys(nextSectionDirty).length === 0) {
          delete nextState[section];
        } else {
          nextState[section] = nextSectionDirty as DirtyState[S];
        }
        return nextState;
      }

      return prev;
    });
  };

  const updateImagesField = <G extends keyof AppConfig["images"], F extends keyof AppConfig["images"][G]>(
    group: G,
    field: F,
    value: AppConfig["images"][G][F]
  ) => {
    setNewConfig((prev) => {
      const nextGroup = {
        ...prev.images[group],
        [field]: value,
      };
      return {
        ...prev,
        images: {
          ...prev.images,
          [group]: nextGroup,
        },
      };
    });

    setDirty((prev) => {
      const originalVal = initialConfig.images[group][field];
      const reverted = originalVal === value;

      const prevImagesDirty = (prev.images ?? {}) as ImagesDirty;
      const prevGroupDirty = (prevImagesDirty[group] ?? {}) as { [k in F]?: boolean };

      let nextGroupDirty = prevGroupDirty;

      if (reverted) {
        if (prevGroupDirty[field]) {
          const clone = { ...prevGroupDirty };
          delete clone[field];
          nextGroupDirty = clone;
        }
      } else if (!prevGroupDirty[field]) {
        nextGroupDirty = { ...prevGroupDirty, [field]: true };
      }

      const nextImagesDirty: ImagesDirty = { ...prevImagesDirty };
      if (Object.keys(nextGroupDirty).length === 0) {
        delete nextImagesDirty[group];
      } else {
        nextImagesDirty[group] = nextGroupDirty;
      }

      const nextState: DirtyState = { ...prev };
      if (Object.keys(nextImagesDirty).length === 0) {
        delete nextState.images;
      } else {
        nextState.images = nextImagesDirty;
      }
      return nextState;
    });
  };

  const anyDirty =
    Object.values(dirty).some((section) => section && Object.values(section).some(Boolean)) ||
    JSON.stringify(initialConfig) !== JSON.stringify(newConfig);

  const updateSectionErrors = <S extends ObjectSectionKeys>(
    section: S,
    errs: Record<string, string> | Partial<Record<string, string>>
  ) => {
    setValidationErrors((prev) => {
      if (!errs || Object.keys(errs).length === 0) {
        const { [section]: _, ...rest } = prev;
        return rest;
      }
      return { ...prev, [section]: errs as Record<string, string> };
    });
  };

  const hasValidationErrors = Object.keys(validationErrors).length > 0;

  return (
    <div className="container mx-auto p-4 sm:p-6 lg:p-8">
      {loading ? (
        <Loader message="Loading configuration..." />
      ) : error ? (
        <ErrorMessage error={error} />
      ) : (
        <>
          {/* If on Dev version (show reload config button) */}
          {process.env.NEXT_PUBLIC_APP_VERSION && process.env.NEXT_PUBLIC_APP_VERSION.endsWith("dev") && (
            <div className="flex justify-center md:justify-end">
              <Button
                variant="ghost"
                onClick={() => fetchAndSetConfig(true)}
                disabled={loading}
                className="cursor-pointer bg-green-500/10 hover:text-primary active:scale-95 hover:brightness-120 mb-4"
              >
                Reload Configuration
              </Button>
            </div>
          )}
          <div className="flex items-center justify-between mb-4">
            <div>
              <h1 className="text-3xl font-bold">
                {activeTab === "user-preferences" ? "User Preferences" : "Settings"}
              </h1>
              <p className="text-gray-600 dark:text-gray-400">
                {activeTab === "user-preferences" ? "Manage your user preferences" : "Manage your application settings"}
              </p>
            </div>
          </div>

          <Tabs defaultValue="app-settings" value={activeTab} onValueChange={setActiveTab} className="w-full">
            <TabsList className="rounded-md p-1 w-full flex">
              <TabsTrigger
                value="app-settings"
                className="flex-1 cursor-pointer text-primary data-[state=active]:bg-primary data-[state=active]:text-background dark:data-[state=active]:bg-primary dark:data-[state=active]:text-background hover:brightness-120 active:scale-95"
              >
                App Settings
              </TabsTrigger>
              <TabsTrigger
                value="user-preferences"
                className="flex-1 cursor-pointer text-primary data-[state=active]:bg-primary data-[state=active]:text-background dark:data-[state=active]:bg-primary dark:data-[state=active]:text-background hover:brightness-120 active:scale-95"
              >
                User Preferences
              </TabsTrigger>
            </TabsList>

            <TabsContent value="app-settings" className="mt-6 w-full">
              <div className="space-y-5 w-full">
                <ConfigSectionMediux
                  value={newConfig.mediux}
                  editing={editing}
                  configAlreadyLoaded={true}
                  dirtyFields={dirty.mediux}
                  onChange={(field, value) => updateConfigField("mediux", field, value)}
                  errorsUpdate={(errs) => updateSectionErrors("mediux", errs as Record<string, string>)}
                />
                <ConfigSectionMediaServer
                  value={newConfig.media_server}
                  editing={editing}
                  configAlreadyLoaded={true}
                  dirtyFields={dirty.media_server}
                  onChange={(field, value) => updateConfigField("media_server", field, value)}
                  errorsUpdate={(errs) => updateSectionErrors("media_server", errs as Record<string, string>)}
                />
                <ConfigSectionAuth
                  value={newConfig.auth}
                  editing={editing}
                  dirtyFields={dirty.auth}
                  onChange={(field, value) => updateConfigField("auth", field, value)}
                  errorsUpdate={(errs) => updateSectionErrors("auth", errs as Record<string, string>)}
                />
                <ConfigSectionLogging
                  value={newConfig.logging}
                  editing={editing}
                  dirtyFields={dirty.logging}
                  onChange={(field, value) => updateConfigField("logging", field, value)}
                  errorsUpdate={(errs) => updateSectionErrors("logging", errs as Record<string, string>)}
                />
                <ConfigSectionImages
                  value={newConfig.images}
                  editing={editing}
                  dirtyFields={
                    dirty.images
                      ? {
                          ...dirty.images,
                          save_images_locally: dirty.images.save_images_locally
                            ? {
                                ...dirty.images.save_images_locally,
                                path:
                                  typeof dirty.images.save_images_locally.path === "string"
                                    ? !!dirty.images.save_images_locally.path
                                    : dirty.images.save_images_locally.path,
                              }
                            : undefined,
                          kometa: dirty.images.kometa
                            ? {
                                ...dirty.images.kometa,
                                asset_directory:
                                  typeof dirty.images.kometa.asset_directory === "string"
                                    ? !!dirty.images.kometa.asset_directory
                                    : dirty.images.kometa.asset_directory,
                                import_cron:
                                  typeof dirty.images.kometa.import_cron === "string"
                                    ? !!dirty.images.kometa.import_cron
                                    : dirty.images.kometa.import_cron,
                              }
                            : undefined,
                        }
                      : undefined
                  }
                  onChange={updateImagesField}
                  errorsUpdate={(errs) => updateSectionErrors("images", errs as Record<string, string>)}
                  mediaServerType={newConfig.media_server.type}
                  libraries={newConfig.media_server.libraries || []}
                />
                <ConfigSectionAutoDownload
                  value={newConfig.auto_download}
                  editing={editing}
                  dirtyFields={dirty.auto_download}
                  onChange={(f, v) => updateConfigField("auto_download", f, v)}
                  errorsUpdate={(errs) => updateSectionErrors("auto_download", errs as Record<string, string>)}
                />

                {/* <ConfigSectionTMDB
									value={newConfig.TMDB}
									editing={editing}
									dirtyFields={dirty.TMDB}
									onChange={(f, v) => updateConfigField("TMDB", f, v)}
									errorsUpdate={(errs) => updateSectionErrors("TMDB", errs as Record<string, string>)}
								/> */}

                <ConfigSectionSonarrRadarr
                  value={newConfig.sonarr_radarr}
                  editing={editing}
                  dirtyFields={
                    dirty.sonarr_radarr
                      ? {
                          applications: dirty.sonarr_radarr as Partial<{
                            type: boolean;
                            library: boolean;
                            url: boolean;
                            api_token: boolean;
                          }>[],
                        }
                      : undefined
                  }
                  onChange={(field, val) => updateConfigField("sonarr_radarr", field, val)}
                  errorsUpdate={(errs) => updateSectionErrors("sonarr_radarr", errs as Record<string, string>)}
                  configAlreadyLoaded={true}
                  libraries={newConfig.media_server.libraries || []}
                />

                {(newConfig.media_server.type === "Plex" ||
                  (Array.isArray(newConfig.sonarr_radarr.applications) &&
                    newConfig.sonarr_radarr.applications.length > 0)) && (
                  <ConfigSectionLabelsAndTags
                    value={newConfig.labels_and_tags}
                    editing={editing}
                    dirtyFields={
                      dirty.labels_and_tags as {
                        applications?: Array<
                          Partial<Record<string, boolean | { enabled?: boolean; add?: boolean; remove?: boolean }>>
                        >;
                      }
                    }
                    mediaServerType={newConfig.media_server.type}
                    srOptions={Array.from(
                      new Set(
                        (newConfig.sonarr_radarr.applications ?? []).map((app) => app.type).filter((type) => !!type)
                      )
                    )}
                    onChange={(field, val) => updateConfigField("labels_and_tags", field, val)}
                    errorsUpdate={(errs) => updateSectionErrors("labels_and_tags", errs as Record<string, string>)}
                  />
                )}

                <ConfigSectionNotifications
                  value={newConfig.notifications}
                  editing={editing}
                  dirtyFields={
                    dirty.notifications as {
                      enabled?: boolean;
                      providers?: Partial<
                        Record<
                          string,
                          | boolean
                          | {
                              enabled?: boolean;
                              webhook?: boolean;
                              user_key?: boolean;
                              api_token?: boolean;
                              url?: boolean;
                              headers?: Record<string, boolean>;
                            }
                        >
                      >[];
                    }
                  }
                  onChange={(field, val) => updateConfigField("notifications", field, val)}
                  errorsUpdate={(errs) => updateSectionErrors("notifications", errs as Record<string, string>)}
                  configAlreadyLoaded={true}
                  templateVariablesCatalog={notificationTemplateVariables}
                />
              </div>
            </TabsContent>

            <TabsContent value="user-preferences" className="mt-6 w-full">
              <div id="preferences-section" ref={preferencesRef} className="w-full">
                <UserPreferencesCard />
              </div>
            </TabsContent>
          </Tabs>

          {activeTab === "app-settings" && editing && hasValidationErrors && (
            <p className="mb-2 text-red-500">Fix validation errors before saving.</p>
          )}

          {activeTab === "app-settings" && editing && (
            <div className="sticky bottom-0 mt-10 z-30">
              <div
                className={`mx-auto w-fit bg-background/90 backdrop-blur border rounded-md shadow px-4 py-3 flex items-center gap-3 ${anyDirty && "border-amber-500"}`}
              >
                <span className="text-sm">{anyDirty ? "Unsaved changes" : "No changes yet"}</span>
              </div>
            </div>
          )}
        </>
      )}

      {/* Debug Mode Toggle & Cache Clear */}
      <div className="flex items-center justify-between mt-6 border-t pt-4">
        <ToggleGroup
          type="single"
          variant={debugEnabled ? "default" : "outline"}
          value={debugEnabled ? "enabled" : "disabled"}
          onValueChange={(value) => toggleDebugMode(value === "enabled")}
        >
          <ToggleGroupItem value="enabled" variant={debugEnabled ? "default" : "outline"}>
            <span className="flex items-center gap-2 cursor-pointer">
              Debug Mode:
              {debugEnabled ? (
                <span className="text-green-500">Enabled</span>
              ) : (
                <span className="text-destructive hover:text-red-500">Disabled</span>
              )}
            </span>
          </ToggleGroupItem>
        </ToggleGroup>

        <ConfirmDestructiveDialogActionButton
          onConfirm={async () => {
            localStorage.clear();
            await ClearAllStores();
            toast.success("App Cache Cleared. Reloading...", { duration: 750 });
            setTimeout(() => {
              router.replace("/settings");
            }, 1000);
          }}
          title="Clear App Cache?"
          description="This will clear all local storage and IndexedDB data. Are you sure you want to continue?"
          confirmText="Yes, Clear Cache"
          cancelText="Cancel"
          variant="ghost"
          className="text-destructive border-1 shadow-none hover:text-red-500 cursor-pointer"
        >
          Clear App Cache
        </ConfirmDestructiveDialogActionButton>
      </div>

      {activeTab !== "user-preferences" && (
        <div className="fixed z-100 right-3 bottom-5 sm:bottom-15 flex flex-col items-end gap-2">
          {editing && anyDirty && (
            <Button
              variant="default"
              size="sm"
              className={cn(
                `mb-1 rounded-full shadow-lg transition-all duration-300 border-1 border-green-500 bg-green-500/50
          hover:text-black hover:!bg-green-500 cursor-pointer`
              )}
              onClick={handleSaveAll}
              disabled={!anyDirty || saving || hasValidationErrors}
              aria-label="save"
            >
              <SaveIcon className="h-3 w-3" />
              <span className="text-xs hidden sm:inline">{saving ? "Saving..." : "Save"}</span>
            </Button>
          )}
          {!editing ? (
            <Button
              variant="ghost"
              size="sm"
              className={cn(
                `rounded-full shadow-lg transition-all duration-300 border-1 border-primary bg-primary/50 
          hover:text-black hover:!bg-primary cursor-pointer`
              )}
              onClick={() => setEditing(true)}
              aria-label="edit"
            >
              <Edit className="h-3 w-3" />
              <span className="text-xs hidden sm:inline">Edit</span>
            </Button>
          ) : (
            <Button
              variant="ghost"
              size="sm"
              className={cn(
                `rounded-full shadow-lg transition-all duration-300 border-1 border-destructive bg-destructive/50
          hover:text-white hover:!bg-destructive cursor-pointer`
              )}
              onClick={handleCancel}
              aria-label="cancel"
            >
              <X className="h-3 w-3" />
              <span className="text-xs hidden sm:inline">Cancel</span>
            </Button>
          )}
        </div>
      )}
    </div>
  );
};

export default SettingsPage;
