import type { Metadata } from 'next';
import { InvitationView } from '@/features/auth/invitation-view';

export const metadata: Metadata = { title: 'Invitation' };

export default function Page() {
  return <InvitationView />;
}
