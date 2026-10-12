'use client';

import { X } from 'lucide-react';
import { useId, type KeyboardEvent, type ReactNode } from 'react';
import { IconButton } from '../components/icon-button';
import { cn } from '../lib/cn';

export interface ContextPanelProps {
  title: ReactNode;
  description?: ReactNode;
  onClose: () => void;
  children: ReactNode;
  /** Optional element below the scrolling body (e.g. a thread composer). */
  footer?: ReactNode;
}

/**
 * Right context panel (threads, details, profiles): 380 px. ≥ 1400 px it docks beside the
 * workspace (AppFrame reserves the space); below that it overlays without blocking the page;
 * ≤ 1100 px it is 370 px wide and on phones it fills the screen. Esc inside the panel closes it.
 */
export function ContextPanel({ title, description, onClose, children, footer }: ContextPanelProps) {
  const titleId = useId();
  const onKeyDown = (event: KeyboardEvent<HTMLElement>) => {
    if (event.key === 'Escape' && !event.defaultPrevented) {
      event.stopPropagation();
      onClose();
    }
  };
  return (
    <aside
      aria-labelledby={titleId}
      onKeyDown={onKeyDown}
      className={cn(
        'fixed top-14 right-0 bottom-0 z-40 flex w-95 animate-slide-in-right flex-col border-l border-border bg-bg shadow-panel',
        'max-tablet:w-92.5 max-phone:inset-0 max-phone:z-50 max-phone:w-full max-phone:border-l-0',
      )}
    >
      <div className="flex shrink-0 items-start justify-between gap-3 border-b border-border px-6 py-5">
        <div className="min-w-0">
          <h2 id={titleId} className="truncate text-16 font-heading">
            {title}
          </h2>
          {description ? <p className="mt-0.75 truncate text-11 text-fg-muted">{description}</p> : null}
        </div>
        <IconButton label="Close panel" onClick={onClose} tooltipSide="left">
          <X aria-hidden />
        </IconButton>
      </div>
      <div className="min-h-0 flex-1 overflow-y-auto pb-5">{children}</div>
      {footer ? <div className="shrink-0 border-t border-border">{footer}</div> : null}
    </aside>
  );
}
