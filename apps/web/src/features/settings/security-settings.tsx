'use client';

import { zodResolver } from '@hookform/resolvers/zod';
import type { SessionItem, TOTPSetup } from '@matecollab/contracts';
import {
  Badge,
  Button,
  ConfirmDialog,
  Dialog,
  DialogContent,
  DialogFooter,
  Field,
  Input,
  Skeleton,
  toast,
} from '@matecollab/ui';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Copy, Download, KeyRound, Laptop, LogOut, ShieldCheck, ShieldOff } from 'lucide-react';
import { useState } from 'react';
import { useForm } from 'react-hook-form';
import { z } from 'zod';
import { DetailRow, Panel } from '@/components/panel';
import { FormNotice } from '@/features/auth/auth-frame';
import { PasswordInput } from '@/features/auth/form-helpers';
import { acceptSession, sessionKey, useSession } from '@/features/auth/session';
import { useStepUp } from '@/features/auth/step-up';
import { ApiError, api, errorMessage, unwrap } from '@/lib/api';
import { formatDateTime, formatRelative } from '@/lib/time';

export function SecuritySettings() {
  const stepUp = useStepUp();
  return (
    <>
      <PasswordPanel runStepUp={stepUp.run} />
      <TwoStepPanel runStepUp={stepUp.run} />
      <SessionsPanel />
      {stepUp.dialog}
    </>
  );
}

type StepUp = ReturnType<typeof useStepUp>['run'];

// ---- password ---------------------------------------------------------------------------------

const passwordSchema = z
  .object({
    password: z.string().min(12, 'Use at least 12 characters.').max(128, 'Use 128 characters or fewer.'),
    confirm: z.string(),
  })
  .refine((v) => v.password === v.confirm, { path: ['confirm'], message: 'The passwords don’t match.' });

function PasswordPanel({ runStepUp }: { runStepUp: StepUp }) {
  const [open, setOpen] = useState(false);
  const client = useQueryClient();
  const form = useForm<z.infer<typeof passwordSchema>>({
    resolver: zodResolver(passwordSchema),
    defaultValues: { password: '', confirm: '' },
  });
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const submit = form.handleSubmit(async ({ password }) => {
    setBusy(true);
    setError(null);
    try {
      const ok = await runStepUp(async () => {
        const res = await unwrap(api.PUT('/auth/password', { body: { new_password: password } }));
        if (res.session) acceptSession(client, res.session);
      });
      if (ok) {
        setOpen(false);
        form.reset();
        toast.success('Password changed. Your other sessions were signed out.');
        void client.invalidateQueries({ queryKey: ['auth', 'sessions'] });
      }
    } catch (err) {
      const pw = err instanceof ApiError ? err.field('new_password') : undefined;
      if (pw) form.setError('password', { message: pw });
      else setError(errorMessage(err));
    } finally {
      setBusy(false);
    }
  });

  return (
    <Panel
      id="password"
      title="Password"
      description="Changing your password signs you out everywhere else."
      action={
        <Button onClick={() => setOpen(true)}>
          <KeyRound aria-hidden /> Change password
        </Button>
      }
    >
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent
          title="Change password"
          description="Use at least 12 characters. Avoid common passwords."
        >
          <form onSubmit={(e) => void submit(e)} noValidate>
            {error ? <FormNotice>{error}</FormNotice> : null}
            <Field label="New password" error={form.formState.errors.password?.message}>
              <PasswordInput autoComplete="new-password" autoFocus {...form.register('password')} />
            </Field>
            <Field label="Confirm new password" error={form.formState.errors.confirm?.message}>
              <PasswordInput autoComplete="new-password" {...form.register('confirm')} />
            </Field>
            <DialogFooter>
              <Button variant="secondary" onClick={() => setOpen(false)}>
                Cancel
              </Button>
              <Button type="submit" variant="primary" loading={busy}>
                Change password
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </Panel>
  );
}

