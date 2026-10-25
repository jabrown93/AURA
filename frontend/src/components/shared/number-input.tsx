"use client";

import { useEffect, useState } from "react";

import { Input } from "@/components/ui/input";

// NumberInput keeps its own text, so the field can be cleared to type a new number
export function NumberInput({
  id,
  value,
  disabled,
  placeholder,
  min,
  max,
  onChange,
}: {
  id: string;
  value: number | undefined;
  disabled?: boolean;
  placeholder?: string;
  min?: number;
  max?: number;
  onChange: (value: number) => void;
}) {
  const valueText = value?.toString() ?? "";
  const [text, setText] = useState(valueText);
  // Show the saved value again when it changes elsewhere
  useEffect(() => setText(valueText), [valueText]);
  return (
    <Input
      id={id}
      type="number"
      inputMode="numeric"
      disabled={disabled}
      placeholder={placeholder}
      min={min}
      max={max}
      value={text}
      onChange={(e) => {
        setText(e.target.value);
        const number = e.target.valueAsNumber;
        if (!Number.isNaN(number)) onChange(number);
      }}
      onBlur={() => setText(valueText)}
    />
  );
}
