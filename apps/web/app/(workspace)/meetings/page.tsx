import { CalendarDays, History } from 'lucide-react';
import type { Metadata } from 'next';
import { ModulePlaceholder } from '@/components/placeholder';

export const metadata: Metadata = { title: 'Meetings' };

export default async function MeetingsPage({ searchParams }: { searchParams: Promise<{ view?: string }> }) {
  const past = (await searchParams).view === 'past';
  return (
    <ModulePlaceholder
      title="Meetings"
      description="Scheduled meetings, calls and their recordings."
      sections={[
        past
          ? {
              title: 'Past meetings',
              icon: <History />,
              emptyTitle: 'No past meetings',
              emptyDescription: 'Meetings you attended will appear here with their notes and recordings.',
            }
          : {
              title: 'Upcoming',
              icon: <CalendarDays />,
              emptyTitle: 'A little room in your calendar',
              emptyDescription: 'Schedule a conversation when your team needs one.',
            },
      ]}
    />
  );
}
