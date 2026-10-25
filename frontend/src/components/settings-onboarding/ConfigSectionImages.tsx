"use client";

import { ReturnErrorMessage } from "@/services/api-error-return";
import { DeleteTempImages } from "@/services/images/api-images-actions";
import { toast } from "sonner";

import React, { useEffect, useRef } from "react";

import { ConfigSectionKometa, kometaError } from "@/components/settings-onboarding/ConfigSectionKometa";
import { ConfirmDestructiveDialogActionButton } from "@/components/shared/dialog-destructive-action";
import { PopoverHelp } from "@/components/shared/popover-help";
import { VariablePicker, insertAtCursor } from "@/components/shared/variable-picker";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";

import { cn } from "@/lib/cn";

import type { AppConfigImageNameTemplates, AppConfigImages, AppConfigMediaServerLibrary } from "@/types/config/config";

// File names for each image type
const NAME_TEMPLATE_FIELDS: { key: keyof AppConfigImageNameTemplates; label: string }[] = [
  { key: "poster", label: "Poster" },
  { key: "backdrop", label: "Backdrop" },
  { key: "season_poster", label: "Season Poster" },
  { key: "special_season_poster", label: "Specials Season Poster" },
  { key: "titlecard", label: "Title Card" },
];

const EMPTY_NAME_TEMPLATES: AppConfigImageNameTemplates = {
  poster: "",
  backdrop: "",
  season_poster: "",
  special_season_poster: "",
  titlecard: "",
};

// nameTemplateProblem is why a template uses the standard name instead, or empty when it's fine
const nameTemplateProblem = (template: string, variables: string[]) => {
  const unknown = (template.match(/\{\{[^}]*\}\}/g) ?? []).filter((v) => !variables.includes(v));
  if (unknown.length > 0) return `Unknown ${unknown.length === 1 ? "variable" : "variables"}: ${unknown.join(", ")}`;
  if (template.split("/").includes("..")) return "Can't use .. in a file name";
  return "";
};

interface ConfigSectionImagesProps {
  value: AppConfigImages;
  editing: boolean;
  showActions?: boolean; // Show actions hidden while editing, as Settings is always editable
  dirtyFields?: {
    cache_images?: { enabled?: boolean };
    save_images_locally?: {
      enabled?: boolean;
      path?: boolean;
      running_on_windows?: boolean;
    };
    kometa?: Partial<Record<keyof AppConfigImages["kometa"], boolean>>; // Fork
  };
  onChange: <K extends keyof AppConfigImages, F extends keyof AppConfigImages[K]>(
    group: K,
    field: F,
    value: AppConfigImages[K][F]
  ) => void;
  errorsUpdate?: (errors: Partial<Record<keyof AppConfigImages, string>>) => void;
  mediaServerType?: string;
  imageNameOptions?: ImageNameOptions | null; // The file name variables and standard names, from aura
  libraries?: AppConfigMediaServerLibrary[]; // Fork: main media server libraries, for Kometa subfolders
}

export interface ImageNameOptions {
  variables: string[];
  defaults: AppConfigImageNameTemplates;
}

