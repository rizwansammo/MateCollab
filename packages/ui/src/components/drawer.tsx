'use client';

import { X } from 'lucide-react';
import { Dialog as DialogPrimitive } from 'radix-ui';
import type { ReactNode } from 'react';
import { cn } from '../lib/cn';
import { IconButton } from './icon-button';

export interface DrawerProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  side?: 'left' | 'right';
  /** Accessible name of the drawer (visually hidden unless showHeader). */
  title: string;
  description?: string;
  showHeader?: boolean;
  /** Width classes; the default is the tablet navigation drawer (85 %, max 300 px). */
  className?: string;
  children: ReactNode;
}

/** Modal side sheet: the context sidebar on tablets and phones, panels on phones. */
export function Drawer({
  open,
  onOpenChange,
  side = 'left',
  title,
  description,
  showHeader = false,
  className,
  children,
}: DrawerProps) {
  return (
    <DialogPrimitive.Root open={open} onOpenChange={onOpenChange}>
      <DialogPrimitive.Portal>
        <DialogPrimitive.Overlay className="fixed inset-0 z-50 animate-fade-in bg-overlay" />
        <DialogPrimitive.Content
          className={cn(
            'fixed inset-y-0 z-50 flex w-[85%] max-w-75 flex-col bg-sidebar text-fg shadow-popover outline-hidden',
            side === 'left'
              ? 'left-0 animate-slide-in-left border-r border-border'
              : 'right-0 animate-slide-in-right border-l border-border',
            className,
          )}
          {...(description ? {} : { 'aria-describedby': undefined })}
        >
          {showHeader ? (
            <div className="flex shrink-0 items-start justify-between gap-3 border-b border-border px-6 py-5">
              <div>
                <DialogPrimitive.Title className="text-16 font-heading">{title}</DialogPrimitive.Title>
                {description ? (
                  <DialogPrimitive.Description className="mt-0.75 text-11 text-fg-muted">
                    {description}
                  </DialogPrimitive.Description>
                ) : null}
              </div>
              <DialogPrimitive.Close asChild>
                <IconButton label="Close" tooltip={false}>
                  <X aria-hidden />
                </IconButton>
              </DialogPrimitive.Close>
            </div>
          ) : (
            <>
              <DialogPrimitive.Title className="sr-only">{title}</DialogPrimitive.Title>
              {description ? (
                <DialogPrimitive.Description className="sr-only">{description}</DialogPrimitive.Description>
              ) : null}
            </>
          )}
          <div className="min-h-0 flex-1 overflow-y-auto">{children}</div>
        </DialogPrimitive.Content>
      </DialogPrimitive.Portal>
    </DialogPrimitive.Root>
  );
}
