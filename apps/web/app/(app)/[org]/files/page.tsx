import type { Metadata } from 'next';
import { ModulePlaceholder } from '@/components/placeholder';

export const metadata: Metadata = { title: 'Files' };

const recent = { title: 'Recent', emptyDescription: 'Upload a file or attach one to a conversation.' };
const views: Record<string, { title: string; emptyDescription: string }> = {
  recent,
  shared: {
    title: 'Shared with me',
    emptyDescription: 'Files your teammates share with you will appear here.',
  },
  spaces: { title: 'Space files', emptyDescription: 'Files shared in your Spaces will appear here.' },
};

export default async function FilesPage({ searchParams }: { searchParams: Promise<{ view?: string }> }) {
  const view = views[(await searchParams).view ?? 'recent'] ?? recent;
  return (
    <ModulePlaceholder
      title="Files"
      description="Everything your team shares, in one place."
      sections={[
        {
          title: view.title,
          emptyTitle: 'A home for your shared files',
          emptyDescription: view.emptyDescription,
        },
      ]}
    />
  );
}
