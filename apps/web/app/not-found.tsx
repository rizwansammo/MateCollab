import { BrandLockup, Button, EmptyState } from '@matecollab/ui';
import Link from 'next/link';

export default function NotFound() {
  return (
    <main className="flex min-h-dvh flex-col items-center justify-center gap-6 bg-bg px-5">
      <BrandLockup height={28} />
      <EmptyState
        title="This page is not available"
        description="Return to your organisation to keep going."
        headingLevel={2}
        action={
          <Button variant="primary" asChild>
            <Link href="/select-organisation">Go to MateCollab</Link>
          </Button>
        }
      />
    </main>
  );
}
