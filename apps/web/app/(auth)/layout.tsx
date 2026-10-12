import type { ReactNode } from 'react';
import { AuthFrame } from '@/features/auth/auth-frame';

// Public pages: sign-in, two-step verification, password recovery, email verification and
// invitations. They need no session; one-time tokens arrive in the URL fragment.
export default function AuthLayout({ children }: { children: ReactNode }) {
  return <AuthFrame>{children}</AuthFrame>;
}
