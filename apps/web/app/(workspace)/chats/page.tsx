import { MessageSquare } from 'lucide-react';
import type { Metadata } from 'next';
import { ModulePlaceholder } from '@/components/placeholder';

export const metadata: Metadata = { title: 'Chats' };

export default function ChatsPage() {
  return (
    <ModulePlaceholder
      title="Chats"
      description="Direct messages and group conversations with your team."
      sections={[
        {
          title: 'Conversations',
          icon: <MessageSquare />,
          emptyTitle: 'Start the conversation',
          emptyDescription: 'A good conversation starts with a simple hello.',
        },
      ]}
    />
  );
}
