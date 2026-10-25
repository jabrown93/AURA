"use client";

import { Plus } from "lucide-react";

import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";

import { cn } from "@/lib/cn";

interface VariablePickerProps {
  variables: string[];
  onPick: (variable: string) => void;
  disabled?: boolean;
  label: string; // What the variable is added to, for screen readers
  className?: string;
}

// VariablePicker is a + button with a menu of variables to add to a field
export const VariablePicker: React.FC<VariablePickerProps> = ({ variables, onPick, disabled, label, className }) => (
  <DropdownMenu>
    <DropdownMenuTrigger asChild>
      <Button
        variant="outline"
        size="icon"
        className={cn("h-8 w-8 sm:h-9 sm:w-9 shrink-0 cursor-pointer", className)}
        disabled={disabled || variables.length === 0}
        aria-label={`Add a variable to ${label}`}
      >
        <Plus className="h-4 w-4" />
      </Button>
    </DropdownMenuTrigger>
    <DropdownMenuContent align="start" className="max-h-72 overflow-y-auto">
      {variables.map((variable) => (
        <DropdownMenuItem key={variable} className="cursor-pointer font-mono text-xs" onSelect={() => onPick(variable)}>
          {variable}
        </DropdownMenuItem>
      ))}
    </DropdownMenuContent>
  </DropdownMenu>
);

// insertAtCursor puts the variable at the field's cursor, or at the end, and returns the new text.
// Once the field shows the new text, the cursor is put after the variable.
export const insertAtCursor = (
  field: HTMLInputElement | HTMLTextAreaElement | null | undefined,
  text: string,
  variable: string
) => {
  const start = field?.selectionStart ?? text.length;
  const end = field?.selectionEnd ?? text.length;
  requestAnimationFrame(() => {
    field?.focus();
    field?.setSelectionRange(start + variable.length, start + variable.length);
  });
  return text.slice(0, start) + variable + text.slice(end);
};
