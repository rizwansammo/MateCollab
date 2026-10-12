'use client';

import { CircleAlert, CircleCheck, Info, X } from 'lucide-react';
import { Toast as ToastPrimitive } from 'radix-ui';
import { useSyncExternalStore, type ReactNode } from 'react';
import { cn } from '../lib/cn';

export type ToastTone = 'neutral' | 'success' | 'danger';

export interface ToastOptions {
  description?: string;
  tone?: ToastTone;
  /** Milliseconds before auto-dismiss (paused while hovered or focused). */
  duration?: number;
  action?: { label: string; onClick: () => void };
}

interface ToastItem extends ToastOptions {
  id: number;
  title: string;
}

// A tiny external store so toasts can be raised from anywhere (event handlers, query callbacks)
// without a provider in every component tree. Render <Toaster /> once.
let items: ToastItem[] = [];
let nextId = 1;
const listeners = new Set<() => void>();
const emit = () => listeners.forEach((listener) => listener());
const subscribe = (listener: () => void) => {
  listeners.add(listener);
  return () => listeners.delete(listener);
};
const EMPTY: ToastItem[] = [];

/** Shows a short, non-blocking notification. Returns its id for dismissToast(). */
export function toast(title: string, options: ToastOptions = {}): number {
  const id = nextId++;
  items = [...items, { id, title, ...options }].slice(-4);
  emit();
  return id;
}
toast.success = (title: string, options: Omit<ToastOptions, 'tone'> = {}) =>
  toast(title, { ...options, tone: 'success' });
toast.error = (title: string, options: Omit<ToastOptions, 'tone'> = {}) =>
  toast(title, { ...options, tone: 'danger' });

export function dismissToast(id: number) {
  items = items.filter((item) => item.id !== id);
  emit();
}

const toneIcon = {
  neutral: <Info className="size-4.5 text-primary-ink" aria-hidden />,
  success: <CircleCheck className="size-4.5 text-success" aria-hidden />,
  danger: <CircleAlert className="size-4.5 text-danger" aria-hidden />,
} satisfies Record<ToastTone, ReactNode>;

/** Renders queued toasts bottom-centre (above the phone navigation bar). */
export function Toaster() {
  const list = useSyncExternalStore(
    subscribe,
    () => items,
    () => EMPTY,
  );
  return (
    <ToastPrimitive.Provider swipeDirection="down" duration={5000} label="Notifications">
      {list.map((item) => (
        <ToastPrimitive.Root
          key={item.id}
          duration={item.duration}
          type={item.tone === 'danger' ? 'foreground' : 'background'}
          onOpenChange={(open) => {
            if (!open) dismissToast(item.id);
          }}
          className={cn(
            'flex w-full animate-slide-in-up items-start gap-3 rounded-md border border-border bg-popover px-4 py-3 text-13 text-fg shadow-popover',
          )}
        >
          <span className="mt-px">{toneIcon[item.tone ?? 'neutral']}</span>
          <div className="min-w-0 flex-1">
            <ToastPrimitive.Title className="font-strong">{item.title}</ToastPrimitive.Title>
            {item.description ? (
              <ToastPrimitive.Description className="mt-0.5 text-12 text-fg-muted">
                {item.description}
              </ToastPrimitive.Description>
            ) : null}
          </div>
          {item.action ? (
            <ToastPrimitive.Action
              altText={item.action.label}
              onClick={item.action.onClick}
              className="shrink-0 rounded-sm px-1.5 py-0.5 text-13 font-strong text-primary-ink hover:bg-muted"
            >
              {item.action.label}
            </ToastPrimitive.Action>
          ) : null}
          <ToastPrimitive.Close
            aria-label="Dismiss notification"
            className="-mr-1 inline-flex size-6 shrink-0 items-center justify-center rounded-sm text-fg-muted hover:bg-muted hover:text-fg"
          >
            <X className="size-3.5" aria-hidden />
          </ToastPrimitive.Close>
        </ToastPrimitive.Root>
      ))}
      <ToastPrimitive.Viewport className="fixed bottom-4 left-1/2 z-70 flex w-[calc(100%-32px)] max-w-90 -translate-x-1/2 flex-col gap-2 outline-hidden max-phone:bottom-18.5" />
    </ToastPrimitive.Provider>
  );
}
