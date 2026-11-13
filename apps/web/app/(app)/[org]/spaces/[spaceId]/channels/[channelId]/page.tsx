import { Page } from '@matecollab/ui';
import type { Metadata } from 'next';
import { ChannelDetails } from '@/features/spaces/channel-detail';

export const metadata: Metadata = { title: 'Channel' };
export default async function ChannelPage({
  params,
}: {
  params: Promise<{ spaceId: string; channelId: string }>;
}) {
  const { spaceId, channelId } = await params;
  return (
    <Page>
      <ChannelDetails spaceId={spaceId} channelId={channelId} />
    </Page>
  );
}
