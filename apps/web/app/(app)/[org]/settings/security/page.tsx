import { Page, PageHeader } from '@matecollab/ui';
import type { Metadata } from 'next';
import { SecuritySettings } from '@/features/settings/security-settings';

export const metadata: Metadata = { title: 'Sign-in and security' };

export default function SecurityPage() {
  return (
    <Page>
      <PageHeader
        eyebrow="Personal settings"
        title="Sign-in and security"
        description="Keep your account protected on every device."
      />
      <div className="max-w-177.5">
        <SecuritySettings />
      </div>
    </Page>
  );
}
