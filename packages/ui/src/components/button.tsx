'use client';

import { cva, type VariantProps } from 'class-variance-authority';
import { LoaderCircle } from 'lucide-react';
import { Slot } from 'radix-ui';
import type { ComponentProps } from 'react';
import { cn } from '../lib/cn';

export const buttonVariants = cva(
  [
    'inline-flex shrink-0 items-center justify-center gap-1.75 whitespace-nowrap rounded-md border font-strong',
    'transition-colors duration-140 disabled:pointer-events-none disabled:opacity-50',
    'aria-disabled:pointer-events-none aria-disabled:opacity-50',
    '[&_svg]:shrink-0 [&_svg:not([class*=size-])]:size-4',
  ],
  {
    variants: {
      variant: {
        primary: 'border-primary bg-primary text-white hover:border-primary-hover hover:bg-primary-hover',
        secondary: 'border-border bg-bg text-fg hover:bg-muted',
        ghost: 'border-transparent bg-transparent text-fg hover:bg-muted',
        danger:
          'border-danger-solid bg-danger-solid text-white hover:border-danger-solid-hover hover:bg-danger-solid-hover',
        link: 'border-0 bg-transparent text-primary-ink underline-offset-4 hover:underline',
      },
      size: {
        sm: 'h-7.75 px-2.75 text-12',
        md: 'h-8.75 px-3.25 text-13',
        lg: 'h-9.5 px-4 text-14',
      },
    },
    compoundVariants: [{ variant: 'link', className: 'h-auto px-0' }],
    defaultVariants: { variant: 'secondary', size: 'md' },
  },
);

export interface ButtonProps extends ComponentProps<'button'>, VariantProps<typeof buttonVariants> {
  /** Render the single child (e.g. a link) with button styling instead of a <button>. */
  asChild?: boolean;
  /** Shows a spinner, sets aria-busy and blocks further clicks. */
  loading?: boolean;
}

/** Text button: primary, secondary (default), ghost, danger or link. 35 px tall (31 px small). */
export function Button({
  className,
  variant,
  size,
  asChild = false,
  loading = false,
  disabled,
  type,
  children,
  ...props
}: ButtonProps) {
  const classes = cn(buttonVariants({ variant, size }), className);
  if (asChild) {
    return (
      <Slot.Root className={classes} {...props}>
        {children}
      </Slot.Root>
    );
  }
  return (
    <button
      type={type ?? 'button'}
      className={classes}
      disabled={disabled || loading}
      aria-busy={loading || undefined}
      {...props}
    >
      {loading ? <LoaderCircle className="animate-spin" aria-hidden /> : null}
      {children}
    </button>
  );
}
