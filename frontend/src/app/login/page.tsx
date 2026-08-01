"use client";

import { AttemptLogin, GetAuthMethods, GetSession } from "@/services/auth/login";
import { Eye, EyeOff, Loader2, Lock } from "lucide-react";

import { useEffect, useState } from "react";

import { useRouter } from "next/navigation";

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

export default function LoginPage() {
  const router = useRouter();
  const [password, setPassword] = useState("");
  const [showPw, setShowPw] = useState(false);
  const [loading, setLoading] = useState(false);
  const [errorMsg, setErrorMsg] = useState<string | null>(null);

  // The session cookie is HttpOnly, so auth state has to come from the backend.
  useEffect(() => {
    let cancelled = false;

    const redirectIfSignedIn = async () => {
      const methods = await GetAuthMethods();
      if (cancelled) return;
      if (methods.data && !methods.data.auth_enabled) {
        router.replace("/");
        return;
      }

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
          <CardDescription>Enter your password to access aura.</CardDescription>
        </CardHeader>
        <form onSubmit={handleSubmit}>
          <CardContent className="space-y-4">
            {errorMsg && (
              <Alert variant="destructive">
                <AlertTitle>Error</AlertTitle>
                <AlertDescription>{errorMsg}</AlertDescription>
              </Alert>
            )}
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
      </Card>
    </div>
  );
}
