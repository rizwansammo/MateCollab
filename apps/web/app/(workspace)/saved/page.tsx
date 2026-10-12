import { Bookmark } from 'lucide-react';
import type { Metadata } from 'next';
import { ModulePlaceholder } from '@/components/placeholder';

export const metadata: Metadata = { title: 'Saved for later' };

export default function SavedPage() {
  return (
    <ModulePlaceholder
      title="Saved for later"
      description="Useful messages, right where you left them."
      sections={[
        {
          title: 'Saved messages',
          icon: <Bookmark />,
          emptyTitle: 'Keep the useful things close',
          emptyDescription: 'Save a message from any conversation. You’ll find it here when you need it.',
        },
      ]}
    />
  );
}
