import { Page, PageHeader } from '@matecollab/ui';
import type { Metadata } from 'next';
import { SpacesManager } from '@/features/spaces/space-list';

export const metadata: Metadata = { title: 'Spaces' };
export default function SpacesPage() {
  return (
    <Page>
      <PageHeader
        eyebrow="Collaboration"
        title="Spaces"
        description="A dedicated home for your teams and projects."
      />
      <SpacesManager />
    </Page>
  );
}
