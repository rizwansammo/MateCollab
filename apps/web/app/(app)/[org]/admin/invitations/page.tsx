import { Page, PageHeader } from '@matecollab/ui';
import type { Metadata } from 'next';
import { InvitationsManager } from '@/features/admin/invitations';

export const metadata: Metadata = { title: 'Invitations' };

export default function InvitationsPage() {
  return (
    <Page>
      <PageHeader
        eyebrow="Administration"
        title="Invitations"
        description="Invite people by email and follow their invitations."
      />
      <InvitationsManager />
    </Page>
  );
}
