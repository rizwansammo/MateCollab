'use client';

import { Tooltip as TooltipPrimitive } from 'radix-ui';
import type { ReactElement, ReactNode } from 'react';

/** Shares hover delay between tooltips; mount once near the root (optional). */
export function TooltipProvider({
  children,
  delayDuration = 250,
}: {
  children: ReactNode;
  delayDuration?: number;
}) {
  return (
    <TooltipPrimitive.Provider delayDuration={delayDuration} skipDelayDuration={300}>
      {children}
    </TooltipPrimitive.Provider>
  );
}

export interface TooltipProps {
  content: ReactNode;
  /** A single focusable element; the tooltip opens on hover and keyboard focus. */
  children: ReactElement;
  side?: 'top' | 'right' | 'bottom' | 'left';
  align?: 'start' | 'center' | 'end';
  disabled?: boolean;
  /** Start open (documentation and visual tests). */
  defaultOpen?: boolean;
}

/** Short supplementary label. Never the only place essential information appears. */
export function Tooltip({
  content,
  children,
  side = 'top',
  align = 'center',
  disabled,
  defaultOpen,
}: TooltipProps) {
  if (disabled || content === null || content === undefined || content === '') return children;
  return (
    <TooltipPrimitive.Provider delayDuration={250} skipDelayDuration={300}>
      <TooltipPrimitive.Root defaultOpen={defaultOpen}>
        <TooltipPrimitive.Trigger asChild>{children}</TooltipPrimitive.Trigger>
        <TooltipPrimitive.Portal>
          <TooltipPrimitive.Content
            side={side}
            align={align}
            sideOffset={6}
            collisionPadding={8}
            className="z-80 max-w-xs animate-fade-in rounded-sm bg-tooltip px-2 py-1 text-12 font-medium text-tooltip-fg shadow-popover"
          >
            {content}
          </TooltipPrimitive.Content>
        </TooltipPrimitive.Portal>
      </TooltipPrimitive.Root>
    </TooltipPrimitive.Provider>
  );
}
