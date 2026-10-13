'use client';

import type { components } from '@matecollab/contracts';
import { Badge, Button, EmptyState, Field, Input, Select, Skeleton, toast } from '@matecollab/ui';
import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Download, FileClock, ScrollText } from 'lucide-react';
import { useState } from 'react';
import { Panel } from '@/components/panel';
import { orgKey, orgPath, useOrg } from '@/features/org/org-context';
import { ApiError, api, errorMessage, unwrap } from '@/lib/api';
import { formatDateTime } from '@/lib/time';
import { AccessDenied } from './access-denied';

type Entry = components['schemas']['AuditLogEntry'];
type Export = components['schemas']['AuditExport'];

const groups = [
  { value: 'all', label: 'All events' },
  { value: 'org', label: 'Organisation' },
  { value: 'member', label: 'Members' },
  { value: 'invitation', label: 'Invitations' },
  { value: 'audit', label: 'Audit exports' },
];

interface Filters {
  action?: string;
  from?: string;
  to?: string;
}

/** Turns a date input (YYYY-MM-DD, the viewer's day) into the RFC 3339 bound the API expects. */
function dayBound(day: string, end: boolean): string | undefined {
  if (!day) return undefined;
  const d = new Date(`${day}T00:00:00`);
  if (end) d.setDate(d.getDate() + 1);
  return d.toISOString().replace(/\.\d{3}Z$/, 'Z');
}

export function AuditLog() {
  const org = useOrg();
  const [group, setGroup] = useState('all');
  const [fromDay, setFromDay] = useState('');
  const [toDay, setToDay] = useState('');
  const filters: Filters = {
    action: group === 'all' ? undefined : group,
    from: dayBound(fromDay, false),
    to: dayBound(toDay, true),
  };
  const log = useInfiniteQuery({
    queryKey: orgKey(org, 'audit', filters),
    queryFn: ({ pageParam }) =>
      unwrap(
        api.GET('/orgs/{orgId}/audit-logs', {
          params: { path: { orgId: org.id }, query: { ...filters, cursor: pageParam, limit: 50 } },
        }),
      ),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (last) => last.page.next_cursor ?? undefined,
  });
  if (log.error instanceof ApiError && log.error.status === 403) {
    return <AccessDenied home={orgPath(org, '/home')} what="view the audit log" />;
  }
  const rows = log.data?.pages.flatMap((p) => p.data) ?? [];
  return (
    <>
      <div className="mb-4 flex flex-wrap items-end gap-3">
        <Field label="Events" className="mb-0 w-48">
          <Select options={groups} value={group} onValueChange={setGroup} />
        </Field>
        <Field label="From" className="mb-0 w-44">
          <Input type="date" value={fromDay} onChange={(e) => setFromDay(e.target.value)} />
        </Field>
        <Field label="To" className="mb-0 w-44">
          <Input type="date" value={toDay} onChange={(e) => setToDay(e.target.value)} />
        </Field>
      </div>
      <div className="rounded-md border border-border bg-card">
        {log.isPending ? (
          <div className="flex flex-col gap-3 p-5">
            {[0, 1, 2].map((i) => (
              <Skeleton key={i} className="h-9 w-full" />
            ))}
          </div>
        ) : log.isError ? (
          <EmptyState
            role="alert"
            title="The audit log couldn’t be loaded"
            description={errorMessage(log.error)}
          />
        ) : rows.length === 0 ? (
          <EmptyState icon={<ScrollText />} title="No events" description="Nothing matches these filters." />
        ) : (
          <ul aria-label="Audit events">
            {rows.map((e) => (
              <AuditRow key={e.id} entry={e} />
            ))}
          </ul>
        )}
      </div>
      {log.hasNextPage ? (
        <div className="mt-4 flex justify-center">
          <Button loading={log.isFetchingNextPage} onClick={() => void log.fetchNextPage()}>
            Show more
          </Button>
        </div>
      ) : null}
      <ExportsPanel filters={filters} />
    </>
  );
}

