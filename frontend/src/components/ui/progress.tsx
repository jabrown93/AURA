"use client";

import * as ProgressPrimitive from "@radix-ui/react-progress";

import * as React from "react";

import { cn } from "@/lib/cn";

function Progress({ className, value, ...props }: React.ComponentProps<typeof ProgressPrimitive.Root>) {
  return (
    <ProgressPrimitive.Root
      data-slot="progress"
      className={cn("bg-primary/20 relative h-2 w-full overflow-hidden rounded-full", className)}
      {...props}
    >
      <ProgressPrimitive.Indicator
        data-slot="progress-indicator"
        className="bg-primary h-full transition-[width]"
        style={{ width: `${Math.min(Math.max(value || 0, 0), 100)}%` }}
      />
    </ProgressPrimitive.Root>
  );
}

export { Progress };
