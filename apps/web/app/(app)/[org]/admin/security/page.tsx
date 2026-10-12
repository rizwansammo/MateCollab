import { Page, PageHeader } from '@matecollab/ui';
import type { Metadata } from 'next';
import { SecurityPolicy } from '@/features/admin/security-policy';

export const metadata: Metadata = { title: 'Security policy' };

export default function SecurityPolicyPage() {
  return (
    <Page>
      <PageHeader
        eyebrow="Administration"
        title="Security policy"
        description="Sign-in and invitation rules for everyone in your organisation."
      />
      <SecurityPolicy />
    </Page>
  );
}
