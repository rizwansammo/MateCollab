'use client';

import { X } from 'lucide-react';
import { AlertDialog as AlertPrimitive, Dialog as DialogPrimitive } from 'radix-ui';
import type { ComponentProps, ReactNode } from 'react';
import { cn } from '../lib/cn';
import { Button } from './button';
import { IconButton } from './icon-button';

// Modals: 8 px radius, 28 px padding, 520 px (680 px wide); full width on phones. Focus is
// trapped while open and returns to the trigger on close; Esc closes.

const overlayClasses = 'fixed inset-0 z-60 animate-fade-in bg-overlay';
const panelClasses = [
  'fixed top-1/2 left-1/2 z-60 flex max-h-[90dvh] w-[calc(100%-32px)] -translate-x-1/2 -translate-y-1/2 flex-col',
  'animate-pop-in overflow-y-auto rounded-lg border border-border bg-bg p-7 text-fg shadow-popover outline-hidden',
  'max-phone:max-h-[92dvh] max-phone:w-[calc(100%-24px)] max-phone:px-5 max-phone:py-5.75',
].join(' ');

export const Dialog = DialogPrimitive.Root;
export const DialogTrigger = DialogPrimitive.Trigger;
export const DialogClose = DialogPrimitive.Close;

export interface DialogContentProps extends Omit<ComponentProps<typeof DialogPrimitive.Content>, 'title'> {
  title: ReactNode;
  /** Shown under the title and announced when the dialog opens. */
  description?: ReactNode;
  /** Keep the description for assistive technology only. */
  hideDescription?: boolean;
  size?: 'default' | 'wide';
  /** Show the close (×) button. */
  closable?: boolean;
}

export function DialogContent({
  title,
  description,
  hideDescription,
  size = 'default',
  closable = true,
  className,
  children,
  ...props
}: DialogContentProps) {
  return (
    <DialogPrimitive.Portal>
      <DialogPrimitive.Overlay className={overlayClasses} />
      <DialogPrimitive.Content
        className={cn(panelClasses, size === 'wide' ? 'max-w-170' : 'max-w-130', className)}
        {...(description ? {} : { 'aria-describedby': undefined })}
        {...props}
      >
        <div className="mb-5.5 pr-8">
          <DialogPrimitive.Title className="text-19 font-heading tracking-title">
            {title}
          </DialogPrimitive.Title>
          {description ? (
            <DialogPrimitive.Description
              className={cn('mt-2 text-13 leading-[1.6] text-fg-muted', hideDescription && 'sr-only')}
            >
              {description}
            </DialogPrimitive.Description>
          ) : null}
        </div>
        {children}
        {closable ? (
          <DialogPrimitive.Close asChild>
            <IconButton label="Close" className="absolute top-5 right-5" tooltip={false}>
              <X aria-hidden />
            </IconButton>
          </DialogPrimitive.Close>
        ) : null}
      </DialogPrimitive.Content>
    </DialogPrimitive.Portal>
  );
}

/** Action row at the bottom of a dialog: divider, right-aligned buttons. */
export function DialogFooter({ className, ...props }: ComponentProps<'div'>) {
  return (
    <div
      className={cn(
        'mt-5.5 flex items-center justify-end gap-2.5 border-t border-border pt-5 max-phone:flex-col-reverse max-phone:items-stretch',
        className,
      )}
      {...props}
    />
  );
}

export interface ConfirmDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: ReactNode;
  description: ReactNode;
  confirmLabel: string;
  cancelLabel?: string;
  /** Destructive confirmations use the danger button. */
  tone?: 'default' | 'danger';
  onConfirm: () => void;
}

/** Confirmation that must be answered explicitly (no outside-click dismissal). */
export function ConfirmDialog({
  open,
  onOpenChange,
  title,
  description,
  confirmLabel,
  cancelLabel = 'Cancel',
  tone = 'danger',
  onConfirm,
}: ConfirmDialogProps) {
  return (
    <AlertPrimitive.Root open={open} onOpenChange={onOpenChange}>
      <AlertPrimitive.Portal>
        <AlertPrimitive.Overlay className={overlayClasses} />
        <AlertPrimitive.Content className={cn(panelClasses, 'max-w-110')}>
          <AlertPrimitive.Title className="text-19 font-heading tracking-title">{title}</AlertPrimitive.Title>
          <AlertPrimitive.Description className="mt-2 text-13 leading-[1.6] text-fg-muted">
            {description}
          </AlertPrimitive.Description>
          <DialogFooter>
            <AlertPrimitive.Cancel asChild>
              <Button variant="secondary">{cancelLabel}</Button>
            </AlertPrimitive.Cancel>
            <AlertPrimitive.Action asChild>
              <Button variant={tone === 'danger' ? 'danger' : 'primary'} onClick={onConfirm}>
                {confirmLabel}
              </Button>
            </AlertPrimitive.Action>
          </DialogFooter>
        </AlertPrimitive.Content>
      </AlertPrimitive.Portal>
    </AlertPrimitive.Root>
  );
}
