"use client";

import { makePlural } from "@/helper/make_plural";
import { finalizeOnboarding } from "@/services/config/onboarding-finalize";
import { GetNotificationTemplateVariables } from "@/services/config/template-variables";
import { UpdateAppConfig } from "@/services/config/update";
import * as yaml from "js-yaml";
import { toast } from "sonner";

import type { JSX } from "react";
import { useCallback, useEffect, useMemo, useState } from "react";

import Image from "next/image";

import { ConfigSectionAuth } from "@/components/settings-onboarding/ConfigSectionAuth";
import { ConfigSectionAutoDownload } from "@/components/settings-onboarding/ConfigSectionAutoDownload";
import { ConfigSectionImages } from "@/components/settings-onboarding/ConfigSectionImages";
import { ConfigSectionLabelsAndTags } from "@/components/settings-onboarding/ConfigSectionLabelsAndTags";
import { ConfigSectionLogging } from "@/components/settings-onboarding/ConfigSectionLogging";
import { ConfigSectionMediaServer } from "@/components/settings-onboarding/ConfigSectionMediaServer";
import { ConfigSectionMediux } from "@/components/settings-onboarding/ConfigSectionMediux";
import { ConfigSectionNotifications } from "@/components/settings-onboarding/ConfigSectionNotifications";
import { ConfigSectionSonarrRadarr } from "@/components/settings-onboarding/ConfigSectionSonarrRadarr";
import { Button } from "@/components/ui/button";
import { H2, P } from "@/components/ui/typography";

import { useOnboardingStore } from "@/lib/stores/global-store-onboarding";

import type { AppConfig, NotificationTemplateVariablesCatalog } from "@/types/config/config";
import { defaultAppConfig } from "@/types/config/config-default-app";

interface StepDef {
  key: string;
  title: string;
  optional?: boolean;
  render: () => JSX.Element;
}

