'use client';

import type { Invitation, InvitableRole } from '@matecollab/contracts';
import {
  Badge,
  Button,
  ConfirmDialog,
  Checkbox,
  Dialog,
  DialogContent,
  DialogFooter,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
  EmptyState,
  Field,
  IconButton,
  Input,
  Select,
  Skeleton,
  Tabs,
  TabsList,
  TabsTrigger,
  toast,
} from '@matecollab/ui';
import { useInfiniteQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { MailPlus, MoreHorizontal, UserRoundPlus } from 'lucide-react';
import { useState, type FormEvent } from 'react';
import { FormNotice } from '@/features/auth/auth-frame';
import { EMAIL } from '@/features/auth/form-helpers';
import { orgKey, orgPath, useOrg } from '@/features/org/org-context';
import { roleDescriptions, roleNames } from '@/features/org/roles';
import { ApiError, api, errorMessage, unwrap } from '@/lib/api';
import { formatDate, formatRelative } from '@/lib/time';
import { useSpaces } from '@/features/spaces/data';
import { AccessDenied } from './access-denied';

export function InvitationsManager() {
  const org = useOrg();
  const [view, setView] = useState<'pending' | 'all'>('pending');
  const [inviting, setInviting] = useState(false);
  const invitations = useInfiniteQuery({
    queryKey: orgKey(org, 'invitations', view),
    queryFn: ({ pageParam }) =>
      unwrap(
        api.GET('/orgs/{orgId}/invitations', {
          params: {
            path: { orgId: org.id },
            query: { status: view === 'pending' ? 'pending' : undefined, cursor: pageParam, limit: 50 },
          },
        }),
      ),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (last) => last.page.next_cursor ?? undefined,
  });

  if (invitations.error instanceof ApiError && invitations.error.status === 403) {
    return <AccessDenied home={orgPath(org, '/home')} what="manage invitations" />;
  }
  const rows = invitations.data?.pages.flatMap((p) => p.data) ?? [];
  return (
    <>
      <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
        <Tabs value={view} onValueChange={(v) => setView(v as 'pending' | 'all')}>
          <TabsList aria-label="Invitations">
            <TabsTrigger value="pending">Pending</TabsTrigger>
            <TabsTrigger value="all">All invitations</TabsTrigger>
          </TabsList>
        </Tabs>
        <Button variant="primary" onClick={() => setInviting(true)}>
          <UserRoundPlus aria-hidden /> Invite people
        </Button>
      </div>
      <div className="rounded-md border border-border bg-card">
        {invitations.isPending ? (
          <div className="flex flex-col gap-3 p-5">
            {[0, 1].map((i) => (
              <Skeleton key={i} className="h-10 w-full" />
            ))}
          </div>
        ) : invitations.isError ? (
          <EmptyState
            role="alert"
            title="Invitations couldn’t be loaded"
            description={errorMessage(invitations.error)}
            action={<Button onClick={() => void invitations.refetch()}>Try again</Button>}
          />
        ) : rows.length === 0 ? (
          <EmptyState
            icon={<MailPlus />}
            title={view === 'pending' ? 'No pending invitations' : 'No invitations yet'}
            description="Invite colleagues by email. Invitations are valid for 7 days."
          />
        ) : (
          <ul aria-label="Invitations">
            {rows.map((inv) => (
              <InvitationRow key={inv.id} invitation={inv} />
            ))}
          </ul>
        )}
      </div>
      {invitations.hasNextPage ? (
        <div className="mt-4 flex justify-center">
          <Button loading={invitations.isFetchingNextPage} onClick={() => void invitations.fetchNextPage()}>
            Show more
          </Button>
        </div>
      ) : null}
      {inviting ? <InviteDialog onClose={() => setInviting(false)} /> : null}
    </>
  );
}

const statusTone = { pending: 'brand', accepted: 'success', revoked: 'neutral', expired: 'warning' } as const;
const statusLabel = {
  pending: 'Pending',
  accepted: 'Accepted',
  revoked: 'Revoked',
  expired: 'Expired',
} as const;

function InvitationRow({ invitation }: { invitation: Invitation }) {
  const org = useOrg();
  const client = useQueryClient();
  const [confirmRevoke, setConfirmRevoke] = useState(false);
  const path = { params: { path: { orgId: org.id, invitationId: invitation.id } } };
  const refresh = () => void client.invalidateQueries({ queryKey: orgKey(org, 'invitations') });
  const resend = useMutation({
    mutationFn: () => unwrap(api.POST('/orgs/{orgId}/invitations/{invitationId}/resend', path)),
    onSuccess: () => {
      toast.success(`Invitation sent again to ${invitation.email}`);
      refresh();
    },
    onError: (err) => toast.error(errorMessage(err)),
  });
  const revoke = useMutation({
    mutationFn: () => unwrap(api.DELETE('/orgs/{orgId}/invitations/{invitationId}', path)),
    onSuccess: () => {
      toast.success(`Invitation for ${invitation.email} revoked`);
      refresh();
    },
    onError: (err) => toast.error(errorMessage(err)),
  });
  const open = invitation.status === 'pending' || invitation.status === 'expired';
  const detail =
    invitation.status === 'pending'
      ? `Expires ${formatRelative(invitation.expires_at)}`
      : invitation.status === 'accepted' && invitation.accepted_at
        ? `Accepted ${formatDate(invitation.accepted_at)}`
        : invitation.status === 'expired'
          ? `Expired ${formatDate(invitation.expires_at)}`
          : `Sent ${formatDate(invitation.last_sent_at)}`;
  return (
    <li className="flex items-center gap-3.5 border-b border-border px-5 py-3.5 last:border-b-0 max-phone:flex-wrap max-phone:px-4">
      <div className="min-w-0 flex-1">
        <p className="truncate text-13 font-strong">{invitation.email}</p>
        <p className="truncate text-12 text-fg-muted">
          Invited by {invitation.invited_by.display_name || 'a former member'} · {detail}
        </p>
      </div>
      <Badge>{roleNames[invitation.role]}</Badge>
      <Badge tone={statusTone[invitation.status]}>{statusLabel[invitation.status]}</Badge>
      {open ? (
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <IconButton label={`Manage invitation for ${invitation.email}`}>
              <MoreHorizontal aria-hidden />
            </IconButton>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuItem onSelect={() => resend.mutate()}>Resend invitation</DropdownMenuItem>
            {invitation.status === 'pending' ? (
              <DropdownMenuItem className="text-danger" onSelect={() => setConfirmRevoke(true)}>
                Revoke
              </DropdownMenuItem>
            ) : null}
          </DropdownMenuContent>
        </DropdownMenu>
      ) : (
        <span className="w-7.5" aria-hidden />
      )}
      <ConfirmDialog
        open={confirmRevoke}
        onOpenChange={setConfirmRevoke}
        title="Revoke this invitation?"
        description={`The link sent to ${invitation.email} will stop working.`}
        confirmLabel="Revoke"
        onConfirm={() => revoke.mutate()}
      />
    </li>
  );
}

/** A short, stable digest of the form values, so editing the form after an error uses a new key. */
function digest(value: string): string {
  let h = 5381;
  for (let i = 0; i < value.length; i++) h = ((h << 5) + h + value.charCodeAt(i)) >>> 0;
  return h.toString(36);
}

function InviteDialog({ onClose }: { onClose: () => void }) {
  const org = useOrg();
  const client = useQueryClient();
  // Roles this person could ever grant; organisation settings may narrow it (the API decides).
  const roles: InvitableRole[] =
    org.role === 'owner'
      ? ['member', 'guest', 'admin']
      : org.role === 'admin'
        ? ['member', 'guest']
        : ['member'];
  const [email, setEmail] = useState('');
  const [role, setRole] = useState<InvitableRole>('member');
  const [spaceIDs, setSpaceIDs] = useState<string[]>([]);
  const spaces = useSpaces();
  // One key per dialog: a double submit or a retry after a network error cannot invite twice.
  const [idempotencyKey] = useState(() => crypto.randomUUID());
  const [fieldError, setFieldError] = useState<string | null>(null);
  const invite = useMutation({
    mutationFn: () =>
      unwrap(
        api.POST('/orgs/{orgId}/invitations', {
          params: {
            path: { orgId: org.id },
            header: {
              'Idempotency-Key': `${idempotencyKey}-${digest(`${email.trim()}|${role}|${[...spaceIDs].sort().join(',')}`)}`,
            },
          },
          body: { email: email.trim(), role, space_ids: role === 'guest' ? spaceIDs : undefined },
        }),
      ),
    onSuccess: (inv) => {
      toast.success(`Invitation sent to ${inv.email}`);
      void client.invalidateQueries({ queryKey: orgKey(org, 'invitations') });
      onClose();
    },
  });
  const submit = (e: FormEvent) => {
    e.preventDefault();
    setFieldError(null);
    if (!EMAIL.test(email.trim())) return setFieldError('Enter a valid email address.');
    invite.mutate();
  };
  const apiError = invite.error instanceof ApiError ? invite.error : null;
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent
        title={`Invite people to ${org.name}`}
        description="They’ll get an email with a link that works for 7 days."
      >
        <form onSubmit={submit} noValidate>
          {apiError && !apiError.field('email') && !apiError.field('role') ? (
            <FormNotice>{errorMessage(apiError)}</FormNotice>
          ) : invite.error && !apiError ? (
            <FormNotice>{errorMessage(invite.error)}</FormNotice>
          ) : null}
          <Field label="Email address" error={fieldError ?? apiError?.field('email')}>
            <Input
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              autoFocus
              placeholder="name@example.com"
            />
          </Field>
          <Field label="Role" description={roleDescriptions[role]} error={apiError?.field('role')}>
            <Select
              options={roles.map((r) => ({ value: r, label: roleNames[r] }))}
              value={role}
              onValueChange={(v) => setRole(v as InvitableRole)}
            />
          </Field>
          {role === 'guest' && (
            <fieldset className="mt-4 rounded-md border border-border p-4">
              <legend className="px-1 text-13 font-strong">Granted Spaces</legend>
              <p className="mb-3 text-12 text-fg-muted">Guests can access only the Spaces you select.</p>
              {spaces.isPending ? (
                <Skeleton className="h-10 w-full" />
              ) : spaces.data?.length ? (
                <div className="max-h-48 space-y-2 overflow-y-auto">
                  {spaces.data.map((space) => (
                    <label key={space.id} className="flex cursor-pointer items-center gap-3 text-13">
                      <Checkbox
                        checked={spaceIDs.includes(space.id)}
                        onCheckedChange={(checked) =>
                          setSpaceIDs((old) =>
                            checked ? [...old, space.id] : old.filter((id) => id !== space.id),
                          )
                        }
                      />
                      <span>{space.name}</span>
                    </label>
                  ))}
                </div>
              ) : (
                <p className="text-12 text-fg-muted">Create a Space before granting guest access.</p>
              )}
              {apiError?.field('space_ids') && <FormNotice>{apiError.field('space_ids')}</FormNotice>}
            </fieldset>
          )}
          <DialogFooter>
            <Button variant="secondary" onClick={onClose}>
              Cancel
            </Button>
            <Button type="submit" variant="primary" loading={invite.isPending}>
              Send invitation
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
