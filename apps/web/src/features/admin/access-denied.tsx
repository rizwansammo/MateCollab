import { Button, EmptyState } from '@matecollab/ui';
import { Lock } from 'lucide-react';
import type { Route } from 'next';
import Link from 'next/link';

/** Shown when the API answers 403 for an administration page (the API is the authority). */
export function AccessDenied({ home, what }: { home: string; what: string }) {
  return (
    <div className="rounded-md border border-border bg-card">
      <EmptyState
        icon={<Lock />}
        title="You don’t have access to this page"
        description={`Only organisation administrators can ${what}. Ask an Owner or Admin if you need access.`}
        action={
          <Button asChild>
            <Link href={home as Route}>Back to Home</Link>
          </Button>
        }
      />
    </div>
  );
}
