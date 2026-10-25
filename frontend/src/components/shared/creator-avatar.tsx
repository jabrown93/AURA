import { User } from "lucide-react";

import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";

import { cn } from "@/lib/cn";

// A MediUX creator's avatar, or a person icon when they don't have one
export function CreatorAvatar({
  username,
  className,
  iconClassName,
}: {
  username: string;
  className?: string;
  iconClassName?: string;
}) {
  return (
    <Avatar className={cn("rounded-md", className)}>
      <AvatarImage src={`/api/images/mediux/avatar?username=${username}`} />
      <AvatarFallback>
        <User className={cn("h-4 w-4", iconClassName)} />
      </AvatarFallback>
    </Avatar>
  );
}