export const ConfigSectionImages: React.FC<ConfigSectionImagesProps> = ({
  value,
  editing,
  showActions = false,
  dirtyFields = {},
  onChange,
  errorsUpdate,
  mediaServerType,
  imageNameOptions,
  libraries = [],
}) => {
  const prevErrorsRef = useRef<string>("{}");
  const nameInputRefs = useRef<Partial<Record<keyof AppConfigImageNameTemplates, HTMLInputElement | null>>>({});

  // insertNameVariable puts the variable at the cursor of the file name, or at the end
  const insertNameVariable = (key: keyof AppConfigImageNameTemplates, variable: string) => {
    const templates = value.save_images_locally.name_templates ?? EMPTY_NAME_TEMPLATES;
    onChange("save_images_locally", "name_templates", {
      ...templates,
      [key]: insertAtCursor(nameInputRefs.current[key], templates[key] ?? "", variable),
    });
  };

  const clearTempImagesFolder = async () => {
    try {
      const response = await DeleteTempImages();
      if (response.status === "error") {
        toast.error(response.error?.message || "Failed to clear temp images folder");
        return;
      }
      toast.success(response.data?.message || "Temp images folder cleared successfully");
    } catch (error) {
      const errorResponse = ReturnErrorMessage<void>(error);
      toast.error(errorResponse.error?.message || "An unexpected error occurred");
    }
  };

  const errors = React.useMemo<Partial<Record<keyof AppConfigImages, string>>>(() => {
    const errs: Partial<Record<keyof AppConfigImages, string>> = {};
    // Fork: Kometa mode
    const kometaErr = kometaError(value.kometa, mediaServerType);
    if (kometaErr) errs.kometa = kometaErr;
    return errs;
  }, [value.kometa, mediaServerType]);

  // Emit errors upward
  useEffect(() => {
    if (!errorsUpdate) return;
    const serialized = JSON.stringify(errors);
    if (serialized === prevErrorsRef.current) return;
    prevErrorsRef.current = serialized;
    errorsUpdate(errors);
  }, [errors, errorsUpdate]);

  return (
    <Card className="gap-4 border-muted p-4 sm:p-5">
      <div className="flex items-center justify-between">
        <h2 className="text-xl font-semibold text-blue-500">Images</h2>
        <ConfirmDestructiveDialogActionButton
          hidden={editing && !showActions}
          onConfirm={clearTempImagesFolder}
          title="Clear Temp Images Folder?"
          description="This will permanently delete all temporary images. Are you sure you want to continue?"
          confirmText="Yes, Clear Images"
          cancelText="Cancel"
          className="text-destructive border-1 shadow-none hover:text-red-500 cursor-pointer"
          variant="ghost"
        >
          Clear Temp Images
        </ConfirmDestructiveDialogActionButton>
      </div>
      {/* Cache Images */}
      <div
        className={cn(
          "flex items-center justify-between border rounded-md p-3 transition",
          "border-muted",
          dirtyFields.cache_images?.enabled && "border-amber-500"
        )}
      >
        <Label className="mr-2">Cache Images</Label>
        <div className="flex items-center gap-2">
          <Switch
            disabled={!editing}
            checked={value.cache_images.enabled}
            onCheckedChange={(v) => onChange("cache_images", "enabled", v)}
          />
          {editing && (
            <PopoverHelp ariaLabel="help-images-cache">
              <p>Store downloaded artwork locally to reduce external requests and speed repeat access.</p>
            </PopoverHelp>
          )}
        </div>
      </div>

      {/* Save Images Locally */}
      <div
        className={cn(
          "border rounded-md p-3 transition",
          "border-muted",
          dirtyFields.save_images_locally?.enabled && "border-amber-500"
        )}
      >
        <div className="flex items-center justify-between mb-1">
          <Label className="mr-2">Save Images Locally</Label>
          <div className="flex items-center gap-2">
            <Switch
              disabled={!editing}
              checked={!!value.save_images_locally.enabled}
              onCheckedChange={(v) => onChange("save_images_locally", "enabled", v)}
            />
            {editing && (
              <PopoverHelp ariaLabel="help-images-save-next-to-content">
                <p>
                  Save images to a local folder on the server. This is useful for not relying on your Media Server
                  database. Make sure the path is accessible by the Aura server.
                </p>
              </PopoverHelp>
            )}
          </div>
        </div>

        {value.save_images_locally.enabled && (
          <div
            className={cn("mt-2", dirtyFields.save_images_locally?.enabled && "border border-amber-500 rounded-md p-2")}
          >
            <div className="flex items-center justify-between mb-1">
              <Label className="mr-2">Path</Label>
              {editing && (
                <PopoverHelp ariaLabel="help-images-save-path">
                  <p>
                    Enter the local folder path where images should be saved. This must be accessible by the Aura
                    server. Leave this blank if you want to save images next to the content.
                  </p>
                </PopoverHelp>
              )}
            </div>
            <Input
              type="text"
              disabled={!editing}
              value={value.save_images_locally.path || ""}
              onChange={(e) => onChange("save_images_locally", "path", e.target.value)}
              className={cn(
                "w-full px-3 py-2 border rounded-md text-xs sm:text-sm focus:outline-none focus:ring-2 focus:ring-primary disabled:opacity-50 transition",
                dirtyFields.save_images_locally?.path && "border-amber-500"
              )}
              placeholder="/path/to/images"
            />
          </div>
        )}

        {value.save_images_locally.enabled && (
          <div className="mt-4 space-y-3 border-t pt-4">
            <div>
              <Label>File Names</Label>
              <p className="text-xs sm:text-sm text-muted-foreground mt-1">
                The name of each saved image, the extension is added for you. The standard names work with every media
                server. Use / for folders, and start with / to place the file from the Path folder instead of next to
                the media. An empty name uses the standard name.
              </p>
            </div>
            {imageNameOptions && (
              <div className="flex flex-wrap gap-1">
                {imageNameOptions.variables.map((variable) => (
                  <span key={variable} className="rounded-sm bg-muted px-1.5 py-0.5 font-mono text-[11px] sm:text-xs">
                    {variable}
                  </span>
                ))}
              </div>
            )}
            {NAME_TEMPLATE_FIELDS.map((field) => {
              const templates = value.save_images_locally.name_templates ?? EMPTY_NAME_TEMPLATES;
              const template = templates[field.key] ?? "";
              const problem = imageNameOptions ? nameTemplateProblem(template, imageNameOptions.variables) : "";
              return (
                <div key={field.key} className="space-y-1">
                  <Label htmlFor={`name-template-${field.key}`} className="text-xs sm:text-sm">
                    {field.label}
                  </Label>
                  <div className="flex items-center gap-2">
                    <VariablePicker
                      variables={imageNameOptions?.variables ?? []}
                      disabled={!editing}
                      label={`the ${field.label} file name`}
                      onPick={(variable) => insertNameVariable(field.key, variable)}
                    />
                    <Input
                      id={`name-template-${field.key}`}
                      ref={(el) => {
                        nameInputRefs.current[field.key] = el;
                      }}
                      disabled={!editing}
                      value={template}
                      placeholder={imageNameOptions?.defaults[field.key]}
                      onChange={(e) =>
                        onChange("save_images_locally", "name_templates", { ...templates, [field.key]: e.target.value })
                      }
                      className="h-8 sm:h-9 font-mono text-xs sm:text-sm"
                    />
                  </div>
                  {problem && <p className="text-xs text-red-500">{problem}, the standard name is used.</p>}
                </div>
              );
            })}
          </div>
        )}

        {value.save_images_locally.enabled && (
          <div
            className={cn(
              "mt-4 flex items-center justify-between border-t pt-4",
              dirtyFields.save_images_locally?.running_on_windows && "border-amber-500"
            )}
          >
            <Label className="mr-2">Running on Windows</Label>
            <div className="flex items-center gap-2">
              <Switch
                disabled={!editing}
                checked={!!value.save_images_locally.running_on_windows}
                onCheckedChange={(v) => onChange("save_images_locally", "running_on_windows", v)}
              />
              {editing && (
                <PopoverHelp ariaLabel="help-images-running-on-windows">
                  <p className="mb-2">Turn on if your media server runs on Windows.</p>
                  <p className="text-muted-foreground">
                    Windows paths from the media server (e.g. <span className="font-mono">C:\Movies</span>) are
                    converted to paths aura can use (e.g. <span className="font-mono">/C/Movies</span>). Make sure those
                    paths are mounted in the aura container.
                  </p>
                </PopoverHelp>
              )}
            </div>
          </div>
        )}
      </div>
      {/* Fork: Kometa mode */}
      <ConfigSectionKometa
        kometa={value.kometa ?? { enabled: false }}
        editing={editing}
        showActions={showActions}
        mediaServerType={mediaServerType}
        libraries={libraries}
        dirtyFields={dirtyFields.kometa}
        onChange={(field, v) => onChange("kometa", field, v)}
      />
    </Card>
  );
};
