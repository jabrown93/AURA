"use client";

import {
  AttemptLogin,
  type AuthMethods_Response,
  GetAuthMethods,
  GetSession,
  OIDC_LOGIN_PATH,
} from "@/services/auth/login";
import { Eye, EyeOff, KeyRound, Loader2, Lock } from "lucide-react";

import { useEffect, useState } from "react";

import { useRouter } from "next/navigation";

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

// The OIDC callback redirects here with a code rather than provider text, so the wording
// stays under our control.
const OIDC_ERRORS: Record<string, string> = {
  oidc_disabled: "Single sign-on is not enabled.",
  oidc_provider: "The identity provider could not be reached or refused the sign-in.",
  oidc_state: "The sign-in attempt expired or did not match. Please try again.",
  oidc_exchange: "The identity provider rejected the authorization code.",
  oidc_verify: "The identity token could not be verified.",
  oidc_forbidden: "Your account is not permitted to access aura.",
  oidc_internal: "Something went wrong while completing sign-in.",
};

export default function LoginPage() {
  const router = useRouter();
  const [password, setPassword] = useState("");
  const [showPw, setShowPw] = useState(false);
  const [loading, setLoading] = useState(false);
  const [errorMsg, setErrorMsg] = useState<string | null>(null);
  const [methods, setMethods] = useState<AuthMethods_Response | null>(null);

  // Surface a failed OIDC round trip. Read from the URL directly rather than through
  // useSearchParams, which would force this page behind a Suspense boundary.
  useEffect(() => {
    const code = new URLSearchParams(window.location.search).get("error");
    if (code) {
      setErrorMsg(OIDC_ERRORS[code] ?? "Sign-in failed.");
    }
  }, []);

  // The session cookie is HttpOnly, so auth state has to come from the backend.
  useEffect(() => {
    let cancelled = false;

    const redirectIfSignedIn = async () => {
      const response = await GetAuthMethods();
      if (cancelled) return;
      if (response.data && !response.data.auth_enabled) {
        router.replace("/");
        return;
      }
      setMethods(response.data ?? null);

      const session = await GetSession();
      if (!cancelled && session.data?.authenticated) {
        router.replace("/");
      }
    };

    void redirectIfSignedIn();
    return () => {
      cancelled = true;
    };
  }, [router]);

  // Until the methods load, assume a password form: it is what every existing install has.
  const passwordEnabled = methods?.password_enabled ?? true;
  const oidcEnabled = methods?.oidc_enabled ?? false;

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setErrorMsg(null);
    if (!password) {
      setErrorMsg("Password required.");
      return;
    }
    try {
      setLoading(true);
      const resp = await AttemptLogin(password);
      if (resp.status === "error" || !resp.data?.token) {
        throw new Error(resp.error?.message || "Invalid Password");
      }
      router.replace("/");
    } catch (err: unknown) {
      setErrorMsg((err as { message?: string })?.message || "Login failed. Check password.");
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="min-h-screen flex items-center justify-center px-8 pb-20 sm:px-20">
      <Card className="w-full max-w-md shadow-md">
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-2xl">
            <Lock className="h-6 w-6" /> Sign In
          </CardTitle>
          <CardDescription>
            {passwordEnabled
              ? "Enter your password to access aura."
              : "Sign in with your identity provider to access aura."}
          </CardDescription>
        </CardHeader>

        {errorMsg && (
          <CardContent>
            <Alert variant="destructive">
              <AlertTitle>Error</AlertTitle>
              <AlertDescription>{errorMsg}</AlertDescription>
            </Alert>
          </CardContent>
        )}

        {oidcEnabled && (
          <CardContent className="space-y-4">
            {/* A full navigation, not a fetch: the browser has to follow the redirect
                chain to the provider and back. */}
            <Button asChild variant="outline" className="w-full">
              <a href={OIDC_LOGIN_PATH}>
                <KeyRound className="mr-2 h-4 w-4" />
                {methods?.oidc_button_label || "Sign in with SSO"}
              </a>
            </Button>
            {passwordEnabled && (
              <div className="flex items-center gap-3">
                <span className="h-px flex-1 bg-border" />
                <span className="text-xs text-muted-foreground uppercase">or</span>
                <span className="h-px flex-1 bg-border" />
              </div>
            )}
          </CardContent>
        )}

        {passwordEnabled && (
          <form onSubmit={handleSubmit}>
            <CardContent className="space-y-4">
              <div className="mb-4">
                <Label className="mb-2 font-medium" htmlFor="password">
                  Password
                </Label>
                <div className="relative">
                  <Input
                    id="password"
                    type={showPw ? "text" : "password"}
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                    autoComplete="current-password"
                    placeholder="••••••••"
                    disabled={loading}
                    className="pr-10" // add padding so text doesn't run under the icon
                  />
                  <Button
                    variant="ghost"
                    onClick={() => setShowPw(!showPw)}
                    aria-label={showPw ? "Hide password" : "Show password"}
                    className="absolute top-1/2 right-3 -translate-y-1/2 text-muted-foreground hover:text-foreground transition-colors"
                    disabled={loading}
                  >
                    {showPw ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                  </Button>
                </div>
              </div>
            </CardContent>
            <CardFooter className="flex flex-col gap-3">
              <Button type="submit" className="w-full" disabled={loading}>
                {loading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                {loading ? "Signing In..." : "Sign In"}
              </Button>
            </CardFooter>
          </form>
        )}
      </Card>
    </div>
  );
}
