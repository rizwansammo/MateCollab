'use client';

import { Button, ConfirmDialog, toast } from '@matecollab/ui';
import { useQueryClient } from '@tanstack/react-query';
import { CalendarClock, Trash2, Undo2 } from 'lucide-react';
import { useState } from 'react';
import { Panel } from '@/components/panel';
import { FormNotice } from '@/features/auth/auth-frame';
import { sessionKey, useSession } from '@/features/auth/session';
import { useStepUp } from '@/features/auth/step-up';
import { ApiError, api, errorMessage, unwrap } from '@/lib/api';
import { formatDate } from '@/lib/time';

/** Account deletion: a request starts a 30-day grace period that can be cancelled. */
export function PrivacySettings() {
  const { user } = useSession();
  const client = useQueryClient();
  const stepUp = useStepUp();
  const [confirm, setConfirm] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const scheduled = user.deletion_scheduled_at;

  const run = async (action: () => Promise<unknown>, done: string) => {
    setBusy(true);
    setError(null);
    try {
      if (await stepUp.run(action)) {
        await client.invalidateQueries({ queryKey: sessionKey });
        toast.success(done);
      }
    } catch (err) {
      setError(errorMessage(err));
      if (!(err instanceof ApiError)) throw err;
    } finally {
      setBusy(false);
    }
  };

  return (
    <>
      <Panel
        id="deletion"
        title="Delete your account"
        description="Your account is deleted 30 days after you ask. Until then you can sign in and cancel."
      >
        {error ? <FormNotice>{error}</FormNotice> : null}
        {scheduled ? (
          <div className="flex items-center justify-between gap-4 max-phone:flex-col max-phone:items-start">
            <p className="flex items-center gap-2.5 text-13">
              <CalendarClock aria-hidden className="size-4 text-warning" />
              <span>
                Your account will be deleted on{' '}
                <strong className="font-strong">{formatDate(scheduled)}</strong>.
              </span>
            </p>
            <Button
              variant="primary"
              loading={busy}
              onClick={() => void run(() => unwrap(api.DELETE('/me/deletion')), 'Account deletion cancelled')}
            >
              <Undo2 aria-hidden /> Cancel deletion
            </Button>
          </div>
        ) : (
          <div className="flex items-center justify-between gap-4 max-phone:flex-col max-phone:items-start">
            <p className="text-13 text-fg-muted">
              Your name becomes “Deleted user”, and your sign-in details and email addresses are removed.
              Messages and files you shared stay in their organisations. If you own an organisation, transfer
              ownership first.
            </p>
            <Button variant="danger" loading={busy} onClick={() => setConfirm(true)}>
              <Trash2 aria-hidden /> Delete account
            </Button>
          </div>
        )}
      </Panel>
      <ConfirmDialog
        open={confirm}
        onOpenChange={setConfirm}
        title="Delete your account?"
        description="Your account will be deleted in 30 days. You can cancel any time before then by signing in."
        confirmLabel="Delete account"
        onConfirm={() =>
          void run(() => unwrap(api.POST('/me/deletion')), 'Your account is scheduled for deletion')
        }
      />
      {stepUp.dialog}
    </>
  );
}
