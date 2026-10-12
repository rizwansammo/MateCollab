'use client';

import { Search } from 'lucide-react';
import { useEffect, useSyncExternalStore } from 'react';
import { cn } from '../lib/cn';
import { Kbd } from './badge';

const noopSubscribe = () => () => {};
const isApplePlatform = () =>
  typeof navigator !== 'undefined' && /Mac|iPhone|iPad|iPod/.test(navigator.platform || navigator.userAgent);

/** "⌘" on Apple platforms, "Ctrl" elsewhere (server render assumes Ctrl). */
export function useModifierKeyLabel(): string {
  return useSyncExternalStore(
    noopSubscribe,
    () => (isApplePlatform() ? '⌘' : 'Ctrl'),
    () => 'Ctrl',
  );
}

export interface SearchTriggerProps {
  onActivate: () => void;
  label?: string;
  /** Listen for ⌘K / Ctrl+K anywhere on the page. */
  shortcut?: boolean;
  className?: string;
}

/** Global-bar search field look-alike that opens the search palette (⌘/Ctrl K). */
export function SearchTrigger({
  onActivate,
  label = 'Search anything in your workspace',
  shortcut = true,
  className,
}: SearchTriggerProps) {
  const mod = useModifierKeyLabel();
  useEffect(() => {
    if (!shortcut) return;
    const onKeyDown = (event: KeyboardEvent) => {
      if ((event.metaKey || event.ctrlKey) && !event.altKey && event.key.toLowerCase() === 'k') {
        event.preventDefault();
        onActivate();
      }
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [shortcut, onActivate]);

  return (
    <button
      type="button"
      onClick={onActivate}
      aria-keyshortcuts={shortcut ? 'Control+K Meta+K' : undefined}
      aria-haspopup="dialog"
      className={cn(
        'flex h-8.25 min-w-0 max-w-105 flex-1 items-center gap-2.5 rounded-md border border-border bg-surface px-2.5',
        'text-left text-12 text-fg-muted transition-colors duration-140 hover:border-border-control max-phone:text-11',
        className,
      )}
    >
      <Search className="size-4 shrink-0" aria-hidden />
      <span className="truncate">{label}</span>
      {shortcut ? <Kbd className="ml-auto max-phone:hidden">{mod} K</Kbd> : null}
    </button>
  );
}
