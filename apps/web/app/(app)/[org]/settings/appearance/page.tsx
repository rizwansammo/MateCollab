import { Page, PageHeader } from '@matecollab/ui';
import type { Metadata } from 'next';
import { cookies } from 'next/headers';
import { AppearanceForm } from '@/features/settings/appearance-form';
import { THEME_COOKIE, parseTheme } from '@/lib/preferences';

export const metadata: Metadata = { title: 'Appearance' };

export default async function AppearancePage() {
  const theme = parseTheme((await cookies()).get(THEME_COOKIE)?.value);
  return (
    <Page>
      <PageHeader
        eyebrow="Your workspace, your way"
        title="Appearance"
        description="Make MateCollab work comfortably for you."
      />
      <AppearanceForm initialTheme={theme} />
    </Page>
  );
}
