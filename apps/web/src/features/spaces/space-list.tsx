'use client';

import type { Space } from '@matecollab/contracts';
import {
  Button,
  Dialog,
  DialogContent,
  DialogFooter,
  EmptyState,
  Field,
  Input,
  Select,
  Skeleton,
  Textarea,
  toast,
} from '@matecollab/ui';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { FolderKanban, Globe2, Layers3, LockKeyhole, Plus, UsersRound } from 'lucide-react';
import Link from 'next/link';
import type { Route } from 'next';
import { useState, type FormEvent } from 'react';
import { FormNotice } from '@/features/auth/auth-frame';
import { orgKey, orgPath, useOrg } from '@/features/org/org-context';
import { ApiError, api, errorMessage, unwrap } from '@/lib/api';
import { AccessDenied } from '@/features/admin/access-denied';
import { useSpaces } from './data';

export function SpacesManager({ administration = false }: { administration?: boolean }) {
  const org = useOrg();
  const spaces = useSpaces();
  const client = useQueryClient();
  const [createOpen, setCreateOpen] = useState(false);
  const join = useMutation({
    mutationFn: (id: string) =>
      unwrap(
        api.POST('/orgs/{orgId}/spaces/{spaceId}/join', {
          params: { path: { orgId: org.id, spaceId: id } },
        }),
      ),
    onSuccess: () => {
      toast.success('Joined Space');
      void client.invalidateQueries({ queryKey: orgKey(org, 'spaces') });
    },
    onError: (error) => toast.error(errorMessage(error)),
  });
  const mayCreate = org.role !== 'guest';
  if (administration && org.role !== 'owner' && org.role !== 'admin') {
    return <AccessDenied home={orgPath(org, '/home')} what="manage organisation Spaces" />;
  }
  return (
    <div className="max-w-250">
      <div className="mb-5 flex flex-wrap items-center justify-between gap-4">
        <p className="text-13 text-fg-muted">
          {administration
            ? 'Manage Spaces across this organisation.'
            : 'Browse public Spaces and return to your teams and projects.'}
        </p>
        {mayCreate && (
          <Button variant="primary" onClick={() => setCreateOpen(true)}>
            <Plus aria-hidden /> Create Space
          </Button>
        )}
      </div>
      {spaces.isPending ? (
        <div className="flex flex-col gap-3">
          {[0, 1, 2].map((n) => (
            <Skeleton key={n} className="h-24 w-full" />
          ))}
        </div>
      ) : spaces.isError ? (
        <EmptyState
          role="alert"
          title="Spaces couldn't be loaded"
          description={errorMessage(spaces.error)}
          action={<Button onClick={() => void spaces.refetch()}>Try again</Button>}
        />
      ) : spaces.data.length === 0 ? (
        <div className="rounded-md border border-border bg-card">
          <EmptyState
            icon={<Layers3 />}
            title="No Spaces yet"
            description={
              mayCreate
                ? 'Create a Space for a team or project.'
                : 'Your administrator can grant you access to a Space.'
            }
            action={
              mayCreate ? (
                <Button variant="primary" onClick={() => setCreateOpen(true)}>
                  Create Space
                </Button>
              ) : undefined
            }
          />
        </div>
      ) : (
        <div className="overflow-hidden rounded-md border border-border bg-card">
          {spaces.data.map((space, i) => (
            <SpaceRow
              key={space.id}
              space={space}
              last={i === spaces.data.length - 1}
              joining={join.isPending}
              join={() => join.mutate(space.id)}
            />
          ))}
        </div>
      )}
      {createOpen && <CreateSpaceDialog onClose={() => setCreateOpen(false)} />}
    </div>
  );
}

