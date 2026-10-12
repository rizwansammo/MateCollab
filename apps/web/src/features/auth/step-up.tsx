'use client';

import { Button, Dialog, DialogContent, DialogFooter, Field, Input } from '@matecollab/ui';
import { useQueryClient } from '@tanstack/react-query';
import { useCallback, useRef, useState, type FormEvent } from 'react';
import { ApiError, api, errorMessage, unwrap } from '@/lib/api';
import { FormNotice } from './auth-frame';
import { PasswordInput } from './form-helpers';
import { sessionKey, useSession } from './session';

/**
 * Sensitive actions need a password check within the last 10 minutes (403
 * reauthentication_required). `run(action)` performs the action and, when the server asks for
 * it, collects the password (and authenticator code), confirms it and retries once.
 */
export function useStepUp() {
  const session = useSession();
  const client = useQueryClient();
  const [open, setOpen] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const pending = useRef<{ action: () => Promise<unknown>; resolve: (ok: boolean) => void } | null>(null);

  const run = useCallback(async (action: () => Promise<unknown>): Promise<boolean> => {
    try {
      await action();
      return true;
    } catch (err) {
      if (!(err instanceof ApiError) || err.code !== 'reauthentication_required') throw err;
      return new Promise<boolean>((resolve) => {
        pending.current = { action, resolve };
        setError(null);
        setOpen(true);
      });
    }
  }, []);

  const close = (ok: boolean) => {
    setOpen(false);
    pending.current?.resolve(ok);
    pending.current = null;
  };

  const confirm = async (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    const data = new FormData(e.currentTarget);
    const password = String(data.get('password') ?? '');
    const code = String(data.get('code') ?? '').replace(/\s+/g, '');
    setBusy(true);
    setError(null);
    try {
      await unwrap(api.POST('/auth/reauthenticate', { body: code ? { password, code } : { password } }));
      await client.invalidateQueries({ queryKey: sessionKey });
      await pending.current?.action();
      close(true);
    } catch (err) {
      setError(
        err instanceof ApiError && err.fields.length
          ? err.fields.map((f) => f.message).join(' ')
          : errorMessage(err),
      );
    } finally {
      setBusy(false);
    }
  };

  const dialog = (
    <Dialog open={open} onOpenChange={(o) => !o && close(false)}>
      <DialogContent title="Confirm it’s you" description="Enter your password to continue with this change.">
        <form onSubmit={(e) => void confirm(e)}>
          {error ? <FormNotice>{error}</FormNotice> : null}
          <Field label="Password">
            <PasswordInput name="password" autoComplete="current-password" required autoFocus />
          </Field>
          {session.mfa.totp_enabled ? (
            <Field label="Authentication code" description="From your authenticator app.">
              <Input name="code" autoComplete="one-time-code" inputMode="numeric" maxLength={7} required />
            </Field>
          ) : null}
          <DialogFooter>
            <Button variant="secondary" onClick={() => close(false)}>
              Cancel
            </Button>
            <Button type="submit" variant="primary" loading={busy}>
              Confirm
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );

  return { run, dialog };
}
