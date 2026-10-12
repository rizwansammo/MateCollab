'use client';

import { cva, type VariantProps } from 'class-variance-authority';
import type { ComponentProps, ReactNode } from 'react';
import { cn } from '../lib/cn';
import { Tooltip, type TooltipProps } from './tooltip';

export const iconButtonVariants = cva(
  [
    'inline-flex shrink-0 items-center justify-center rounded-md transition-colors duration-140',
    'disabled:pointer-events-none disabled:opacity-50 [&_svg:not([class*=size-])]:size-4.5',
  ],
  {
    variants: {
      tone: {
        default: 'text-fg-muted hover:bg-muted hover:text-fg',
        rail: 'text-rail-fg hover:bg-rail-hover hover:text-white',
      },
      size: {
        sm: 'size-6.5',
        md: 'size-7.5',
        lg: 'size-9',
      },
      selected: {
        true: 'bg-accent text-primary-ink hover:bg-accent hover:text-primary-ink',
        false: '',
      },
    },
    defaultVariants: { tone: 'default', size: 'md', selected: false },
  },
);

export interface IconButtonProps
  extends Omit<ComponentProps<'button'>, 'aria-label' | 'children'>, VariantProps<typeof iconButtonVariants> {
  /** Accessible name; also shown as the tooltip. Required: icon-only buttons need a label. */
  label: string;
  children: ReactNode;
  /** Toggle buttons pass their state; it is announced as "pressed". */
  pressed?: boolean;
  tooltip?: boolean;
  tooltipSide?: TooltipProps['side'];
}

/** Icon-only button (30 px) with a mandatory accessible name and matching tooltip. */
export function IconButton({
  label,
  children,
  className,
  tone,
  size,
  selected,
  pressed,
  tooltip = true,
  tooltipSide,
  type,
  ...props
}: IconButtonProps) {
  return (
    <Tooltip content={label} side={tooltipSide} disabled={!tooltip}>
      <button
        type={type ?? 'button'}
        aria-label={label}
        aria-pressed={pressed}
        className={cn(iconButtonVariants({ tone, size, selected: selected ?? pressed ?? false }), className)}
        {...props}
      >
        {children}
      </button>
    </Tooltip>
  );
}
