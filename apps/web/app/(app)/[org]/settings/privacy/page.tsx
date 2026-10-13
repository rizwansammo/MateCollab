import { Page, PageHeader } from '@matecollab/ui';
import type { Metadata } from 'next';
import { PrivacySettings } from '@/features/settings/privacy-settings';

export const metadata: Metadata = { title: 'Privacy' };

export default function PrivacyPage() {
  return (
    <Page>
      <PageHeader
        eyebrow="Personal settings"
        title="Privacy"
        description="Control what happens to your account."
      />
      <div className="max-w-177.5">
        <PrivacySettings />
      </div>
    </Page>
  );
}
