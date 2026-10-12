import { Page, PageHeader } from '@matecollab/ui';
import type { Metadata } from 'next';
import { MembersDirectory } from '@/features/admin/members';

export const metadata: Metadata = { title: 'Members' };

export default function MembersPage() {
  return (
    <Page>
      <PageHeader
        eyebrow="Administration"
        title="Members"
        description="Everyone in your organisation, their roles and their access."
      />
      <MembersDirectory />
    </Page>
  );
}
