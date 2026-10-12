'use client';

import { BrandMark, Button, EmptyState, Spinner } from '@matecollab/ui';
import { useQuery } from '@tanstack/react-query';
import { CloudOff } from 'lucide-react';
import { useEffect, type ReactNode } from 'react';
import { api, unwrap } from '@/lib/api';
import { applyTheme, parseTheme, THEME_COOKIE } from '@/lib/preferences';
import { SessionProvider, isSignedOut, useSessionQuery } from './session';

/** Full-page placeholder while the session or organisation resolves. */
export function LoadingScreen({ label }: { label: string }) {
  return (
    <div className="flex min-h-dvh flex-col items-center justify-center gap-5 bg-bg">
      <BrandMark size={40} alt="" />
      <Spinner label={label} />
    </div>
  );
}

/**
 * Signed-in area. The proxy already sent visitors without a session cookie to /login; this
 * resolves the session with the API (the authority) and provides it to the app. A 401 is
 * redirected to sign-in by the query client.
 */
export function SessionGate({ children }: { children: ReactNode }) {
  const session = useSessionQuery();
  if (session.isPending || (session.isError && isSignedOut(session.error))) {
    return <LoadingScreen label="Loading your workspace" />;
  }
  if (session.isError) {
    return (
      <main className="flex min-h-dvh items-center justify-center bg-bg">
        <EmptyState
          role="alert"
          icon={<CloudOff />}
          title="MateCollab can’t be reached"
          description="Check your connection. We’ll keep your place."
          action={
            <Button variant="primary" onClick={() => void session.refetch()}>
              Try again
            </Button>
          }
        />
      </main>
    );
  }
  return (
    <SessionProvider value={session.data}>
      <ThemeSync />
      {children}
    </SessionProvider>
  );
}

// The theme preference follows the account across devices: the cookie gives the first paint,
// the server copy wins once it is known.
function ThemeSync() {
  const prefs = useQuery({
    queryKey: ['me', 'preferences'],
    queryFn: () => unwrap(api.GET('/me/preferences')),
    staleTime: Infinity,
  });
  useEffect(() => {
    const theme = prefs.data?.theme;
    if (!theme) return;
    const cookie = document.cookie
      .split('; ')
      .find((c) => c.startsWith(`${THEME_COOKIE}=`))
      ?.split('=')[1];
    if (parseTheme(cookie) !== theme) applyTheme(theme);
  }, [prefs.data?.theme]);
  return null;
}
