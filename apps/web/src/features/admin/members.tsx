'use client';

import type { Member, MembershipStatus, Role } from '@matecollab/contracts';
import {
  Avatar,
  Badge,
  Button,
  ConfirmDialog,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
  EmptyState,
  IconButton,
  Input,
  Select,
  Skeleton,
  toast,
} from '@matecollab/ui';
import { useInfiniteQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { MoreHorizontal, Search, UserRoundPlus, UsersRound } from 'lucide-react';
import type { Route } from 'next';
import Link from 'next/link';
import { useDeferredValue, useState } from 'react';
import { useSession } from '@/features/auth/session';
import { orgKey, orgPath, useOrg } from '@/features/org/org-context';
import { roleNames, statusNames } from '@/features/org/roles';
import { ApiError, api, errorMessage, unwrap } from '@/lib/api';
import { formatDate } from '@/lib/time';
import { AccessDenied } from './access-denied';

const roleFilter = [
  { value: 'all', label: 'All roles' },
  ...(['owner', 'admin', 'member', 'guest'] as const).map((r) => ({ value: r, label: roleNames[r] })),
];
const statusFilter = [
  { value: 'all', label: 'Any status' },
  ...(['active', 'suspended', 'deactivated'] as const).map((s) => ({ value: s, label: statusNames[s] })),
];

export function MembersDirectory() {
  const org = useOrg();
  const [q, setQ] = useState('');
  const [role, setRole] = useState('all');
  const [status, setStatus] = useState('all');
  const query = useDeferredValue(q.trim());
  const filters = {
    q: query || undefined,
    role: role === 'all' ? undefined : (role as Role),
    status: status === 'all' ? undefined : (status as MembershipStatus),
  };

  const members = useInfiniteQuery({
    queryKey: orgKey(org, 'members', filters),
    queryFn: ({ pageParam }) =>
      unwrap(
        api.GET('/orgs/{orgId}/members', {
          params: { path: { orgId: org.id }, query: { ...filters, cursor: pageParam, limit: 50 } },
        }),
      ),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (last) => last.page.next_cursor ?? undefined,
  });

  if (members.error instanceof ApiError && members.error.status === 403) {
    return <AccessDenied home={orgPath(org, '/home')} what="view the member directory" />;
  }

  const rows = members.data?.pages.flatMap((p) => p.data) ?? [];
  return (
    <>
      <div className="mb-4 flex flex-wrap items-center gap-3">
        <div className="relative min-w-60 flex-1">
          <Search
            aria-hidden
            className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-icon-muted"
          />
          <Input
            type="search"
            aria-label="Search members"
            placeholder="Search by name or email"
            value={q}
            onChange={(e) => setQ(e.target.value)}
            className="pl-9"
          />
        </div>
        <Select
          aria-label="Filter by role"
          options={roleFilter}
          value={role}
          onValueChange={setRole}
          className="w-40"
        />
        <Select
          aria-label="Filter by status"
          options={statusFilter}
          value={status}
          onValueChange={setStatus}
          className="w-40"
        />
      </div>
      <div className="rounded-md border border-border bg-card">
        {members.isPending ? (
          <div className="flex flex-col gap-3 p-5">
            {[0, 1, 2].map((i) => (
              <Skeleton key={i} className="h-10 w-full" />
            ))}
          </div>
        ) : members.isError ? (
          <EmptyState
            role="alert"
            title="Members couldn’t be loaded"
            description={errorMessage(members.error)}
            action={<Button onClick={() => void members.refetch()}>Try again</Button>}
          />
        ) : rows.length === 0 ? (
          <EmptyState
            icon={<UsersRound />}
            title={query || role !== 'all' || status !== 'all' ? 'No matching members' : 'No members yet'}
            description="Invite people to start working together."
            action={
              <Button asChild variant="primary">
                <Link href={orgPath(org, '/admin/invitations') as Route}>
                  <UserRoundPlus aria-hidden /> Invite people
                </Link>
              </Button>
            }
          />
        ) : (
          <ul aria-label="Members">
            {rows.map((m) => (
              <MemberRow key={m.user_id} member={m} />
            ))}
          </ul>
        )}
      </div>
      {members.hasNextPage ? (
        <div className="mt-4 flex justify-center">
          <Button loading={members.isFetchingNextPage} onClick={() => void members.fetchNextPage()}>
            Show more
          </Button>
        </div>
      ) : null}
    </>
  );
}

const statusTone = { active: 'success', suspended: 'warning', deactivated: 'neutral' } as const;

function MemberRow({ member }: { member: Member }) {
  const org = useOrg();
  const { user } = useSession();
  const client = useQueryClient();
  const [confirm, setConfirm] = useState<MembershipStatus | null>(null);
  const update = useMutation({
    mutationFn: (body: { role?: Role; status?: MembershipStatus }) =>
      unwrap(
        api.PATCH('/orgs/{orgId}/members/{userId}', {
          params: { path: { orgId: org.id, userId: member.user_id } },
          body,
        }),
      ),
    onSuccess: (m, body) => {
      toast.success(
        body.role
          ? `${m.display_name} is now ${roleNames[m.role]}`
          : `${m.display_name} is ${statusNames[m.status].toLowerCase()}`,
      );
      void client.invalidateQueries({ queryKey: orgKey(org, 'members') });
    },
    onError: (err) => toast.error(errorMessage(err)),
  });

  const self = member.user_id === user.id;
  // Owners and Admins manage others; the Owner is changed only by ownership transfer, and only
  // the Owner manages Admins. The API enforces the same rules.
  const canManage =
    !self &&
    member.role !== 'owner' &&
    (org.role === 'owner' || (org.role === 'admin' && member.role !== 'admin'));
  const assignable: Role[] = org.role === 'owner' ? ['admin', 'member', 'guest'] : ['member', 'guest'];

  return (
    <li className="flex items-center gap-3.5 border-b border-border px-5 py-3.5 last:border-b-0 max-phone:flex-wrap max-phone:px-4">
      <Avatar name={member.display_name} size={36} decorative />
      <div className="min-w-0 flex-1">
        <p className="truncate text-13 font-strong">
          {member.display_name}
          {self ? <span className="font-book text-fg-muted"> (you)</span> : null}
        </p>
        <p className="truncate text-12 text-fg-muted">{member.email ?? '—'}</p>
      </div>
      <div className="flex items-center gap-2">
        <Badge tone={member.role === 'owner' || member.role === 'admin' ? 'brand' : 'neutral'}>
          {roleNames[member.role]}
        </Badge>
        <Badge tone={statusTone[member.status]}>{statusNames[member.status]}</Badge>
      </div>
      <span className="w-28 text-right text-12 text-fg-muted max-tablet:hidden">
        Joined {formatDate(member.joined_at)}
      </span>
      {canManage ? (
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <IconButton label={`Manage ${member.display_name}`} disabled={update.isPending}>
              <MoreHorizontal aria-hidden />
            </IconButton>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" className="w-52">
            <DropdownMenuLabel>Change role</DropdownMenuLabel>
            {assignable.map((r) => (
              <DropdownMenuItem
                key={r}
                disabled={r === member.role}
                onSelect={() => update.mutate({ role: r })}
              >
                {roleNames[r]}
              </DropdownMenuItem>
            ))}
            <DropdownMenuSeparator />
            {member.status === 'active' ? (
              <>
                <DropdownMenuItem onSelect={() => setConfirm('suspended')}>Suspend access</DropdownMenuItem>
                <DropdownMenuItem className="text-danger" onSelect={() => setConfirm('deactivated')}>
                  Deactivate
                </DropdownMenuItem>
              </>
            ) : (
              <DropdownMenuItem onSelect={() => update.mutate({ status: 'active' })}>
                Reactivate
              </DropdownMenuItem>
            )}
          </DropdownMenuContent>
        </DropdownMenu>
      ) : (
        <span className="w-7.5" aria-hidden />
      )}
      <ConfirmDialog
        open={confirm !== null}
        onOpenChange={(o) => !o && setConfirm(null)}
        title={
          confirm === 'deactivated' ? `Deactivate ${member.display_name}?` : `Suspend ${member.display_name}?`
        }
        description={
          confirm === 'deactivated'
            ? `They lose access to ${org.name} straight away. Their messages and files stay. You can reactivate them later.`
            : `They can’t open ${org.name} until you reactivate them.`
        }
        confirmLabel={confirm === 'deactivated' ? 'Deactivate' : 'Suspend'}
        onConfirm={() => confirm && update.mutate({ status: confirm })}
      />
    </li>
  );
}
