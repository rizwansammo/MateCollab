import type { Metadata } from 'next';
import { Suspense } from 'react';
import { MFAView } from '@/features/auth/auth-views';

export const metadata: Metadata = { title: 'Two-step verification' };

export default function Page() {
  return (
    <Suspense>
      <MFAView mode="code" />
    </Suspense>
  );
}
