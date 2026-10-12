import type { Metadata } from 'next';
import { Suspense } from 'react';
import { MFAView } from '@/features/auth/auth-views';

export const metadata: Metadata = { title: 'Recovery code' };

export default function Page() {
  return (
    <Suspense>
      <MFAView mode="recovery" />
    </Suspense>
  );
}
