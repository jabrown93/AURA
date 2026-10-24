import { AlertCircle, CheckCircle2, Loader2 } from "lucide-react";

import { Button } from "@/components/ui/button";

import { cn } from "@/lib/cn";

import type { ConfigSectionSaveState } from "@/hooks/use-config-autosave";

interface ConfigSaveStatusProps {
  state?: ConfigSectionSaveState;
  blockedByErrors?: boolean;
  onRetry?: () => void;
  className?: string;
}

// Shows whether a settings section is saving, saved, or failed to save. Shows nothing until a change is saved
export function ConfigSaveStatus({ state, blockedByErrors, onRetry, className }: ConfigSaveStatusProps) {
  const status = state?.status ?? "idle";

  let content: React.ReactNode;
  if (status === "saving") {
    content = (
      <span className="flex items-center gap-1.5 text-muted-foreground">
        <Loader2 className="h-4 w-4 animate-spin" />
        Saving...
      </span>
    );
  } else if (status === "error") {
    content = (
      <span className="flex items-center gap-2 text-red-500">
        <AlertCircle className="h-4 w-4 shrink-0" />
        <span className="line-clamp-2">{state?.error || "Failed to save"}</span>
        {onRetry && (
          <Button variant="outline" size="sm" className="h-7 cursor-pointer" onClick={onRetry}>
            Retry
          </Button>
        )}
      </span>
    );
  } else if (blockedByErrors) {
    content = (
      <span className="flex items-center gap-1.5 text-amber-500">
        <AlertCircle className="h-4 w-4 shrink-0" />
        Fix the errors below to save your changes
      </span>
    );
  } else if (status === "saved") {
    content = (
      <span className="flex items-center gap-1.5 text-green-500">
        <CheckCircle2 className="h-4 w-4" />
        Saved
      </span>
    );
  } else {
    // Nothing to show until a change is saved
    return null;
  }

  return (
    <div className={cn("text-sm", className)} role="status" aria-live="polite">
      {content}
    </div>
  );
}
