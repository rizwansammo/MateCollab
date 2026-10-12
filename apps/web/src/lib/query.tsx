'use client';

import { MutationCache, QueryCache, QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { useState, type ReactNode } from 'react';
import { ApiError } from './api';

declare module '@tanstack/react-query' {
  interface Register {
    queryMeta: { allowAnonymous?: boolean };
    mutationMeta: { allowAnonymous?: boolean };
  }
}

/** Where the browser goes when the session ends mid-use; the current page becomes `next`. */
export function signInUrl(reason?: 'expired') {
  const here = `${window.location.pathname}${window.location.search}`;
  const params = new URLSearchParams();
  if (here !== '/' && !here.startsWith('/login')) params.set('next', here);
  if (reason) params.set('reason', reason);
  const query = params.toString();
  return query ? `/login?${query}` : '/login';
}

function onAuthError(err: unknown, allowAnonymous: boolean | undefined) {
  if (allowAnonymous || !(err instanceof ApiError) || err.status !== 401) return;
  if (err.code === 'unauthenticated' || err.code === 'session_expired') {
    window.location.assign(signInUrl(err.code === 'session_expired' ? 'expired' : undefined));
  }
}

// Client errors (4xx) are answers, not outages: retrying them only delays the message.
function retry(count: number, err: unknown) {
  if (err instanceof ApiError && err.status >= 400 && err.status < 500) return false;
  return count < 2;
}

export function QueryProvider({ children }: { children: ReactNode }) {
  const [client] = useState(
    () =>
      new QueryClient({
        queryCache: new QueryCache({ onError: (err, query) => onAuthError(err, query.meta?.allowAnonymous) }),
        mutationCache: new MutationCache({
          onError: (err, _vars, _ctx, mutation) => onAuthError(err, mutation.meta?.allowAnonymous),
        }),
        defaultOptions: {
          queries: { retry, staleTime: 30_000, refetchOnWindowFocus: false },
          mutations: { retry: false },
        },
      }),
  );
  return <QueryClientProvider client={client}>{children}</QueryClientProvider>;
}
