import { Page, PageHeader } from '@matecollab/ui';
import type { Metadata } from 'next';
import { RolesAndPermissions } from '@/features/admin/roles';

export const metadata: Metadata = { title: 'Roles and permissions' };

export default function RolesPage() {
  return (
    <Page>
      <PageHeader
        eyebrow="Administration"
        title="Roles and permissions"
        description="What each role can do in this organisation, with your current settings applied."
      />
      <RolesAndPermissions />
    </Page>
  );
}
