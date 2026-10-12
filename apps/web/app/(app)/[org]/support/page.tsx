import { LifeBuoy } from 'lucide-react';
import type { Metadata } from 'next';
import { ModulePlaceholder } from '@/components/placeholder';

export const metadata: Metadata = { title: 'My support requests' };

export default function SupportPage() {
  return (
    <ModulePlaceholder
      title="My support requests"
      description="Requests you raised with your IT or support team, and their progress."
      sections={[
        {
          title: 'Requests',
          icon: <LifeBuoy />,
          emptyTitle: 'No support requests',
          emptyDescription: 'Requests you create from a conversation or this page will appear here.',
        },
      ]}
    />
  );
}