// ---- two-step verification --------------------------------------------------------------------

function TwoStepPanel({ runStepUp }: { runStepUp: StepUp }) {
  const client = useQueryClient();
  const mfa = useQuery({ queryKey: ['auth', 'mfa'], queryFn: () => unwrap(api.GET('/auth/mfa')) });
  const [enrolling, setEnrolling] = useState(false);
  const [confirmDisable, setConfirmDisable] = useState(false);
  const [codes, setCodes] = useState<string[] | null>(null);

  const refresh = () => {
    void client.invalidateQueries({ queryKey: ['auth', 'mfa'] });
    void client.invalidateQueries({ queryKey: sessionKey });
    void client.invalidateQueries({ queryKey: ['org'] }); // organisation 2FA policies re-evaluate
  };

  const disable = async () => {
    try {
      if (await runStepUp(() => unwrap(api.POST('/auth/mfa/totp/disable')))) {
        toast.success('Two-step verification is off.');
        refresh();
      }
    } catch (err) {
      toast.error(errorMessage(err));
    }
  };
  const regenerate = async () => {
    try {
      await runStepUp(async () =>
        setCodes((await unwrap(api.POST('/auth/mfa/recovery-codes'))).recovery_codes),
      );
      refresh();
    } catch (err) {
      toast.error(errorMessage(err));
    }
  };

  const enabled = mfa.data?.totp_enabled;
  return (
    <Panel
      id="two-step"
      title={
        <span className="flex items-center gap-2.5">
          Two-step verification
          {mfa.data ? <Badge tone={enabled ? 'success' : 'neutral'}>{enabled ? 'On' : 'Off'}</Badge> : null}
        </span>
      }
      description="Sign in with your password and a code from an authenticator app."
      action={
        mfa.isPending ? null : enabled ? (
          <>
            <Button onClick={() => void regenerate()}>New recovery codes</Button>
            <Button variant="ghost" onClick={() => setConfirmDisable(true)}>
              <ShieldOff aria-hidden /> Turn off
            </Button>
          </>
        ) : (
          <Button variant="primary" onClick={() => setEnrolling(true)}>
            <ShieldCheck aria-hidden /> Turn on
          </Button>
        )
      }
    >
      {mfa.isPending ? (
        <Skeleton className="h-5 w-60" />
      ) : mfa.isError ? (
        <p className="text-13 text-danger">{errorMessage(mfa.error)}</p>
      ) : enabled ? (
        <DetailRow label="Recovery codes left">{mfa.data.recovery_codes_remaining} of 10</DetailRow>
      ) : (
        <p className="text-13 text-fg-muted">
          Use an authenticator app such as 1Password, Microsoft Authenticator or Google Authenticator.
        </p>
      )}
      {enrolling ? (
        <EnrolDialog
          onClose={() => setEnrolling(false)}
          onEnabled={(c) => {
            setEnrolling(false);
            setCodes(c);
            refresh();
          }}
        />
      ) : null}
      {codes ? <RecoveryCodesDialog codes={codes} onClose={() => setCodes(null)} /> : null}
      <ConfirmDialog
        open={confirmDisable}
        onOpenChange={setConfirmDisable}
        title="Turn off two-step verification?"
        description="Your account will be protected by your password only. Organisations that require two-step verification will block access until you turn it on again."
        confirmLabel="Turn off"
        onConfirm={() => void disable()}
      />
    </Panel>
  );
}

