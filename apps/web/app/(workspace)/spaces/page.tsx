import type { Metadata } from 'next';
import { ModulePlaceholder } from '@/components/placeholder';

export const metadata: Metadata = { title: 'Spaces' };

export default function SpacesPage() {
  return (
    <ModulePlaceholder
      title="Your Spaces"
      description="A shared home for every team and project."
      sections={[
        {
          title: 'Spaces',
          emptyTitle: 'Your next chapter starts here',
          emptyDescription: 'Create a Space to bring your team and their work together.',
        },
      ]}
    />
  );
}
