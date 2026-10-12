import type { Metadata } from 'next';
import { Suspense } from 'react';
import { ForgotPasswordView } from '@/features/auth/auth-views';

export const metadata: Metadata = { title: 'Reset your password' };

export default function Page() {
  return (
    <Suspense>
      <ForgotPasswordView />
    </Suspense>
  );
}
