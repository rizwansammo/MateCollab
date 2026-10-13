import { Page, PageHeader } from '@matecollab/ui';
import type { Metadata } from 'next';
import { AuditLog } from '@/features/admin/audit-log';

export const metadata: Metadata = { title: 'Audit log' };

export default function AuditPage() {
  return (
    <Page>
      <PageHeader
        eyebrow="Administration"
        title="Audit log"
        description="Who changed what in your organisation, and when."
      />
      <AuditLog />
    </Page>
  );
}
