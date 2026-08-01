import React, { useEffect, useMemo, useRef } from "react";

import Link from "next/link";

import { PopoverHelp } from "@/components/shared/popover-help";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";

import { cn } from "@/lib/cn";

import type { AppConfigAuth, AppConfigOIDC } from "@/types/config/config";

interface ConfigSectionAuthProps {
  value: AppConfigAuth;
  editing: boolean;
  dirtyFields?: { enabled?: boolean; password?: boolean; oidc?: boolean };
  onChange: <K extends keyof AppConfigAuth>(field: K, value: AppConfigAuth[K]) => void;
  errorsUpdate?: (errors: Partial<Record<keyof AppConfigAuth, string>>) => void;
}

const hashRegex = /^\$argon2id\$v=\d+\$m=\d+,t=\d+,p=\d+\$[A-Za-z0-9+/=]+\$[A-Za-z0-9+/=]+$/;

/**
 * Mirrors the backend's masking of stored secrets. A masked value means "unchanged", so it
 * must not be validated as if the user had typed it.
 */
const maskedSecretRegex = /^\*{3}[^*]+$/;

const emptyOIDC: AppConfigOIDC = {
  enabled: false,
  issuer_url: "",
  client_id: "",
  client_secret: "",
  redirect_url: "",
  scopes: [],
  groups_claim: "",
  allowed_groups: [],
  allowed_emails: [],
  allowed_subjects: [],
  button_label: "",
  rp_initiated_logout: false,
};

/** Splits a comma or newline separated input into a trimmed list. */
const parseList = (raw: string): string[] =>
  raw
    .split(/[\n,]/)
    .map((entry) => entry.trim())
    .filter(Boolean);

const isAbsoluteURL = (raw: string): boolean => {
  try {
    const parsed = new URL(raw);
    return !!parsed.protocol && !!parsed.host;
  } catch {
    return false;
  }
};

