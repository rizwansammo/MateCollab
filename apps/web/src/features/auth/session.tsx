'use client';

import type { SessionDocument } from '@matecollab/contracts';
import { useMutation, useQuery, useQueryClient, type QueryClient } from '@tanstack/react-query';
import { useRouter } from 'next/navigation';
import { createContext, useContext } from 'react';
import { ApiError, api, setCsrfToken, unwrap } from '@/lib/api';

export const sessionKey = ['session'] as const;

async function fetchSession(): Promise<SessionDocument> {
  const doc = await unwrap(api.GET('/auth/session'));
  setCsrfToken(doc.csrf_token);
  return doc;
}

/** The session document; `allowAnonymous` pages treat 401 as "signed out" instead of redirecting. */
export function useSessionQuery({ allowAnonymous = false }: { allowAnonymous?: boolean } = {}) {
  return useQuery({
    queryKey: sessionKey,
    queryFn: fetchSession,
    staleTime: 60_000,
    meta: { allowAnonymous },
  });
}

/** True when the query failed because nobody is signed in. */
export function isSignedOut(err: unknown) {
  return err instanceof ApiError && err.status === 401;
}

/** Stores a session document returned by sign-in, so the app starts without another request. */
export function acceptSession(client: QueryClient, doc: SessionDocument) {
  setCsrfToken(doc.csrf_token);
  client.setQueryData(sessionKey, doc);
}

const SessionContext = createContext<SessionDocument | null>(null);
export const SessionProvider = SessionContext.Provider;

/** The signed-in session inside the app (guaranteed by the session gate). */
export function useSession(): SessionDocument {
  const doc = useContext(SessionContext);
  if (!doc) throw new Error('useSession must be used inside the signed-in app');
  return doc;
}

/** Signs out of this browser and returns to the sign-in page. */
export function useSignOut() {
  const client = useQueryClient();
  const router = useRouter();
  return useMutation({
    mutationFn: () => unwrap(api.POST('/auth/logout')),
    meta: { allowAnonymous: true },
    onSettled: () => {
      setCsrfToken(null);
      client.clear();
      router.replace('/login');
    },
  });
}

/** Only same-site paths are accepted as a post-sign-in destination (no open redirects). */
export function safeNext(next: string | null | undefined): string | null {
  if (!next || !next.startsWith('/') || next.startsWith('//') || next.startsWith('/\\')) return null;
  if (next.startsWith('/login')) return null;
  return next;
}
