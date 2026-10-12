import type { Metadata } from 'next';
import { Suspense } from 'react';
import { SignInView } from '@/features/auth/auth-views';

export const metadata: Metadata = { title: 'Sign in' };

export default function Page() {
  return (
    <Suspense>
      <SignInView />
    </Suspense>
  );
}