export const ConfigSectionAuth: React.FC<ConfigSectionAuthProps> = ({
  value,
  editing,
  dirtyFields = {},
  onChange,
  errorsUpdate,
}) => {
  const prevErrorRef = useRef<string>("");
  const oidc = value.oidc ?? emptyOIDC;

  const updateOIDC = <K extends keyof AppConfigOIDC>(field: K, fieldValue: AppConfigOIDC[K]) => {
    onChange("oidc", { ...oidc, [field]: fieldValue });
  };

  // Validation
  const errors = useMemo<Partial<Record<keyof AppConfigAuth, string>>>(() => {
    const errs: Partial<Record<keyof AppConfigAuth, string>> = {};
    if (!value.enabled) return errs;

    // Password Errors. A password is optional once OIDC can sign users in, but leaving
    // both off would make the app unreachable.
    const password = value.password.trim();
    if (password.length === 0) {
      if (!oidc.enabled) {
        errs.password = "Set a password hash or enable OpenID Connect when authentication is enabled.";
      }
    } else if (!maskedSecretRegex.test(password) && !hashRegex.test(password)) {
      errs.password = "Invalid Argon2id hash format.";
    }

    // OIDC Errors
    if (oidc.enabled) {
      if (!oidc.issuer_url.trim()) {
        errs.oidc = "Issuer URL is required.";
      } else if (!isAbsoluteURL(oidc.issuer_url.trim())) {
        errs.oidc = "Issuer URL must be a full URL, e.g. https://auth.example.com/application/o/aura/";
      } else if (!oidc.client_id.trim()) {
        errs.oidc = "Client ID is required.";
      } else if (!oidc.client_secret.trim()) {
        errs.oidc = "Client secret is required.";
      } else if (!oidc.redirect_url.trim()) {
        errs.oidc = "Redirect URL is required.";
      } else if (!isAbsoluteURL(oidc.redirect_url.trim())) {
        errs.oidc = "Redirect URL must be the full callback URL, e.g. https://aura.example.com/api/auth/oidc/callback";
      }
    }

    return errs;
  }, [value.enabled, value.password, oidc]);

  // Emit errors upward
  useEffect(() => {
    if (!errorsUpdate) return;
    const serialized = JSON.stringify(errors);
    if (serialized === prevErrorRef.current) return;
    prevErrorRef.current = serialized;
    errorsUpdate(errors);
  }, [errors, errorsUpdate]);

  return (
    <Card className={`p-5 ${Object.values(errors).some(Boolean) ? "border-red-500" : "border-muted"}`}>
      <div className="flex items-center justify-between">
        <h2 className="text-xl font-semibold text-blue-500">Authentication</h2>
      </div>

      <div
        className={cn(
          "flex items-center justify-between border rounded-md p-3 transition",
          "border-muted",
          dirtyFields.enabled && "border-amber-500"
        )}
      >
        <Label className="mr-2">Enabled</Label>
        <div className="flex items-center gap-2">
          <Switch disabled={!editing} checked={value.enabled} onCheckedChange={(c) => onChange("enabled", c)} />
          {editing && (
            <PopoverHelp ariaLabel="help-auth-enabled">
              <p>Turn on to enforce authentication. A valid Argon2id password hash must be provided below.</p>
            </PopoverHelp>
          )}
        </div>
      </div>

      <div className="flex">
        <div className={cn("relative flex-1 border rounded-md p-3 space-y-2 transition")}>
          <div>
            <div className="flex items-center justify-between">
              <Label htmlFor="auth-hash">Argon2id Password Hash</Label>
              {editing && (
                <PopoverHelp ariaLabel="help-auth-password-hash">
                  <p className="mb-2">
                    Provide an Argon2id hash. If authentication is enabled this hash must match the user&apos;s
                    password.
                  </p>
                  <p className="mb-2">
                    A stored hash is shown masked. Leave it as-is to keep it, paste a new hash to replace it, or clear
                    the field to remove password sign-in once OIDC is enabled.
                  </p>
                  <p>
                    You can use a site like{" "}
                    <Link
                      className="text-primary underline"
                      href="https://argon2.online/"
                      target="_blank"
                      rel="noopener noreferrer"
                    >
                      Argon2.Online
                    </Link>{" "}
                    to generate a hash.
                  </p>
                </PopoverHelp>
              )}
            </div>
            <Input
              id="auth-hash"
              disabled={!editing}
              placeholder="$argon2id$v=19$m=65536,t=3,p=1$..."
              type="text"
              value={value.password}
              onChange={(e) => onChange("password", e.target.value)}
              className={cn("w-full mt-1", dirtyFields.password && "ring-2 ring-amber-500")}
            />
          </div>
          {errors.password && <p className="text-xs text-red-500">{errors.password}</p>}
        </div>
      </div>

      <div className={cn("border rounded-md p-3 space-y-3 transition", dirtyFields.oidc && "border-amber-500")}>
        <div className="flex items-center justify-between">
          <Label className="mr-2 text-base font-semibold">Single Sign-On (OpenID Connect)</Label>
          <div className="flex items-center gap-2">
            <Switch
              disabled={!editing}
              checked={oidc.enabled}
              onCheckedChange={(checked) => updateOIDC("enabled", checked)}
            />
            {editing && (
              <PopoverHelp ariaLabel="help-auth-oidc-enabled">
                <p className="mb-2">
                  Sign in through an identity provider such as Authentik, Keycloak, Auth0 or Google. Register aura as a
                  confidential client with the authorization code flow.
                </p>
                <p>Leave the password hash set as well if you want a way in when the provider is unavailable.</p>
              </PopoverHelp>
            )}
          </div>
        </div>

        {oidc.enabled && (
          <>
            <div>
              <Label htmlFor="oidc-issuer">Issuer URL</Label>
              <Input
                id="oidc-issuer"
                disabled={!editing}
                placeholder="https://auth.example.com/application/o/aura/"
                value={oidc.issuer_url}
                onChange={(e) => updateOIDC("issuer_url", e.target.value)}
                className="w-full mt-1"
              />
            </div>

            <div>
              <Label htmlFor="oidc-client-id">Client ID</Label>
              <Input
                id="oidc-client-id"
                disabled={!editing}
                value={oidc.client_id}
                onChange={(e) => updateOIDC("client_id", e.target.value)}
                className="w-full mt-1"
              />
            </div>

            <div>
              <div className="flex items-center justify-between">
                <Label htmlFor="oidc-client-secret">Client Secret</Label>
                {editing && (
                  <PopoverHelp ariaLabel="help-auth-oidc-secret">
                    <p>
                      The stored secret is shown masked. Leave it as-is to keep it, or paste a new one to replace it.
                    </p>
                  </PopoverHelp>
                )}
              </div>
              <Input
                id="oidc-client-secret"
                disabled={!editing}
                type="text"
                value={oidc.client_secret}
                onChange={(e) => updateOIDC("client_secret", e.target.value)}
                className="w-full mt-1"
              />
            </div>

            <div>
              <div className="flex items-center justify-between">
                <Label htmlFor="oidc-redirect">Redirect URL</Label>
                {editing && (
                  <PopoverHelp ariaLabel="help-auth-oidc-redirect">
                    <p>
                      The callback registered with your provider. It must be the full external URL of this app followed
                      by <code>/api/auth/oidc/callback</code>.
                    </p>
                  </PopoverHelp>
                )}
              </div>
              <Input
                id="oidc-redirect"
                disabled={!editing}
                placeholder="https://aura.example.com/api/auth/oidc/callback"
                value={oidc.redirect_url}
                onChange={(e) => updateOIDC("redirect_url", e.target.value)}
                className="w-full mt-1"
              />
            </div>

            <div>
              <div className="flex items-center justify-between">
                <Label htmlFor="oidc-scopes">Scopes</Label>
                {editing && (
                  <PopoverHelp ariaLabel="help-auth-oidc-scopes">
                    <p>Comma separated. Defaults to openid, profile, email. openid is always requested.</p>
                  </PopoverHelp>
                )}
              </div>
              <Input
                id="oidc-scopes"
                disabled={!editing}
                placeholder="openid, profile, email, groups"
                value={(oidc.scopes ?? []).join(", ")}
                onChange={(e) => updateOIDC("scopes", parseList(e.target.value))}
                className="w-full mt-1"
              />
            </div>

            <div>
              <div className="flex items-center justify-between">
                <Label htmlFor="oidc-groups-claim">Groups Claim</Label>
                {editing && (
                  <PopoverHelp ariaLabel="help-auth-oidc-groups-claim">
                    <p>Which ID token claim holds group membership. Defaults to groups.</p>
                  </PopoverHelp>
                )}
              </div>
              <Input
                id="oidc-groups-claim"
                disabled={!editing}
                placeholder="groups"
                value={oidc.groups_claim ?? ""}
                onChange={(e) => updateOIDC("groups_claim", e.target.value)}
                className="w-full mt-1"
              />
            </div>

            <div>
              <div className="flex items-center justify-between">
                <Label htmlFor="oidc-allowed-groups">Allowed Groups</Label>
                {editing && (
                  <PopoverHelp ariaLabel="help-auth-oidc-allowlist">
                    <p className="mb-2">
                      Comma separated. Leave every allowlist empty to let anyone your provider authenticates in, which
                      is the right choice when the provider already restricts who can open this application.
                    </p>
                    <p>If any list is filled in, a user must match at least one entry across the three lists.</p>
                  </PopoverHelp>
                )}
              </div>
              <Input
                id="oidc-allowed-groups"
                disabled={!editing}
                placeholder="media-users, admins"
                value={(oidc.allowed_groups ?? []).join(", ")}
                onChange={(e) => updateOIDC("allowed_groups", parseList(e.target.value))}
                className="w-full mt-1"
              />
            </div>

            <div>
              <Label htmlFor="oidc-allowed-emails">Allowed Emails</Label>
              <Input
                id="oidc-allowed-emails"
                disabled={!editing}
                placeholder="you@example.com"
                value={(oidc.allowed_emails ?? []).join(", ")}
                onChange={(e) => updateOIDC("allowed_emails", parseList(e.target.value))}
                className="w-full mt-1"
              />
            </div>

            <div>
              <Label htmlFor="oidc-allowed-subjects">Allowed Subjects</Label>
              <Input
                id="oidc-allowed-subjects"
                disabled={!editing}
                value={(oidc.allowed_subjects ?? []).join(", ")}
                onChange={(e) => updateOIDC("allowed_subjects", parseList(e.target.value))}
                className="w-full mt-1"
              />
            </div>

            <div>
              <Label htmlFor="oidc-button-label">Sign-In Button Label</Label>
              <Input
                id="oidc-button-label"
                disabled={!editing}
                placeholder="Sign in with SSO"
                value={oidc.button_label ?? ""}
                onChange={(e) => updateOIDC("button_label", e.target.value)}
                className="w-full mt-1"
              />
            </div>

            <div className="flex items-center justify-between">
              <Label className="mr-2">End Provider Session on Logout</Label>
              <div className="flex items-center gap-2">
                <Switch
                  disabled={!editing}
                  checked={oidc.rp_initiated_logout}
                  onCheckedChange={(checked) => updateOIDC("rp_initiated_logout", checked)}
                />
                {editing && (
                  <PopoverHelp ariaLabel="help-auth-oidc-logout">
                    <p>
                      Sends you to the provider&apos;s logout endpoint after signing out here. The provider must accept
                      the login page URL as a post-logout redirect.
                    </p>
                  </PopoverHelp>
                )}
              </div>
            </div>

            {errors.oidc && <p className="text-xs text-red-500">{errors.oidc}</p>}
          </>
        )}
      </div>
    </Card>
  );
};
