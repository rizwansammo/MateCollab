import { Page, PageHeader } from '@matecollab/ui';
import type { Metadata } from 'next';
import { SpacesManager } from '@/features/spaces/space-list';

export const metadata: Metadata = { title: 'Manage Spaces' };
export default function AdminSpacesPage() {
  return (
    <Page>
      <PageHeader
        eyebrow="Administration"
        title="Spaces"
        description="Manage organisation Spaces and their access."
      />
      <SpacesManager administration />
    </Page>
  );
}
