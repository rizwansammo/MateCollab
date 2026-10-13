'use client';

import type { EmailAddress } from '@matecollab/contracts';
import {
  Badge,
  Button,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
  Field,
  IconButton,
  Input,
  Skeleton,
  toast,
} from '@matecollab/ui';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Mail, MoreHorizontal, Plus } from 'lucide-react';
import { useState, type FormEvent } from 'react';
import { Panel } from '@/components/panel';
import { EMAIL } from '@/features/auth/form-helpers';
import { sessionKey, useSession } from '@/features/auth/session';
import { useStepUp } from '@/features/auth/step-up';
import { ApiError, api, errorMessage, unwrap } from '@/lib/api';
import { MembershipProfilePanel } from './membership-profile';

export function ProfileSettings() {
  const stepUp = useStepUp();
  return (
    <>
      <NamePanel />
      <MembershipProfilePanel />
      <EmailsPanel runStepUp={stepUp.run} />
      {stepUp.dialog}
    </>
  );
}

function NamePanel() {
  const { user } = useSession();
  const client = useQueryClient();
  const [name, setName] = useState(user.display_name);
  const save = useMutation({
    mutationFn: () => unwrap(api.PATCH('/me', { body: { display_name: name } })),
    onSuccess: (u) => {
      setName(u.display_name);
      toast.success('Profile updated');
      void client.invalidateQueries({ queryKey: sessionKey });
    },
  });
  const error =
    save.error instanceof ApiError
      ? (save.error.field('display_name') ?? errorMessage(save.error))
      : undefined;
  return (
    <Panel id="profile" title="Your name" description="Shown to people in every organisation you belong to.">
      <form
        className="flex max-w-120 items-start gap-3 max-phone:flex-col max-phone:items-stretch"
        onSubmit={(e) => {
          e.preventDefault();
          save.mutate();
        }}
      >
        <Field label="Display name" hideLabel error={error} className="mb-0 flex-1">
          <Input value={name} onChange={(e) => setName(e.target.value)} autoComplete="name" maxLength={80} />
        </Field>
        <Button
          type="submit"
          variant="primary"
          loading={save.isPending}
          disabled={name.trim() === user.display_name}
        >
          Save
        </Button>
      </form>
    </Panel>
  );
}

function EmailsPanel({ runStepUp }: { runStepUp: ReturnType<typeof useStepUp>['run'] }) {
  const client = useQueryClient();
  const emails = useQuery({ queryKey: ['me', 'emails'], queryFn: () => unwrap(api.GET('/me/emails')) });
  const refresh = () => {
    void client.invalidateQueries({ queryKey: ['me', 'emails'] });
    void client.invalidateQueries({ queryKey: sessionKey });
  };
  const [adding, setAdding] = useState(false);
  const [address, setAddress] = useState('');
  const [addError, setAddError] = useState<string | null>(null);

  const add = useMutation({
    mutationFn: (email: string) => unwrap(api.POST('/me/emails', { body: { email } })),
    onSuccess: (e) => {
      toast.success(`We sent a confirmation link to ${e.email}`);
      setAddress('');
      setAdding(false);
      refresh();
    },
    onError: (err) =>
      setAddError(err instanceof ApiError ? (err.field('email') ?? errorMessage(err)) : errorMessage(err)),
  });
  const submit = (e: FormEvent) => {
    e.preventDefault();
    setAddError(null);
    if (!EMAIL.test(address.trim())) return setAddError('Enter a valid email address.');
    add.mutate(address.trim());
  };

  return (
    <Panel
      id="emails"
      title="Email addresses"
      description="Your primary address is used to sign in and for account emails."
      action={
        adding ? null : (
          <Button onClick={() => setAdding(true)}>
            <Plus aria-hidden /> Add address
          </Button>
        )
      }
    >
      {emails.isPending ? (
        <Skeleton className="h-10 w-full" />
      ) : emails.isError ? (
        <p className="text-13 text-danger">{errorMessage(emails.error)}</p>
      ) : (
        <ul aria-label="Email addresses" className="-my-1">
          {emails.data.data.map((e) => (
            <EmailRow key={e.id} email={e} runStepUp={runStepUp} onChanged={refresh} />
          ))}
        </ul>
      )}
      {adding ? (
        <form
          onSubmit={submit}
          className="mt-4 flex max-w-120 items-start gap-3 max-phone:flex-col max-phone:items-stretch"
        >
          <Field label="New email address" hideLabel error={addError ?? undefined} className="mb-0 flex-1">
            <Input
              type="email"
              value={address}
              onChange={(e) => setAddress(e.target.value)}
              placeholder="name@example.com"
              autoFocus
            />
          </Field>
          <Button type="submit" variant="primary" loading={add.isPending}>
            Add
          </Button>
          <Button variant="ghost" onClick={() => setAdding(false)}>
            Cancel
          </Button>
        </form>
      ) : null}
    </Panel>
  );
}

function EmailRow({
  email,
  runStepUp,
  onChanged,
}: {
  email: EmailAddress;
  runStepUp: ReturnType<typeof useStepUp>['run'];
  onChanged: () => void;
}) {
  const act = async (action: () => Promise<unknown>, done: string, sensitive = false) => {
    try {
      const ok = sensitive ? await runStepUp(action) : (await action(), true);
      if (ok) {
        toast.success(done);
        onChanged();
      }
    } catch (err) {
      toast.error(errorMessage(err));
    }
  };
  const path = { params: { path: { emailId: email.id } } };
  return (
    <li className="flex items-center gap-3 border-b border-border py-3 last:border-b-0">
      <Mail aria-hidden className="size-4 shrink-0 text-icon-muted" />
      <span className="min-w-0 flex-1 truncate text-13">{email.email}</span>
      {email.is_primary ? <Badge tone="brand">Primary</Badge> : null}
      {email.verified_at ? null : <Badge tone="warning">Unconfirmed</Badge>}
      {email.is_primary && email.verified_at ? null : (
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <IconButton label={`Actions for ${email.email}`}>
              <MoreHorizontal aria-hidden />
            </IconButton>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            {email.verified_at ? null : (
              <DropdownMenuItem
                onSelect={() =>
                  void act(
                    () => unwrap(api.POST('/auth/email/resend', { body: { email_id: email.id } })),
                    'Confirmation email sent',
                  )
                }
              >
                Resend confirmation
              </DropdownMenuItem>
            )}
            {!email.is_primary && email.verified_at ? (
              <DropdownMenuItem
                onSelect={() =>
                  void act(
                    () => unwrap(api.PUT('/me/emails/{emailId}/primary', path)),
                    'Primary address changed',
                    true,
                  )
                }
              >
                Make primary
              </DropdownMenuItem>
            ) : null}
            {!email.is_primary ? (
              <DropdownMenuItem
                className="text-danger"
                onSelect={() =>
                  void act(() => unwrap(api.DELETE('/me/emails/{emailId}', path)), 'Address removed')
                }
              >
                Remove
              </DropdownMenuItem>
            ) : null}
          </DropdownMenuContent>
        </DropdownMenu>
      )}
    </li>
  );
}
