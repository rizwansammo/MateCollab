import type { Metadata } from 'next';
import { SelectOrganisationView } from '@/features/org/select-organisation';

export const metadata: Metadata = { title: 'Choose an organisation' };

export default function Page() {
  return <SelectOrganisationView />;
}