function describe(e: Entry): string {
  const md = e.metadata as Record<string, unknown>;
  switch (e.action) {
    case 'member.role_changed':
      return `Role changed from ${String(md.from)} to ${String(md.to)}`;
    case 'member.status_changed':
      return `Access changed from ${String(md.from)} to ${String(md.to)}`;
    case 'invitation.created':
      return `Invitation sent (${String(md.role)})`;
    case 'org.ownership_transferred':
      return 'Ownership transferred';
    default:
      return e.action;
  }
}

function AuditRow({ entry }: { entry: Entry }) {
  const actor = entry.actor.display_name ?? (entry.actor.type === 'system' ? 'System' : 'Former member');
  return (
    <li className="flex items-center gap-4 border-b border-border px-5 py-3 text-13 last:border-b-0 max-phone:flex-wrap">
      <span className="w-40 shrink-0 text-12 text-fg-muted">{formatDateTime(entry.created_at)}</span>
      <span className="min-w-0 flex-1">
        <span className="font-strong">{actor}</span>
        <span className="text-fg-muted"> · {describe(entry)}</span>
      </span>
      <code className="shrink-0 font-mono text-11 text-fg-muted max-tablet:hidden">{entry.action}</code>
      {entry.outcome === 'success' ? null : <Badge tone="danger">{entry.outcome}</Badge>}
    </li>
  );
}

const statusTone = {
  pending: 'neutral',
  running: 'neutral',
  completed: 'success',
  failed: 'danger',
  expired: 'neutral',
} as const;

function ExportsPanel({ filters }: { filters: Filters }) {
  const org = useOrg();
  const client = useQueryClient();
  const exportsQuery = useQuery({
    queryKey: orgKey(org, 'audit', 'exports'),
    queryFn: () =>
      unwrap(api.GET('/orgs/{orgId}/audit-logs/exports', { params: { path: { orgId: org.id } } })),
    // Poll while a job is still working.
    refetchInterval: (q) =>
      q.state.data?.data.some((e) => e.status === 'pending' || e.status === 'running') ? 3000 : false,
  });
  const create = useMutation({
    mutationFn: () =>
      unwrap(
        api.POST('/orgs/{orgId}/audit-logs/exports', { params: { path: { orgId: org.id } }, body: filters }),
      ),
    onSuccess: () => {
      toast.success('Export started. It will be ready to download shortly.');
      void client.invalidateQueries({ queryKey: orgKey(org, 'audit', 'exports') });
    },
    onError: (err) => toast.error(errorMessage(err)),
  });
  if (exportsQuery.error instanceof ApiError && exportsQuery.error.status === 403) return null;
  const list: Export[] = exportsQuery.data?.data ?? [];
  return (
    <Panel
      id="exports"
      title="Exports"
      description="Download the events matching the filters above as a CSV file. Files are kept for 7 days."
      action={
        <Button loading={create.isPending} onClick={() => create.mutate()}>
          <FileClock aria-hidden /> Export CSV
        </Button>
      }
    >
      {list.length === 0 ? (
        <p className="text-13 text-fg-muted">No exports yet.</p>
      ) : (
        <ul aria-label="Exports" className="-my-1">
          {list.map((e) => (
            <li
              key={e.id}
              className="flex items-center gap-3 border-b border-border py-2.5 text-13 last:border-b-0"
            >
              <span className="min-w-0 flex-1">
                {formatDateTime(e.created_at)}
                {e.row_count !== null ? <span className="text-fg-muted"> · {e.row_count} events</span> : null}
              </span>
              <Badge tone={statusTone[e.status]}>{e.status}</Badge>
              {e.status === 'completed' ? (
                <Button asChild size="sm">
                  <a href={`/api/v1/orgs/${org.id}/audit-logs/exports/${e.id}/download`}>
                    <Download aria-hidden /> Download
                  </a>
                </Button>
              ) : null}
            </li>
          ))}
        </ul>
      )}
    </Panel>
  );
}
