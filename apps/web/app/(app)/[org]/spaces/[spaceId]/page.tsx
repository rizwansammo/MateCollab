import { Page } from '@matecollab/ui';
import type { Metadata } from 'next';
import { SpaceDetails } from '@/features/spaces/space-detail';

export const metadata: Metadata = { title: 'Space' };
export default async function SpacePage({ params }: { params: Promise<{ spaceId: string }> }) {
  const { spaceId } = await params;
  return (
    <Page>
      <SpaceDetails spaceId={spaceId} />
    </Page>
  );
}
