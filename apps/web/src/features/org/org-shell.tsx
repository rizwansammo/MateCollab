'use client';

import { BrandLockup, Button, EmptyState, Page } from '@matecollab/ui';
import { useQuery } from '@tanstack/react-query';
import { Building2, ShieldAlert } from 'lucide-react';
import type { Route } from 'next';
import Link from 'next/link';
import { usePathname } from 'next/navigation';
import type { ReactNode } from 'react';
import { AppShell } from '@/components/shell/app-shell';
import { useSession } from '@/features/auth/session';
import { ApiError, api, unwrap } from '@/lib/api';
import { OrgProvider, orgKey, orgPath, useOrg } from './org-context';

/**
 * Resolves /{org} against the user's memberships. Organisations the user does not belong to are
 * indistinguishable from ones that do not exist, mirroring the API's 404.
 */
export function OrgShell({
  slug,
  initialSidebarWidth,
  children,
}: {
  slug: string;
  initialSidebarWidth: number;
  children: ReactNode;
}) {
  const { organisations } = useSession();
  const org = organisations.find((o) => o.slug === slug.toLowerCase());
  if (!org) {
    return (
      <Unavailable
        title="Organisation not found"
        description="This organisation doesn’t exist, or you’re not a member of it."
      />
    );
  }
  if (org.status !== 'active') {
    return (
      <Unavailable
        title={`${org.name} is unavailable`}
        description="This organisation is suspended. Contact its owner for details."
      />
    );
  }
  return (
    <OrgProvider value={org}>
      <AppShell org={org} initialSidebarWidth={initialSidebarWidth}>
        <OrgAccessGate>{children}</OrgAccessGate>
      </AppShell>
    </OrgProvider>
  );
}

function Unavailable({ title, description }: { title: string; description: string }) {
  return (
    <main className="flex min-h-dvh flex-col items-center justify-center gap-6 bg-bg px-5">
      <BrandLockup height={28} />
      <EmptyState
        icon={<Building2 />}
        title={title}
        description={description}
        action={
          <Button variant="primary" asChild>
            <Link href="/select-organisation">Choose an organisation</Link>
          </Button>
        }
      />
    </main>
  );
}

/** The organisation's profile; also the probe for organisation-level access policies. */
export function useOrganisationProfile() {
  const org = useOrg();
  return useQuery({
    queryKey: orgKey(org, 'profile'),
    queryFn: () => unwrap(api.GET('/orgs/{orgId}', { params: { path: { orgId: org.id } } })),
  });
}

// An organisation can require two-step verification. Until the user enrols, its pages show how
// to do that; personal settings (where enrolment happens) stay reachable.
function OrgAccessGate({ children }: { children: ReactNode }) {
  const org = useOrg();
  const pathname = usePathname();
  const profile = useOrganisationProfile();
  const personal = pathname.startsWith(orgPath(org, '/settings'));
  if (!personal && profile.error instanceof ApiError && profile.error.code === 'mfa_enrollment_required') {
    return (
      <Page>
        <EmptyState
          icon={<ShieldAlert />}
          title="Two-step verification required"
          description={`${org.name} requires two-step verification. Turn it on for your account to continue.`}
          action={
            <Button variant="primary" asChild>
              <Link href={orgPath(org, '/settings/security') as Route}>Set up two-step verification</Link>
            </Button>
          }
        />
      </Page>
    );
  }
  return children;
}
