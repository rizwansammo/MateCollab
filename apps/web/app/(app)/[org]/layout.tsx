import { cookies } from 'next/headers';
import { Suspense, type ReactNode } from 'react';
import { OrgShell } from '@/features/org/org-shell';
import { SIDEBAR_WIDTH_COOKIE, parseSidebarWidth } from '@/lib/preferences';

// Organisation routes (/{org}/…, FRONTEND.md §3). The slug is resolved against the signed-in
// user's memberships; the API independently checks membership on every tenant request.
export default async function OrganisationLayout({
  children,
  params,
}: {
  children: ReactNode;
  params: Promise<{ org: string }>;
}) {
  const [{ org }, jar] = await Promise.all([params, cookies()]);
  const width = parseSidebarWidth(jar.get(SIDEBAR_WIDTH_COOKIE)?.value);
  return (
    <Suspense>
      <OrgShell slug={org} initialSidebarWidth={width}>
        {children}
      </OrgShell>
    </Suspense>
  );
}
