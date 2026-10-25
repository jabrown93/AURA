"use client";

import { ArrowUp } from "lucide-react";

import { useEffect, useState } from "react";

import { Button } from "@/components/ui/button";

import { cn } from "@/lib/cn";

export function JumpToTop() {
  const [isVisible, setIsVisible] = useState(false);

  useEffect(() => {
    const toggleVisibility = () => {
      // Show button when page is scrolled up to given distance
      const scrollThreshold = 300;
      if (window.pageYOffset > scrollThreshold) {
        setIsVisible(true);
      } else {
        setIsVisible(false);
      }
    };

    window.addEventListener("scroll", toggleVisibility);

    return () => {
      window.removeEventListener("scroll", toggleVisibility);
    };
  }, []);

  const scrollToTop = () => {
    window.scrollTo({
      top: 0,
      behavior: "smooth",
    });
  };

  // Hidden near the top of the page
  if (!isVisible) return null;

  return (
    <Button
      variant="ghost"
      size="sm"
      className={cn(
        `order-first cursor-pointer
				rounded-full shadow-lg
				transition-all duration-300
				border-1 border-primary
				bg-primary/50 
				hover:text-black hover:!bg-primary
				`
      )}
      onClick={scrollToTop}
      aria-label="jump to top"
    >
      <ArrowUp className="h-3 w-3" />
      <span className="text-xs hidden sm:inline">Jump to Top</span>
    </Button>
  );
}
