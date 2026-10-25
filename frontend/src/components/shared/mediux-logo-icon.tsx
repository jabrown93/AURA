import { cn } from "@/lib/cn";

// The MediUX logo, sized like an icon
export function MediuxLogoIcon({ className }: { className?: string }) {
  return <img src="/mediux_logo.svg" alt="" aria-hidden="true" className={cn("object-contain", className)} />;
}
