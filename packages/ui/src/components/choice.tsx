'use client';

import { Check, Minus } from 'lucide-react';
import {
  Checkbox as CheckboxPrimitive,
  RadioGroup as RadioPrimitive,
  Switch as SwitchPrimitive,
} from 'radix-ui';
import type { ComponentProps, ReactNode } from 'react';
import { cn } from '../lib/cn';
import { useFieldControl } from './field';

/** 16 px checkbox. Checked fill uses the control token (≥ 3:1 against surface and tick). */
export function Checkbox({ className, ...props }: ComponentProps<typeof CheckboxPrimitive.Root>) {
  return (
    <CheckboxPrimitive.Root
      className={cn(
        'peer inline-flex size-4 shrink-0 items-center justify-center rounded-sm border border-border-control bg-bg text-white',
        'transition-colors duration-140 disabled:cursor-not-allowed disabled:opacity-50',
        'data-[state=checked]:border-primary-control data-[state=checked]:bg-primary-control',
        'data-[state=indeterminate]:border-primary-control data-[state=indeterminate]:bg-primary-control',
        'aria-invalid:border-danger',
        className,
      )}
      {...useFieldControl(props)}
    >
      <CheckboxPrimitive.Indicator className="inline-flex">
        {props.checked === 'indeterminate' ? (
          <Minus className="size-3" strokeWidth={3.5} aria-hidden />
        ) : (
          <Check className="size-3" strokeWidth={3.5} aria-hidden />
        )}
      </CheckboxPrimitive.Indicator>
    </CheckboxPrimitive.Root>
  );
}

/** Group of mutually exclusive options; arrow keys move the selection. */
export function RadioGroup({ className, ...props }: ComponentProps<typeof RadioPrimitive.Root>) {
  return <RadioPrimitive.Root className={cn('grid gap-2.5', className)} {...props} />;
}

export function RadioGroupItem({ className, ...props }: ComponentProps<typeof RadioPrimitive.Item>) {
  return (
    <RadioPrimitive.Item
      className={cn(
        'inline-flex size-4 shrink-0 items-center justify-center rounded-full border border-border-control bg-bg',
        'transition-colors duration-140 disabled:cursor-not-allowed disabled:opacity-50',
        'data-[state=checked]:border-primary-control',
        className,
      )}
      {...props}
    >
      <RadioPrimitive.Indicator className="size-2 rounded-full bg-primary-control" />
    </RadioPrimitive.Item>
  );
}

/** On/off switch for settings that apply immediately. */
export function Switch({ className, ...props }: ComponentProps<typeof SwitchPrimitive.Root>) {
  return (
    <SwitchPrimitive.Root
      className={cn(
        'inline-flex h-5 w-9 shrink-0 items-center rounded-full border-2 border-transparent bg-switch-off',
        'transition-colors duration-140 disabled:cursor-not-allowed disabled:opacity-50',
        'data-[state=checked]:bg-primary-control',
        className,
      )}
      {...useFieldControl(props)}
    >
      <SwitchPrimitive.Thumb className="pointer-events-none block size-4 rounded-full bg-white shadow-raised transition-transform duration-140 data-[state=checked]:translate-x-4" />
    </SwitchPrimitive.Root>
  );
}

/** Large selectable cards for a single choice with a visual preview (e.g. the theme picker). */
export function RadioCardGroup({ className, ...props }: ComponentProps<typeof RadioPrimitive.Root>) {
  return <RadioPrimitive.Root className={cn('grid gap-4', className)} {...props} />;
}

export interface RadioCardProps {
  value: string;
  label: ReactNode;
  /** Decorative illustration above the label. */
  preview?: ReactNode;
  disabled?: boolean;
  className?: string;
}

export function RadioCard({ value, label, preview, disabled, className }: RadioCardProps) {
  return (
    <RadioPrimitive.Item
      value={value}
      disabled={disabled}
      className={cn(
        'group flex flex-col gap-2.5 rounded-md border border-border bg-bg p-2.5 text-left',
        'transition-colors duration-140 hover:border-border-control disabled:cursor-not-allowed disabled:opacity-50',
        'data-[state=checked]:border-primary data-[state=checked]:ring-1 data-[state=checked]:ring-primary',
        className,
      )}
    >
      {preview ? (
        <span aria-hidden className="block overflow-hidden rounded-sm border border-border">
          {preview}
        </span>
      ) : null}
      <span className="flex items-center gap-2 px-1 pb-0.5 text-13 font-medium text-fg [&_svg]:size-4 [&_svg]:shrink-0">
        {label}
        <Check
          className="ml-auto text-primary-ink opacity-0 group-data-[state=checked]:opacity-100"
          aria-hidden
        />
      </span>
    </RadioPrimitive.Item>
  );
}

export interface ChoiceLabelProps {
  control: ReactNode;
  label: ReactNode;
  description?: ReactNode;
  className?: string;
}

/** A checkbox, radio or switch with its clickable text label (and optional description). */
export function ChoiceLabel({ control, label, description, className }: ChoiceLabelProps) {
  return (
    <label className={cn('flex cursor-pointer items-start gap-2.5 text-13 text-fg', className)}>
      <span className="mt-0.5 inline-flex">{control}</span>
      <span className="flex flex-col gap-0.5">
        <span className="font-strong">{label}</span>
        {description ? <span className="text-12 text-fg-muted">{description}</span> : null}
      </span>
    </label>
  );
}
