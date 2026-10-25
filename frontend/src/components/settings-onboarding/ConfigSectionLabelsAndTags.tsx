"use client";

import { Plus, Trash2 } from "lucide-react";

import React, { useEffect, useRef, useState } from "react";

import { PopoverHelp } from "@/components/shared/popover-help";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";

import { cn } from "@/lib/cn";

import type { AppConfigLabelsAndTags, AppConfigLabelsAndTagsApplication } from "@/types/config/config";

interface ConfigSectionLabelsAndTagsProps {
  value: AppConfigLabelsAndTags;
  editing: boolean;
  dirtyFields?: {
    applications?: Array<
      Partial<
        Record<
          string,
          | boolean
          | {
              enabled?: boolean;
              add?: boolean;
              remove?: boolean;
              add_label_tag_for_selected_types?: boolean;
            }
        >
      >
    >;
    remove_overlay_label_only_on_poster_download?: boolean;
  };
  onChange: <K extends keyof AppConfigLabelsAndTags>(field: K, value: AppConfigLabelsAndTags[K]) => void;
  errorsUpdate?: (errors: Record<string, string>) => void;
  mediaServerType?: string;
  srOptions?: string[];
}

export const ConfigSectionLabelsAndTags: React.FC<ConfigSectionLabelsAndTagsProps> = ({
  value,
  editing,
  dirtyFields = {},
  onChange,
  errorsUpdate,
  mediaServerType,
  srOptions,
}) => {
  const prevErrorsRef = useRef<string>("");

  const APPLICATION_TYPES = React.useMemo(() => {
    let appTypes = [];
    if (mediaServerType === "Plex") {
      appTypes.push("Plex");
    }
    if (Array.isArray(srOptions) && srOptions.length > 0) {
      appTypes = appTypes.concat(srOptions);
    }
    return appTypes;
  }, [mediaServerType, srOptions]);

  const [newApplicationType, setNewApplicationType] = useState(APPLICATION_TYPES[0] || "");

  const applications = React.useMemo(
    () => (Array.isArray(value.applications) ? value.applications : []),
    [value.applications]
  );

  // ----- Validation -----
  const errors = React.useMemo(() => {
    const errs: Record<string, string> = {};

    applications.forEach((app, idx) => {
      const addLabels = Array.isArray(app.add) ? app.add : [];
      const removeLabels = Array.isArray(app.remove) ? app.remove : [];
      const orLabelTag = app.application === "Plex" ? "Label" : "Tag";
      if (app.enabled && addLabels.length === 0 && removeLabels.length === 0) {
        errs[`Applications.[${idx}]`] = `At least one ${orLabelTag.toLowerCase()} to add or remove must be specified.`;
      }
      const intersection = addLabels.filter((label) => removeLabels.includes(label));
      if (intersection.length > 0) {
        errs[`Applications.[${idx}]`] = `${orLabelTag} cannot be in both Add and Remove: ${intersection.join(", ")}`;
      }
    });

    return errs;
  }, [applications]);

  useEffect(() => {
    if (!errorsUpdate) return;
    const serialized = JSON.stringify(errors);
    if (serialized === prevErrorsRef.current) return;
    prevErrorsRef.current = serialized;
    errorsUpdate(errors);
  }, [errors, errorsUpdate]);

  // ----- Mutators -----
  const setApplications = (apps: AppConfigLabelsAndTagsApplication[]) => onChange("applications", apps);
  const providerExists = applications.some((app) => app.application === newApplicationType);

  const addApplication = () => {
    if (!editing || providerExists) return;
    const type = newApplicationType;
    let newEntry: AppConfigLabelsAndTagsApplication;
    if (type === "Plex") {
      newEntry = {
        application: "Plex",
        enabled: true,
        add: [],
        remove: [],
        add_label_tag_for_selected_types: false,
      };
    } else if (Array.isArray(srOptions) && srOptions.includes(type)) {
      newEntry = {
        application: type,
        enabled: true,
        add: [],
        remove: [],
        add_label_tag_for_selected_types: false,
      };
    }
    setApplications([...applications, newEntry!]);
  };

  useEffect(() => {
    const usedTypes = applications.map((app) => app.application);
    const nextAvailable = APPLICATION_TYPES.find((type) => !usedTypes.includes(type)) || "";
    setNewApplicationType(nextAvailable);
  }, [APPLICATION_TYPES, applications]);

  const removeApplication = (index: number) => {
    if (!editing) return;
    const apps = [...applications];
    apps.splice(index, 1);
    setApplications(apps);
  };

  const updateEnabled = (index: number, enabled: boolean) => {
    const next = applications.slice();
    next[index] = { ...next[index], enabled: enabled };
    setApplications(next);
  };

  const updateApplication = <K extends keyof AppConfigLabelsAndTagsApplication>(
    index: number,
    field: K,
    val: AppConfigLabelsAndTagsApplication[K]
  ) => {
    const next = applications.slice();
    next[index] = { ...next[index], [field]: val };
    setApplications(next);
  };

  return (
    <Card className={`gap-4 p-4 sm:p-5 ${Object.values(errors).some(Boolean) ? "border-red-500" : "border-muted"}`}>
      <div className="flex items-center justify-between">
        <h2 className="text-xl font-semibold text-blue-500">Labels & Tags</h2>
      </div>
      <div className={cn("space-y-4", "rounded-md")}>
        <div className="flex items-center justify-between">
          {editing &&
            (() => {
              const availableTypes = APPLICATION_TYPES.filter(
                (p) => !applications.some((app) => app.application === p)
              );
              return availableTypes.length > 0 ? (
                <div className="flex items-center gap-2">
                  <Select value={newApplicationType} onValueChange={(v) => setNewApplicationType(v)}>
                    <SelectTrigger className="h-8 w-36">
                      <SelectValue placeholder="Type" />
                    </SelectTrigger>
                    <SelectContent>
                      {availableTypes.map((p) => (
                        <SelectItem key={p} value={p}>
                          {p}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                  <Button type="button" variant="outline" size="sm" onClick={addApplication} disabled={providerExists}>
                    <Plus className="h-4 w-4 mr-1" />
                    Add
                  </Button>
                </div>
              ) : null;
            })()}
        </div>

        {!mediaServerType && (!srOptions || srOptions.length === 0) ? (
          <p className="text-sm text-red-500 mb-2">
            You need to have a Media Server configured and/or at least one Sonarr/Radarr instance before you can use
            Labels &amp; Tags.
          </p>
        ) : applications.length === 0 ? (
          <p className="text-xs text-muted-foreground">No applications configured.</p>
        ) : null}

        {applications.map((app, idx) => {
          const appDirty = dirtyFields.applications?.[idx] as Partial<{
            enabled?: boolean;
            add?: boolean;
            remove?: boolean;
          }>;
          const appError = errors[`applications.[${idx}]`];
          const addLabels = Array.isArray(app.add) ? app.add : [];
          const removeLabels = Array.isArray(app.remove) ? app.remove : [];
          const orLabelTag = app.application === "Plex" ? "Labels" : "Tags";
          return (
            <div
              key={idx}
              className={cn(
                "space-y-3 rounded-md border p-3 transition",
                appError ? "border-red-500" : appDirty ? "border-amber-500" : "border-muted"
              )}
            >
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-3">
                  <p className="font-medium text-lg">{app.application}</p>
                  <Switch disabled={!editing} checked={app.enabled} onCheckedChange={(v) => updateEnabled(idx, v)} />
                </div>
                {editing && (
                  <Button
                    variant="ghost"
                    size="icon"
                    onClick={() => removeApplication(idx)}
                    aria-label="remove-application"
                    //disabled={applications.length === 1}
                    className="bg-red-700"
                  >
                    <Trash2 className="h-4 w-4" />
                  </Button>
                )}
              </div>

              {/* Add Labels/Tags */}
              <div className="space-y-2">
                <div className="flex items-center justify-between">
                  <Label>Add {orLabelTag}</Label>
                  {editing && (
                    <PopoverHelp ariaLabel="help-labels-to-add">
                      <p>{orLabelTag} to add to items after processing.</p>
                    </PopoverHelp>
                  )}
                </div>
                <div className="flex flex-wrap items-center gap-2">
                  {addLabels.length === 0 && (
                    <span className="inline-flex h-7 items-center rounded-full border border-dashed px-3 text-xs text-muted-foreground">
                      No {orLabelTag.toLowerCase()} to add
                    </span>
                  )}
                  {addLabels.map((label, i) => (
                    <span
                      key={i}
                      className={cn(
                        "inline-flex items-center rounded-full px-3 py-1 text-xs font-normal cursor-pointer select-none transition duration-200",
                        "bg-green-600 text-white",
                        editing && "hover:bg-red-500 hover:text-white"
                      )}
                      onClick={() =>
                        editing &&
                        updateApplication(
                          idx,
                          "add",
                          addLabels.filter((_, j) => j !== i)
                        )
                      }
                      title={editing ? `Remove ${orLabelTag}` : label}
                    >
                      {label}
                    </span>
                  ))}
                  {editing && (
                    <form
                      onSubmit={(e) => {
                        e.preventDefault();
                        const input = e.currentTarget.elements.namedItem("addLabel") as HTMLInputElement;
                        let val = input.value.trim();
                        if (app.application && ["Sonarr", "Radarr"].includes(app.application)) {
                          // Lowercase tags for Sonarr/Radarr
                          val = val.toLowerCase();
                        }
                        if (val && !addLabels.includes(val)) {
                          updateApplication(idx, "add", [...addLabels, val]);
                        }
                        input.value = "";
                      }}
                      className="flex items-center gap-1"
                    >
                      <Input name="addLabel" placeholder={`Add ${orLabelTag}...`} className="h-7 w-40 text-xs" />
                      <Button
                        type="submit"
                        variant="outline"
                        size="icon"
                        className="h-7 w-7"
                        title={`Add ${orLabelTag}`}
                      >
                        <Plus className="h-4 w-4" />
                      </Button>
                    </form>
                  )}
                </div>
              </div>

              {/* Add/Remove for Selected Types */}
              <div className="space-y-2">
                <Label className="mb-2">Add {orLabelTag.toLowerCase()} for selected image types</Label>
                <div className="flex w-full items-center gap-2">
                  <Switch
                    disabled={!editing}
                    checked={app.add_label_tag_for_selected_types || false}
                    onCheckedChange={(v) => updateApplication(idx, "add_label_tag_for_selected_types", v)}
                  />
                  {editing && (
                    <div className="ml-auto flex items-center">
                      <PopoverHelp ariaLabel="help-labels-tags-selected-types">
                        <div className="space-y-2">
                          <p>
                            When enabled, Aura will add/remove a separate {orLabelTag.toLowerCase()} for each selected
                            image type.
                          </p>
                          <ul className="list-disc pl-5 space-y-1">
                            <li>Helps you quickly identify which image types Aura downloaded.</li>
                            <li>
                              Uses the format <span className="font-medium">aura-&lt;type&gt;</span>.
                            </li>
                          </ul>
                          <p className="text-muted-foreground">
                            Example: selecting <span className="font-medium">'Poster'</span> and{" "}
                            <span className="font-medium">'Backdrop'</span> adds the {orLabelTag.toLowerCase()}{" "}
                            <span className="font-medium">'aura-poster'</span> and{" "}
                            <span className="font-medium">'aura-backdrop'</span>.
                          </p>
                        </div>
                      </PopoverHelp>
                    </div>
                  )}
                </div>
              </div>

              {/* Remove Labels */}
              <div className="space-y-2">
                <div className="flex items-center justify-between">
                  <Label>Remove {orLabelTag}</Label>
                  {editing && (
                    <PopoverHelp ariaLabel="help-labels-tags-remove">
                      <p>{orLabelTag} to remove from items after processing.</p>
                    </PopoverHelp>
                  )}
                </div>
                <div className="flex flex-wrap items-center gap-2">
                  {removeLabels.length === 0 && (
                    <span className="inline-flex h-7 items-center rounded-full border border-dashed px-3 text-xs text-muted-foreground">
                      No {orLabelTag.toLowerCase()} to remove
                    </span>
                  )}
                  {removeLabels.map((label, i) => (
                    <span
                      key={i}
                      className={cn(
                        "inline-flex items-center rounded-full px-3 py-1 text-xs font-normal cursor-pointer select-none transition duration-200",
                        "bg-red-600 text-white",
                        editing && "hover:bg-red-800 hover:text-white"
                      )}
                      onClick={() =>
                        editing &&
                        updateApplication(
                          idx,
                          "remove",
                          removeLabels.filter((_, j) => j !== i)
                        )
                      }
                      title={editing ? `Remove ${orLabelTag}` : label}
                    >
                      {label}
                    </span>
                  ))}
                  {editing && (
                    <form
                      onSubmit={(e) => {
                        e.preventDefault();
                        const input = e.currentTarget.elements.namedItem("removeLabel") as HTMLInputElement;
                        let val = input.value.trim();
                        if (app.application && ["Sonarr", "Radarr"].includes(app.application)) {
                          // Lowercase tags for Sonarr/Radarr
                          val = val.toLowerCase();
                        }
                        if (val && !removeLabels.includes(val)) {
                          updateApplication(idx, "remove", [...removeLabels, val]);
                        }
                        input.value = "";
                      }}
                      className="flex items-center gap-1"
                    >
                      <Input name="removeLabel" placeholder={`Remove ${orLabelTag}...`} className="h-7 w-40 text-xs" />
                      <Button
                        type="submit"
                        variant="outline"
                        size="icon"
                        className="h-7 w-7"
                        title={`Remove ${orLabelTag}`}
                      >
                        <Plus className="h-4 w-4" />
                      </Button>
                    </form>
                  )}
                </div>
              </div>

              {/* If the application is Plex and you have "Overlay" in your Remove list, show a toggle to remove the "Overlay" label only on poster download, which allows Kometa to reprocess the image and apply the overlays. This is a Plex exclusive feature. */}
              {app.application === "Plex" && removeLabels.includes("Overlay") && (
                <div className="space-y-2">
                  <Label className="mb-2">Remove "Overlay" label only on poster download</Label>
                  <div className="flex w-full items-center gap-2">
                    <Switch
                      disabled={!editing}
                      checked={value.remove_overlay_label_only_on_poster_download || false}
                      onCheckedChange={(v) => onChange("remove_overlay_label_only_on_poster_download", v)}
                    />
                    {editing && (
                      <div className="ml-auto flex items-center">
                        <PopoverHelp ariaLabel="help-remove-overlay-only-on-poster-download">
                          <div className="space-y-2">
                            <p>
                              When enabled, Aura will only remove the "Overlay" label from media items if it downloaded
                              a poster image. This allows Kometa to reprocess the image and apply the overlays.
                            </p>
                            <p className="text-red-500 font-medium">
                              Important: This is a Plex exclusive feature and should be set to true if you have
                              "Overlay" in your "Remove" list for the Plex application under "Labels &amp; Tags".
                            </p>
                          </div>
                        </PopoverHelp>
                      </div>
                    )}
                  </div>
                </div>
              )}

              {appError && <p className="text-xs text-red-500">{appError}</p>}
            </div>
          );
        })}
      </div>
    </Card>
  );
};
