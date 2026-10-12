import { Page, PageHeader } from '@matecollab/ui';
import type { Metadata } from 'next';
import { ProfileSettings } from '@/features/settings/profile-settings';

export const metadata: Metadata = { title: 'Profile' };

export default function ProfilePage() {
  return (
    <Page>
      <PageHeader
        eyebrow="Personal settings"
        title="Profile"
        description="How you appear across MateCollab."
      />
      <div className="max-w-177.5">
        <ProfileSettings />
      </div>
    </Page>
  );
}
