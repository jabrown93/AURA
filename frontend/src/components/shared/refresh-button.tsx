"use client";

import { RefreshCcw as RefreshIcon } from "lucide-react";

import { useEffect, useState } from "react";
import { createPortal } from "react-dom";

import { Button } from "@/components/ui/button";

import { cn } from "@/lib/cn";

interface RefreshButtonProps {
  onClick: () => void;
  refreshing?: boolean; // Clicks are ignored while refreshing
  className?: string;
}

// Shown on the right of Jump to Top
export function RefreshButton({ onClick, refreshing = false, className }: RefreshButtonProps) {
  const [container, setContainer] = useState<HTMLElement | null>(null);
  useEffect(() => setContainer(document.getElementById("floating-buttons")), []);
  if (!container) return null;

  return createPortal(
    <Button
      variant="ghost"
      size="sm"
      className={cn(
        `order-last cursor-pointer
				rounded-full shadow-lg
				transition-all duration-300
				border-1 border-primary
				bg-primary/50 
				hover:text-black hover:!bg-primary
				`,
        className
      )}
      onClick={() => !refreshing && onClick()}
      aria-label="refresh"
    >
      <RefreshIcon className="h-3 w-3" />
      <span className="text-xs hidden sm:inline">{refreshing ? "Refreshing..." : "Refresh"}</span>
    </Button>,
    container
  );
}
