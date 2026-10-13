import { Page, PageHeader } from '@matecollab/ui';
import type { Metadata } from 'next';
import { GeneralSettings } from '@/features/admin/general';

export const metadata: Metadata = { title: 'General' };

export default function GeneralPage() {
  return (
    <Page>
      <PageHeader
        eyebrow="Administration"
        title="General"
        description="Your organisation’s name, defaults and ownership."
      />
      <GeneralSettings />
    </Page>
  );
}
