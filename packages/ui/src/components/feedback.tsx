'use client';

import { Inbox, LoaderCircle } from 'lucide-react';
import { Progress as ProgressPrimitive } from 'radix-ui';
import type { ComponentProps, ReactNode } from 'react';
import { cn } from '../lib/cn';

export interface EmptyStateProps {
  /** Lucide icon element; defaults to an inbox. */
  icon?: ReactNode;
  title: ReactNode;
  description?: ReactNode;
  /** Usually one Button (primary for the recovery action). */
  action?: ReactNode;
  headingLevel?: 2 | 3;
  /** `status` announces changes politely (e.g. reconnecting); `alert` interrupts. */
  role?: 'status' | 'alert';
  className?: string;
}

/** Calm, helpful empty/error/recovery state (prototype `Blank`). */
export function EmptyState({
  icon,
  title,
  description,
  action,
  headingLevel = 2,
  role,
  className,
}: EmptyStateProps) {
  const Heading = headingLevel === 2 ? 'h2' : 'h3';
  return (
    <div role={role} className={cn('flex flex-col items-center px-6 py-13.5 text-center', className)}>
      <div
        aria-hidden
        className="mb-3 flex size-14.5 items-center justify-center rounded-lg border border-border bg-surface text-fg-muted [&_svg]:size-6.75"
      >
        {icon ?? <Inbox />}
      </div>
      <Heading className="mt-2 text-18 font-semibold text-fg">{title}</Heading>
      {description ? <p className="mt-2 max-w-85 text-14 text-fg-muted">{description}</p> : null}
      {action ? <div className="mt-5.5 flex flex-wrap justify-center gap-2.5">{action}</div> : null}
    </div>
  );
}

/** Placeholder block shown while content loads; hidden from assistive technology. */
export function Skeleton({ className, ...props }: ComponentProps<'div'>) {
  return <div aria-hidden className={cn('animate-pulse rounded-md bg-muted', className)} {...props} />;
}

/** Inline loading indicator with an accessible label. */
export function Spinner({ label = 'Loading', className }: { label?: string; className?: string }) {
  return (
    <span role="status" className={cn('inline-flex text-fg-muted', className)}>
      <LoaderCircle className="size-4 animate-spin" aria-hidden />
      <span className="sr-only">{label}</span>
    </span>
  );
}

export interface ProgressProps {
  /** 0–100; null shows an indeterminate bar. */
  value: number | null;
  /** Accessible name, e.g. "Uploading roadmap.pdf". */
  label: string;
  /** Human-readable value, e.g. "1.6 MB of 2.4 MB". */
  valueText?: string;
  className?: string;
}

/** Determinate or indeterminate progress bar. */
export function Progress({ value, label, valueText, className }: ProgressProps) {
  return (
    <ProgressPrimitive.Root
      value={value}
      max={100}
      aria-label={label}
      getValueLabel={valueText ? () => valueText : undefined}
      className={cn('relative h-1.5 w-full overflow-hidden rounded-full bg-border', className)}
    >
      <ProgressPrimitive.Indicator
        className={cn(
          'h-full bg-primary transition-[width] duration-300',
          value === null && 'w-1/3 animate-pulse',
        )}
        style={value === null ? undefined : { width: `${Math.min(100, Math.max(0, value))}%` }}
      />
    </ProgressPrimitive.Root>
  );
}
