import type { Metadata } from 'next';
import { Suspense } from 'react';
import { VerifyEmailView } from '@/features/auth/auth-views';

export const metadata: Metadata = { title: 'Confirm your email address' };

export default function Page() {
  return (
    <Suspense>
      <VerifyEmailView />
    </Suspense>
  );
}