const OnboardingPage = () => {
  const { status, fetchStatus } = useOnboardingStore();

  // Hydrate/fetch onboarding status on mount
  useEffect(() => {
    if (!status) fetchStatus();
  }, [status, fetchStatus]);

  const [applyLoading, setApplyLoading] = useState(false);
  const [configState, setConfigState] = useState<AppConfig>(() => status?.current_setup || defaultAppConfig());
  const [notificationTemplateVariables, setNotificationTemplateVariables] =
    useState<NotificationTemplateVariablesCatalog | null>(null);
  const [validationErrors, setValidationErrors] = useState<Record<string, Record<string, string>>>({});
  const [errorSummaryOpen, setErrorSummaryOpen] = useState(false);

  useEffect(() => {
    let mounted = true;

    const fetchTemplateVariables = async () => {
      const response = await GetNotificationTemplateVariables();
      if (!mounted) return;
      if (response.status === "success" && response.data?.variables) {
        setNotificationTemplateVariables(response.data.variables);
      }
    };

    fetchTemplateVariables();

    return () => {
      mounted = false;
    };
  }, []);

  // Keep configState in sync with backend status if it changes
  useEffect(() => {
    if (status?.current_setup) {
      setConfigState(status.current_setup);
    }
  }, [status?.current_setup]);

  const updateSectionErrors = useCallback((section: string, errs?: Record<string, string>) => {
    setValidationErrors((prev) => {
      if (!errs || Object.keys(errs).length === 0) {
        const { [section]: _, ...rest } = prev;
        return rest;
      }
      return { ...prev, [section]: errs };
    });
  }, []);

  const updateSectionField = useCallback(
    <S extends keyof AppConfig, K extends keyof AppConfig[S]>(section: S, field: K, value: AppConfig[S][K]) => {
      setConfigState(
        (prev) =>
          ({
            ...prev,
            [section]: { ...(prev[section] as object), [field]: value },
          }) as AppConfig
      );
    },
    []
  );

  const updateImagesField = useCallback(
    <G extends keyof AppConfig["images"], F extends keyof AppConfig["images"][G]>(
      group: G,
      field: F,
      value: AppConfig["images"][G][F]
    ) => {
      setConfigState((prev) => ({
        ...prev,
        images: {
          ...prev.images,
          [group]: {
            ...prev.images[group],
            [field]: value,
          },
        },
      }));
    },
    []
  );

  const stripEmpty = useCallback(function stripEmpty(obj: unknown): unknown {
    if (Array.isArray(obj)) {
      return obj
        .map(stripEmpty)
        .filter(
          (v) =>
            v !== undefined &&
            v !== null &&
            !(typeof v === "string" && v === "") &&
            !(Array.isArray(v) && v.length === 0) &&
            !(typeof v === "object" && v && Object.keys(v).length === 0)
        );
    }
    if (typeof obj === "object" && obj !== null) {
      const out: Record<string, unknown> = {};
      for (const [k, v] of Object.entries(obj)) {
        const cleaned = stripEmpty(v);
        if (
          cleaned !== undefined &&
          cleaned !== null &&
          !(typeof cleaned === "string" && cleaned === "") &&
          !(Array.isArray(cleaned) && cleaned.length === 0) &&
          !(typeof cleaned === "object" && cleaned && Object.keys(cleaned).length === 0)
        ) {
          out[k] = cleaned;
        }
      }
      return out;
    }
    return obj;
  }, []);

  // Memoized YAML representation
  const reviewYaml = useMemo(() => {
    try {
      // Clone and strip empty values
      const plain = stripEmpty(JSON.parse(JSON.stringify(configState)));
      return yaml.dump(plain, {
        noRefs: true,
        lineWidth: 100,
        skipInvalid: true,
      });
    } catch {
      return "# Failed to serialize configuration to YAML";
    }
  }, [configState, stripEmpty]);

  const steps: StepDef[] = useMemo(
    () => [
      {
        key: "welcome",
        title: "Welcome",
        render: () => (
          <div className="space-y-6">
            <div className="flex items-center gap-4">
              <H2 className="text-4xl font-bold">Welcome to</H2>
              <Image src="/aura_word_logo.svg" alt="Aura Logo" width={120} height={120} />
            </div>

            <P className="text-muted-foreground max-w-xl">
              {status?.config_loaded && (
                <span className="text-destructive">Your configuration file might have some errors. </span>
              )}
              This quick setup will guide you through core configuration. Use Next / Back to move through the steps.
            </P>
          </div>
        ),
      },
      {
        key: "mediux",
        title: "Mediux",
        render: () => (
          <ConfigSectionMediux
            value={configState.mediux}
            editing
            configAlreadyLoaded={status?.config_loaded || false}
            dirtyFields={{}}
            onChange={(f, v) => updateSectionField("mediux", f, v)}
            errorsUpdate={(errs) => updateSectionErrors("mediux", errs as Record<string, string>)}
          />
        ),
      },
      {
        key: "mediaserver",
        title: "Media Server",
        render: () => (
          <ConfigSectionMediaServer
            value={configState.media_server}
            editing
            configAlreadyLoaded={status?.config_loaded || false}
            dirtyFields={{}}
            onChange={(f, v) => updateSectionField("media_server", f, v)}
            errorsUpdate={(errs) => updateSectionErrors("media_server", errs as Record<string, string>)}
          />
        ),
      },
      {
        key: "auth",
        title: "Auth",
        optional: true,
        render: () => (
          <ConfigSectionAuth
            value={configState.auth}
            editing
            dirtyFields={{}}
            onChange={(f, v) => updateSectionField("auth", f, v)}
            errorsUpdate={(errs) => updateSectionErrors("auth", errs as Record<string, string>)}
          />
        ),
      },
      {
        key: "logging",
        title: "Logging",
        render: () => (
          <ConfigSectionLogging
            value={configState.logging}
            editing
            dirtyFields={{}}
            onChange={(f, v) => updateSectionField("logging", f, v)}
            errorsUpdate={(errs) => updateSectionErrors("logging", errs as Record<string, string>)}
          />
        ),
      },
      {
        key: "images",
        title: "Images",
        optional: true,
        render: () => (
          <ConfigSectionImages
            value={configState.images}
            editing
            dirtyFields={{}}
            onChange={updateImagesField}
            errorsUpdate={(errs) => updateSectionErrors("images", errs as Record<string, string>)}
            mediaServerType={configState.media_server.type}
          />
        ),
      },
      {
        key: "autodownload",
        title: "Auto Download",
        optional: true,
        render: () => (
          <ConfigSectionAutoDownload
            value={configState.auto_download}
            editing
            dirtyFields={{}}
            onChange={(f, v) => updateSectionField("auto_download", f, v)}
            errorsUpdate={(errs) => updateSectionErrors("auto_download", errs as Record<string, string>)}
          />
        ),
      },
      {
        key: "sonarr_and_radarr",
        title: "Sonarr/Radarr",
        optional: true,
        render: () => (
          <ConfigSectionSonarrRadarr
            value={configState.sonarr_radarr}
            editing
            dirtyFields={{}}
            onChange={(f, v) => updateSectionField("sonarr_radarr", f, v)}
            errorsUpdate={(errs) => updateSectionErrors("sonarr_radarr", errs as Record<string, string>)}
            configAlreadyLoaded={status?.config_loaded || false}
            libraries={configState.media_server.libraries || []}
          />
        ),
      },
      {
        key: "labels_and_tags",
        title: "Labels & Tags",
        optional: true,
        render: () => (
          <ConfigSectionLabelsAndTags
            value={configState.labels_and_tags}
            editing
            dirtyFields={{}}
            onChange={(f, v) => updateSectionField("labels_and_tags", f, v)}
            errorsUpdate={(errs) => updateSectionErrors("labels_and_tags", errs as Record<string, string>)}
            mediaServerType={configState.media_server.type}
            srOptions={
              Array.from(
                new Set(
                  (Array.isArray(configState.sonarr_radarr?.applications) ? configState.sonarr_radarr.applications : [])
                    .map((app) => app.type)
                    .filter((type) => !!type)
                )
              ) || []
            }
          />
        ),
      },
      {
        key: "notifications",
        title: "Notifications",
        optional: true,
        render: () => (
          <ConfigSectionNotifications
            value={configState.notifications}
            editing
            dirtyFields={{}}
            onChange={(f, v) => updateSectionField("notifications", f, v)}
            errorsUpdate={(errs) => updateSectionErrors("notifications", errs as Record<string, string>)}
            configAlreadyLoaded={status?.config_loaded || false}
            templateVariablesCatalog={notificationTemplateVariables}
          />
        ),
      },
      {
        key: "review",
        title: "Review & Finish",
        render: () => (
          <div className="space-y-6">
            <h2 className="text-2xl font-semibold">Review</h2>
            <p className="text-sm text-muted-foreground">
              Press Finish to apply configuration and start the application.
            </p>
            <pre className="p-4 bg-muted rounded text-xs overflow-auto max-h-96">{reviewYaml}</pre>
            {Object.keys(validationErrors).length > 0 && (
              <div className="text-red-500 text-sm">Resolve validation errors before finishing.</div>
            )}
          </div>
        ),
      },
    ],
    [
      configState.auth,
      configState.auto_download,
      configState.images,
      configState.labels_and_tags,
      configState.logging,
      configState.media_server,
      configState.mediux,
      configState.notifications,
      configState.sonarr_radarr,
      notificationTemplateVariables,
      reviewYaml,
      status?.config_loaded,
      updateImagesField,
      updateSectionErrors,
      updateSectionField,
      validationErrors,
    ]
  );

  const [index, setIndex] = useState(0);
  const current = steps[index];
  const lastIndex = steps.length - 1;
  const hasErrors = Object.keys(validationErrors).length > 0;

  const next = () => setIndex((i) => Math.min(i + 1, lastIndex));
  const prev = () => setIndex((i) => Math.max(i - 1, 0));

  const finish = async () => {
    if (hasErrors) {
      toast.error("Fix validation errors first.");
      return;
    }
    setApplyLoading(true);

    try {
      const resp = await UpdateAppConfig(configState);
      if (resp.status === "success") {
        if (resp.data?.status) {
          const finalizeResp = await finalizeOnboarding(resp.data.status.current_setup);
          if (finalizeResp.status === "success") {
            toast.success("Configuration applied successfully, redirecting...");
            if (configState.auth.enabled) {
              localStorage.removeItem("aura-auth-token");
              setTimeout(() => (window.location.href = "/login"), 3000);
              return;
            }
            setTimeout(() => (window.location.href = "/"), 50);
          } else {
            toast.error("Failed to finalize onboarding.");
          }
        } else {
          toast.error("Failed to apply configuration.");
        }
      } else if (resp.status === "warn") {
        toast.warning(typeof resp.data === "string" ? resp.data : "Warning occurred while applying configuration.");
      } else {
        toast.error("Failed to apply configuration.");
      }
    } catch {
      toast.error("An error occurred while applying configuration.");
    }

    setApplyLoading(false);
  };

  // Build a lookup from (lowercased) step key to its index for quick jumps
  const stepIndexByKey = useMemo(() => {
    const m: Record<string, number> = {};
    steps.forEach((s, i) => {
      m[s.key.toLowerCase()] = i;
    });
    return m;
  }, [steps]);

  // Map config section names (e.g. MediaServer) to step keys (lowercase)
  const jumpToSection = (sectionName: string) => {
    const target = stepIndexByKey[sectionName.toLowerCase()];
    if (typeof target === "number") {
      setIndex(target);
      requestAnimationFrame(() => {
        window.scrollTo({ top: 0, behavior: "smooth" });
      });
    }
  };

  const ErrorSummary = ({ errors }: { errors: Record<string, Record<string, string>> }) => {
    const sections = Object.entries(errors);
    const total = sections.reduce((sum, [, errs]) => sum + Object.keys(errs).length, 0);
    if (total === 0) return null;

    return (
      <div className="w-full rounded-md border border-destructive/30 bg-destructive/5 p-3 mt-2">
        <div className="flex items-center justify-between gap-4">
          <P className="m-0 text-sm font-medium text-destructive">
            {total} {makePlural(total, "validation error")}
          </P>
          <Button
            type="button"
            variant="ghost"
            size="sm"
            onClick={() => setErrorSummaryOpen((o) => !o)}
            className="h-6 px-2"
          >
            {errorSummaryOpen ? "Hide" : "Show"}
          </Button>
        </div>

        {errorSummaryOpen && (
          <div className="mt-3 grid gap-2 sm:grid-cols-2">
            {sections.map(([section, errs]) => {
              const count = Object.keys(errs).length;
              return (
                <div
                  key={section}
                  role="button"
                  tabIndex={0}
                  onClick={() => jumpToSection(section)}
                  onKeyDown={(e) => {
                    if (e.key === "Enter" || e.key === " ") {
                      e.preventDefault();
                      jumpToSection(section);
                    }
                  }}
                  className="group cursor-pointer rounded border border-destructive/30 bg-destructive/10 p-2 transition hover:border-destructive/60 hover:bg-destructive/15 focus:outline-none focus:ring-2 focus:ring-destructive/60"
                >
                  <div className="flex items-center justify-between">
                    <p className="m-0 font-semibold text-sm text-destructive group-hover:underline">
                      {section.replace(/([A-Z])/g, " $1")}
                    </p>
                    <span className="rounded bg-destructive/20 px-2 py-0.5 text-sm text-destructive">
                      {count} {makePlural(count, "error")}
                    </span>
                  </div>
                  <ul className="mt-1 space-y-0.5">
                    {Object.entries(errs).map(([field, msg]) => (
                      <li key={field} className="text-sm text-destructive">
                        <span className="font-mono">{field}</span>
                        <span className="mx-1 opacity-60"> - </span>
                        {msg}
                      </li>
                    ))}
                  </ul>
                </div>
              );
            })}
          </div>
        )}
      </div>
    );
  };

  return (
    <div className="mx-auto max-w-5xl p-6">
      <div className="flex flex-row flex-wrap items-center justify-between gap-3">
        {index > 0 ? <H2 className="text-4xl font-bold mb-2">Onboarding</H2> : <div />}
        <div className="flex flex-row flex-wrap gap-2 justify-end">
          {index > 0 && (
            <Button variant="outline" onClick={prev} disabled={applyLoading}>
              ← Back
            </Button>
          )}
          {index < lastIndex && (
            <Button onClick={next} disabled={applyLoading}>
              Next →
            </Button>
          )}
          {index === lastIndex && (
            <Button onClick={finish} disabled={applyLoading || hasErrors}>
              {applyLoading ? "Applying..." : "Apply & Save"}
            </Button>
          )}
        </div>
      </div>

      {index > 0 && (
        <div className="flex items-center gap-2">
          <P className="text-md text-muted-foreground">
            Step {index} of {steps.length}: {current.title}{" "}
          </P>
          {current.optional && <P className="text-md text-amber-600">(Optional)</P>}
        </div>
      )}

      {/* Config Section Item  */}
      <div>{current.render()}</div>

      {/* Error summary below full width */}
      {hasErrors && <ErrorSummary errors={validationErrors} />}
    </div>
  );
};

export default OnboardingPage;
