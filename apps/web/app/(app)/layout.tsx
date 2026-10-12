import type { ReactNode } from 'react';
import { SessionGate } from '@/features/auth/session-gate';

// Signed-in area: organisation selection and every /{org}/… route.
export default function SignedInLayout({ children }: { children: ReactNode }) {
  return <SessionGate>{children}</SessionGate>;
}
