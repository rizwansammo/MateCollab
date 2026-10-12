import { CircleCheck } from 'lucide-react';
import type { Metadata } from 'next';
import { ModulePlaceholder } from '@/components/placeholder';

export const metadata: Metadata = { title: 'My action items' };

export default function ActionsPage() {
  return (
    <ModulePlaceholder
      title="My action items"
      description="Follow-ups assigned to you from conversations and meetings."
      sections={[
        {
          title: 'Open',
          icon: <CircleCheck />,
          emptyTitle: 'All clear',
          emptyDescription: 'You have no action items in this view.',
        },
      ]}
    />
  );
}