function EnrolDialog({ onClose, onEnabled }: { onClose: () => void; onEnabled: (codes: string[]) => void }) {
  const setup = useQuery<TOTPSetup>({
    queryKey: ['auth', 'mfa', 'setup'],
    queryFn: () => unwrap(api.POST('/auth/mfa/totp/setup')),
    staleTime: Infinity,
    gcTime: 0,
  });
  const [code, setCode] = useState('');
  const enable = useMutation({
    mutationFn: () => unwrap(api.POST('/auth/mfa/totp/enable', { body: { code: code.replace(/\s+/g, '') } })),
    onSuccess: (res) => onEnabled(res.recovery_codes),
  });
  const codeError =
    enable.error instanceof ApiError && enable.error.field('code')
      ? enable.error.field('code')
      : enable.error
        ? errorMessage(enable.error)
        : undefined;
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent
        title="Set up two-step verification"
        description="Scan the QR code with your authenticator app, then enter the 6-digit code it shows."
      >
        {setup.isPending ? (
          <Skeleton className="mx-auto size-44" />
        ) : setup.isError ? (
          <FormNotice>{errorMessage(setup.error)}</FormNotice>
        ) : (
          <form
            onSubmit={(e) => {
              e.preventDefault();
              enable.mutate();
            }}
          >
            <div className="flex items-center gap-5 max-phone:flex-col">
              <img
                src={setup.data.qr_code}
                alt="QR code for your authenticator app"
                width={176}
                height={176}
                className="size-44 shrink-0 rounded-md border border-border bg-white p-2"
              />
              <div className="min-w-0 text-13">
                <p className="text-fg-muted">Can’t scan? Enter this key in your app:</p>
                <code className="mt-2 block break-all rounded-md bg-muted px-2.5 py-2 font-mono text-12 tracking-wider">
                  {setup.data.secret.match(/.{1,4}/g)?.join(' ')}
                </code>
              </div>
            </div>
            <Field label="Authentication code" error={codeError} className="mt-5">
              <Input
                value={code}
                onChange={(e) => setCode(e.target.value)}
                autoComplete="one-time-code"
                inputMode="numeric"
                maxLength={7}
                autoFocus
                className="font-mono tracking-[0.3em]"
              />
            </Field>
            <DialogFooter>
              <Button variant="secondary" onClick={onClose}>
                Cancel
              </Button>
              <Button
                type="submit"
                variant="primary"
                loading={enable.isPending}
                disabled={code.trim().length < 6}
              >
                Turn on
              </Button>
            </DialogFooter>
          </form>
        )}
      </DialogContent>
    </Dialog>
  );
}

