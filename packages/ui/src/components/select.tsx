'use client';

import { Check, ChevronDown } from 'lucide-react';
import { Select as SelectPrimitive } from 'radix-ui';
import { cn } from '../lib/cn';
import { useFieldControl } from './field';
import { controlClasses } from './input';

export interface SelectOption {
  value: string;
  label: string;
  disabled?: boolean;
}

export interface SelectProps {
  options: SelectOption[];
  value?: string;
  defaultValue?: string;
  onValueChange?: (value: string) => void;
  placeholder?: string;
  disabled?: boolean;
  required?: boolean;
  name?: string;
  id?: string;
  className?: string;
  'aria-label'?: string;
  'aria-describedby'?: string;
  'aria-invalid'?: boolean;
}

/** Single choice from a short list (keyboard: arrows, type-ahead, Enter, Esc). */
export function Select({
  options,
  value,
  defaultValue,
  onValueChange,
  placeholder,
  disabled,
  name,
  className,
  ...rest
}: SelectProps) {
  const control = useFieldControl(rest);
  return (
    <SelectPrimitive.Root
      value={value}
      defaultValue={defaultValue}
      onValueChange={onValueChange}
      disabled={disabled}
      required={control.required}
      name={name}
    >
      <SelectPrimitive.Trigger
        id={control.id}
        aria-label={control['aria-label']}
        aria-describedby={control['aria-describedby']}
        aria-invalid={control['aria-invalid']}
        className={cn(
          controlClasses,
          'flex h-9.5 items-center justify-between gap-2 text-left text-13 data-placeholder:text-fg-muted',
          className,
        )}
      >
        <span className="truncate">
          <SelectPrimitive.Value placeholder={placeholder} />
        </span>
        <SelectPrimitive.Icon asChild>
          <ChevronDown className="size-4 shrink-0 text-fg-muted" aria-hidden />
        </SelectPrimitive.Icon>
      </SelectPrimitive.Trigger>
      <SelectPrimitive.Portal>
        <SelectPrimitive.Content
          position="popper"
          sideOffset={4}
          collisionPadding={8}
          className="relative z-65 max-h-(--radix-select-content-available-height) min-w-(--radix-select-trigger-width) animate-pop-in overflow-hidden rounded-md border border-border bg-popover p-1.25 text-13 text-fg shadow-popover"
        >
          <SelectPrimitive.Viewport>
            {options.map((option) => (
              <SelectPrimitive.Item
                key={option.value}
                value={option.value}
                disabled={option.disabled}
                className="relative flex cursor-pointer select-none items-center rounded-sm py-2 pr-8 pl-2.5 outline-hidden data-disabled:cursor-not-allowed data-disabled:opacity-50 data-highlighted:bg-muted"
              >
                <SelectPrimitive.ItemText>{option.label}</SelectPrimitive.ItemText>
                <SelectPrimitive.ItemIndicator className="absolute right-2.5 inline-flex">
                  <Check className="size-4 text-primary-ink" aria-hidden />
                </SelectPrimitive.ItemIndicator>
              </SelectPrimitive.Item>
            ))}
          </SelectPrimitive.Viewport>
        </SelectPrimitive.Content>
      </SelectPrimitive.Portal>
    </SelectPrimitive.Root>
  );
}
