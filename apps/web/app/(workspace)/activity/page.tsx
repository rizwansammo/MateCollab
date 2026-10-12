import { Bell } from 'lucide-react';
import type { Metadata } from 'next';
import { ModulePlaceholder } from '@/components/placeholder';

export const metadata: Metadata = { title: 'Activity' };

const titles: Record<string, string> = {
  all: 'All activity',
  mentions: 'Mentions',
  replies: 'Replies',
  meetings: 'Meetings',
  support: 'Support',
};

export default async function ActivityPage({ searchParams }: { searchParams: Promise<{ type?: string }> }) {
  const title = titles[(await searchParams).type ?? 'all'] ?? 'All activity';
  return (
    <ModulePlaceholder
      title="Activity"
      description="The updates that keep you in the loop."
      sections={[
        {
          title,
          icon: <Bell />,
          emptyTitle: 'You’re all caught up',
          emptyDescription: 'New updates will appear here.',
        },
      ]}
    />
  );
}