function RecoveryCodesDialog({ codes, onClose }: { codes: string[]; onClose: () => void }) {
  const text = `MateCollab recovery codes\nEach code works once.\n\n${codes.join('\n')}\n`;
  const download = () => {
    const url = URL.createObjectURL(new Blob([text], { type: 'text/plain' }));
    const a = Object.assign(document.createElement('a'), {
      href: url,
      download: 'matecollab-recovery-codes.txt',
    });
    a.click();
    URL.revokeObjectURL(url);
  };
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent
        title="Save your recovery codes"
        description="If you lose access to your authenticator app, each code lets you sign in once. They won’t be shown again."
        closable={false}
        onInteractOutside={(e) => e.preventDefault()}
      >
        <ul
          aria-label="Recovery codes"
          className="grid grid-cols-2 gap-2 rounded-md border border-border bg-surface p-4 font-mono text-13"
        >
          {codes.map((c) => (
            <li key={c}>{c}</li>
          ))}
        </ul>
        <DialogFooter>
          <Button
            onClick={() =>
              void navigator.clipboard
                .writeText(codes.join('\n'))
                .then(() => toast.success('Recovery codes copied'))
                .catch(() => toast.error('Copy failed. Download the codes instead.'))
            }
          >
            <Copy aria-hidden /> Copy
          </Button>
          <Button onClick={download}>
            <Download aria-hidden /> Download
          </Button>
          <Button variant="primary" onClick={onClose}>
            I’ve saved them
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

// ---- sessions ---------------------------------------------------------------------------------

function deviceName(ua: string | null): string {
  if (!ua) return 'Unknown device';
  const browser = /Edg\//.test(ua)
    ? 'Edge'
    : /Firefox\//.test(ua)
      ? 'Firefox'
      : /Chrome\//.test(ua)
        ? 'Chrome'
        : /Safari\//.test(ua)
          ? 'Safari'
          : 'Browser';
  const os = /Windows/.test(ua)
    ? 'Windows'
    : /Mac OS X|Macintosh/.test(ua)
      ? 'macOS'
      : /Android/.test(ua)
        ? 'Android'
        : /iPhone|iPad/.test(ua)
          ? 'iOS'
          : /Linux/.test(ua)
            ? 'Linux'
            : null;
  return os ? `${browser} on ${os}` : browser;
}

function SessionsPanel() {
  const client = useQueryClient();
  const sessions = useQuery({
    queryKey: ['auth', 'sessions'],
    queryFn: () => unwrap(api.GET('/auth/sessions')),
  });
  const refresh = () => void client.invalidateQueries({ queryKey: ['auth', 'sessions'] });
  const revoke = useMutation({
    mutationFn: (id: string) =>
      unwrap(api.DELETE('/auth/sessions/{sessionId}', { params: { path: { sessionId: id } } })),
    onSuccess: () => {
      toast.success('Session signed out');
      refresh();
    },
    onError: (err) => toast.error(errorMessage(err)),
  });
  const revokeOthers = useMutation({
    mutationFn: () => unwrap(api.DELETE('/auth/sessions')),
    onSuccess: (res) => {
      toast.success(
        res.revoked === 1 ? '1 other session signed out' : `${res.revoked} other sessions signed out`,
      );
      refresh();
    },
    onError: (err) => toast.error(errorMessage(err)),
  });
  const others = sessions.data?.data.filter((s) => !s.current).length ?? 0;
  return (
    <Panel
      id="sessions"
      title="Where you’re signed in"
      description="Sign out of devices you don’t recognise or no longer use."
      action={
        others > 0 ? (
          <Button loading={revokeOthers.isPending} onClick={() => revokeOthers.mutate()}>
            <LogOut aria-hidden /> Sign out other sessions
          </Button>
        ) : null
      }
    >
      {sessions.isPending ? (
        <Skeleton className="h-12 w-full" />
      ) : sessions.isError ? (
        <p className="text-13 text-danger">{errorMessage(sessions.error)}</p>
      ) : (
        <ul aria-label="Active sessions" className="-my-1">
          {sessions.data.data.map((s) => (
            <SessionRow
              key={s.id}
              session={s}
              onRevoke={() => revoke.mutate(s.id)}
              busy={revoke.isPending && revoke.variables === s.id}
            />
          ))}
        </ul>
      )}
    </Panel>
  );
}

function SessionRow({
  session,
  onRevoke,
  busy,
}: {
  session: SessionItem;
  onRevoke: () => void;
  busy: boolean;
}) {
  const { session: current } = useSession();
  return (
    <li className="flex items-center gap-4 border-b border-border py-3 last:border-b-0 max-phone:flex-wrap">
      <span
        aria-hidden
        className="flex size-9 shrink-0 items-center justify-center rounded-md bg-muted text-fg-muted [&_svg]:size-4.5"
      >
        <Laptop />
      </span>
      <div className="min-w-0 flex-1 text-13">
        <p className="flex items-center gap-2 font-strong">
          {deviceName(session.user_agent)}
          {session.current || session.id === current.id ? <Badge tone="brand">This device</Badge> : null}
        </p>
        <p className="text-12 text-fg-muted">
          {[
            session.ip,
            `Active ${formatRelative(session.last_seen_at)}`,
            `Signed in ${formatDateTime(session.created_at)}`,
          ]
            .filter(Boolean)
            .join(' · ')}
        </p>
      </div>
      {session.current ? null : (
        <Button size="sm" variant="ghost" loading={busy} onClick={onRevoke}>
          Sign out
        </Button>
      )}
    </li>
  );
}
