import { EmptyState, Page, PageHeader, SectionTitle } from '@matecollab/ui';
import { Bell, CalendarDays, CircleCheck, MessageSquare } from 'lucide-react';
import type { Metadata } from 'next';

export const metadata: Metadata = { title: 'Home' };

export default function HomePage() {
  return (
    <Page className="pt-9.5 wide:pt-11.5">
      <PageHeader
        eyebrow="Your workspace, in focus"
        title="Welcome to MateCollab"
        description="A little clarity before the day gets busy."
        className="mb-4.5 border-b border-border pb-6 wide:[&_h1]:text-32"
      />
      <div className="mt-6.75 grid grid-cols-[minmax(0,1fr)_290px] gap-8.5 wide:grid-cols-[minmax(0,1fr)_330px] wide:gap-11 max-compact:grid-cols-[minmax(0,1fr)_260px] max-compact:gap-6 max-tablet:grid-cols-[minmax(0,1fr)_280px] max-phone:grid-cols-1">
        <div className="min-w-0">
          <section aria-labelledby="continue" className="mt-7.5 first:mt-0">
            <SectionTitle title={<span id="continue">Pick up where you left off</span>} />
            <div className="rounded-md border border-border">
              <EmptyState
                icon={<MessageSquare />}
                title="Start the conversation"
                description="Your recent channels and conversations will be waiting here."
                headingLevel={3}
                className="py-10"
              />
            </div>
          </section>
          <section aria-labelledby="attention" className="mt-7.5 first:mt-0">
            <SectionTitle title={<span id="attention">Your attention, here</span>} />
            <div className="rounded-md border border-border">
              <EmptyState
                icon={<Bell />}
                title="You’re all caught up"
                description="Mentions, replies and updates that need you will appear here."
                headingLevel={3}
                className="py-10"
              />
            </div>
          </section>
        </div>
        <aside
          aria-label="Your day"
          className="border-l border-border pl-7 wide:pl-8.75 max-compact:pl-5.5 max-phone:border-t max-phone:border-l-0 max-phone:pt-6 max-phone:pl-0"
        >
          <SectionTitle title="On your calendar" />
          <EmptyState
            icon={<CalendarDays />}
            title="A little room in your calendar"
            description="Upcoming meetings will appear here."
            headingLevel={3}
            className="px-2 py-8"
          />
          <SectionTitle title="Your action items" />
          <EmptyState
            icon={<CircleCheck />}
            title="All clear"
            description="Action items assigned to you will appear here."
            headingLevel={3}
            className="px-2 py-8"
          />
        </aside>
      </div>
    </Page>
  );
}
