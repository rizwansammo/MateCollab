import { cookies } from 'next/headers';
import { Suspense, type ReactNode } from 'react';
import { AppShell } from '@/components/shell/app-shell';
import { SIDEBAR_WIDTH_COOKIE, parseSidebarWidth } from '@/lib/preferences';

// Workspace routes. They move under /{org}/ once organisations and sign-in exist (FRONTEND.md §3).
export default async function WorkspaceLayout({ children }: { children: ReactNode }) {
  const width = parseSidebarWidth((await cookies()).get(SIDEBAR_WIDTH_COOKIE)?.value);
  return (
    <Suspense>
      <AppShell initialSidebarWidth={width}>{children}</AppShell>
    </Suspense>
  );
}