function SpaceRow({
  space,
  last,
  joining,
  join,
}: {
  space: Space;
  last: boolean;
  joining: boolean;
  join: () => void;
}) {
  const org = useOrg();
  const url = orgPath(org, `/spaces/${space.id}`) as Route;
  return (
    <div
      className={`flex items-center gap-4 px-5 py-4 max-phone:flex-wrap ${last ? '' : 'border-b border-border'}`}
    >
      <div className="flex size-10 shrink-0 items-center justify-center rounded-md bg-primary/10 text-primary-ink">
        {space.kind === 'project' ? (
          <FolderKanban aria-hidden className="size-5" />
        ) : (
          <UsersRound aria-hidden className="size-5" />
        )}
      </div>
      <div className="min-w-0 flex-1">
        <Link href={url} className="text-14 font-strong text-fg hover:text-primary-ink hover:underline">
          {space.name}
        </Link>
        <p className="mt-0.5 line-clamp-1 text-12 text-fg-muted">{space.description || 'No description'}</p>
        <div className="mt-2 flex items-center gap-3 text-11 text-fg-muted">
          <span className="inline-flex items-center gap-1">
            {space.visibility === 'private' ? (
              <LockKeyhole className="size-3" />
            ) : (
              <Globe2 className="size-3" />
            )}
            {space.visibility === 'private' ? 'Private' : 'Public'}
          </span>
          <span>{space.kind === 'team' ? 'Team' : 'Project'}</span>
          {space.joined && <span>{space.viewer_role === 'space_admin' ? 'Space Admin' : 'Joined'}</span>}
        </div>
      </div>
      {space.joined ? (
        <Button asChild variant="secondary">
          <Link href={url}>Open Space</Link>
        </Button>
      ) : (
        <Button onClick={join} disabled={joining}>
          Join Space
        </Button>
      )}
    </div>
  );
}

export function CreateSpaceDialog({ onClose }: { onClose: () => void }) {
  const org = useOrg();
  const client = useQueryClient();
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [kind, setKind] = useState<'team' | 'project'>('team');
  const [visibility, setVisibility] = useState<'public' | 'private'>('public');
  const create = useMutation({
    mutationFn: () =>
      unwrap(
        api.POST('/orgs/{orgId}/spaces', {
          params: { path: { orgId: org.id } },
          body: { name: name.trim(), kind, visibility, description: description.trim() || null },
        }),
      ),
    onSuccess: () => {
      toast.success('Space created');
      void client.invalidateQueries({ queryKey: orgKey(org, 'spaces') });
      onClose();
    },
  });
  function submit(e: FormEvent) {
    e.preventDefault();
    create.mutate();
  }
  const err = create.error instanceof ApiError ? create.error : null;
  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent
        title="Create Space"
        description="Give a team or project a dedicated place to collaborate."
      >
        <form onSubmit={submit} className="flex flex-col gap-4">
          {create.error && !err?.fields.length && <FormNotice>{errorMessage(create.error)}</FormNotice>}
          <Field label="Space name" error={err?.field('name')}>
            <Input value={name} onChange={(e) => setName(e.target.value)} required maxLength={80} autoFocus />
          </Field>
          <Field
            label="Description"
            description="Optional, visible to people who can discover the Space."
            error={err?.field('description')}
          >
            <Textarea
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              maxLength={500}
              rows={3}
            />
          </Field>
          <Field label="Type">
            <Select
              value={kind}
              onValueChange={(v) => setKind(v as typeof kind)}
              options={[
                { value: 'team', label: 'Team' },
                { value: 'project', label: 'Project' },
              ]}
            />
          </Field>
          <Field label="Visibility">
            <Select
              value={visibility}
              onValueChange={(v) => setVisibility(v as typeof visibility)}
              options={[
                { value: 'public', label: 'Public. People can discover and join.' },
                { value: 'private', label: 'Private. Only invited members can see it.' },
              ]}
            />
          </Field>
          <DialogFooter>
            <Button type="button" onClick={onClose}>
              Cancel
            </Button>
            <Button type="submit" variant="primary" loading={create.isPending} disabled={!name.trim()}>
              Create Space
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
