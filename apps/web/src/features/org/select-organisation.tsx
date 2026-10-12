'use client';

import { Button, OrgBadge, Spinner } from '@matecollab/ui';
import { ChevronRight, LogOut } from 'lucide-react';
import type { Route } from 'next';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { useEffect } from 'react';
import { AuthFootnote, AuthFrame, AuthHeading } from '@/features/auth/auth-frame';
import { useSession, useSignOut } from '@/features/auth/session';
import { orgPath } from './org-context';
import { roleNames } from './roles';

/** After sign-in: open the only organisation, or let the user choose (prototype "workspace" step). */
export function SelectOrganisationView() {
  const { user, organisations } = useSession();
  const router = useRouter();
  const signOut = useSignOut();
  const only = organisations.length === 1 ? organisations[0] : null;

  useEffect(() => {
    if (only) router.replace(orgPath(only, '/home') as Route);
  }, [only, router]);

  return (
    <AuthFrame>
      {only ? (
        <div className="flex items-center gap-3 text-14 text-fg-muted">
          <Spinner label={`Opening ${only.name}`} /> Opening {only.name}…
        </div>
      ) : organisations.length === 0 ? (
        <>
          <AuthHeading title="No organisation yet">
            You’re signed in as <strong className="font-strong text-fg">{user.primary_email}</strong>, but you
            aren’t a member of an organisation. Ask an administrator to invite you.
          </AuthHeading>
          <Button
            variant="secondary"
            className="h-10 w-full"
            loading={signOut.isPending}
            onClick={() => signOut.mutate()}
          >
            <LogOut aria-hidden /> Sign out
          </Button>
        </>
      ) : (
        <>
          <AuthHeading title="Choose an organisation">
            Signed in as <strong className="font-strong text-fg">{user.primary_email}</strong>.
          </AuthHeading>
          <ul className="flex flex-col gap-2.5" aria-label="Your organisations">
            {organisations.map((o) => (
              <li key={o.id}>
                <Link
                  href={orgPath(o, '/home') as Route}
                  className="flex items-center gap-4 rounded-md border border-border bg-card px-4 py-3.5 transition-colors duration-140 hover:border-org-line hover:bg-accent"
                >
                  <OrgBadge name={o.name} className="size-10.5 text-19" />
                  <span className="min-w-0 flex-1">
                    <strong className="block truncate text-14 font-heading">{o.name}</strong>
                    <small className="block text-12 text-fg-muted">
                      {roleNames[o.role]}
                      {o.status !== 'active' ? ' · Suspended' : ''}
                    </small>
                  </span>
                  <ChevronRight aria-hidden className="size-4 text-fg-muted" />
                </Link>
              </li>
            ))}
          </ul>
          <AuthFootnote>
            <button
              type="button"
              className="underline-offset-4 hover:underline"
              onClick={() => signOut.mutate()}
            >
              Sign in with a different account
            </button>
          </AuthFootnote>
        </>
      )}
    </AuthFrame>
  );
}
