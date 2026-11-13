import { Page, PageHeader } from '@matecollab/ui';
import type { Metadata } from 'next';
import { MessagingPolicy } from '@/features/admin/messaging-policy';

export const metadata: Metadata = { title: 'Messaging policy' };
export default function MessagingPolicyPage() {
  return (
    <Page>
      <PageHeader
        eyebrow="Administration"
        title="Messaging policy"
        description="Control who can create new Spaces in your organisation."
      />
      <MessagingPolicy />
    </Page>
  );
}
