import { cva, type VariantProps } from 'class-variance-authority';
import type { ComponentProps } from 'react';
import { cn } from '../lib/cn';

export const badgeVariants = cva(
  'inline-flex h-5 items-center justify-center gap-1 whitespace-nowrap rounded-sm border px-1.75 text-11 font-strong',
  {
    variants: {
      tone: {
        neutral: 'border-border bg-surface text-fg-muted',
        brand: 'border-transparent bg-accent text-primary-ink',
        success: 'border-transparent bg-success-bg text-success',
        warning: 'border-transparent bg-warning-bg text-warning',
        danger: 'border-transparent bg-danger-badge text-danger-badge-fg',
      },
      /** Status chips such as "UP NEXT": uppercase with letter spacing. */
      caps: {
        true: 'uppercase tracking-[0.7px]',
        false: '',
      },
    },
    defaultVariants: { tone: 'neutral', caps: false },
  },
);

export interface BadgeProps extends ComponentProps<'span'>, VariantProps<typeof badgeVariants> {}

/** Short status or count label. Colour is never the only signal: the text carries the meaning. */
export function Badge({ className, tone, caps, ...props }: BadgeProps) {
  return <span className={cn(badgeVariants({ tone, caps }), className)} {...props} />;
}

/** Keyboard key, e.g. <Kbd>⌘ K</Kbd>. */
export function Kbd({ className, ...props }: ComponentProps<'kbd'>) {
  return (
    <kbd
      className={cn(
        'inline-flex h-5 items-center whitespace-nowrap rounded-sm border border-border bg-bg px-1.25 font-sans text-11 text-fg-muted',
        className,
      )}
      {...props}
    />
  );
}

/** Small numeric count used in navigation (unread items, open actions). */
export function NavCount({ className, active, ...props }: ComponentProps<'span'> & { active?: boolean }) {
  return (
    <span
      className={cn(
        'ml-auto inline-flex h-4.5 min-w-4.5 items-center justify-center rounded-sm px-1 text-11 font-heading tabular-nums',
        active ? 'bg-accent-strong text-accent-strong-fg' : 'bg-nav-count text-nav-count-fg',
        className,
      )}
      {...props}
    />
  );
}
