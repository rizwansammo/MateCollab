'use client';

import { cn } from '@matecollab/ui';
import { useRuntimeConfig } from '@/lib/api';

/** Sidebar footer status, backed by a real GET /api/v1/config through the same origin. */
export function ConnectionStatus() {
  const state = useRuntimeConfig();
  const tone =
    state.status === 'ready'
      ? 'bg-presence-available'
      : state.status === 'error'
        ? 'bg-presence-dnd'
        : 'bg-presence-offline';
  return (
    <p role="status" className="flex items-center gap-1.75 text-11 text-fg-muted">
      <span aria-hidden className={cn('size-1.5 shrink-0 rounded-full', tone)} />
      {state.status === 'loading' ? <span>Connecting…</span> : null}
      {state.status === 'ready' ? <span>Connected · {state.config.version}</span> : null}
      {state.status === 'error' ? (
        <>
          <span>Can’t reach the server</span>
          <button
            type="button"
            onClick={state.retry}
            className="ml-auto font-strong text-primary-ink hover:underline"
          >
            Retry
          </button>
        </>
      ) : null}
    </p>
  );
}
