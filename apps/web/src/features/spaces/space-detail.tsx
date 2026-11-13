'use client';

import type { Space } from '@matecollab/contracts';
import {
  Button,
  ConfirmDialog,
  Dialog,
  DialogContent,
  DialogFooter,
  EmptyState,
  Field,
  Input,
  PageHeader,
  Select,
  Skeleton,
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
  Textarea,
  toast,
} from '@matecollab/ui';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Archive, Hash, LockKeyhole, Plus, Settings, UsersRound } from 'lucide-react';
import Link from 'next/link';
import type { Route } from 'next';
import { useRouter } from 'next/navigation';
import { useState, type FormEvent } from 'react';
import { FormNotice } from '@/features/auth/auth-frame';
import { orgKey, orgPath, useOrg } from '@/features/org/org-context';
import { api, errorMessage, unwrap } from '@/lib/api';
import { useChannels, useSpace } from './data';

export function SpaceDetails({ spaceId }: { spaceId: string }) {
  const org = useOrg();
  const router = useRouter();
  const client = useQueryClient();
  const query = useSpace(spaceId);
  const [channelOpen, setChannelOpen] = useState(false);
  const [editOpen, setEditOpen] = useState(false);
  const [confirmArchive, setConfirmArchive] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const archive = useMutation({
    mutationFn: () =>
      unwrap(
        api.POST('/orgs/{orgId}/spaces/{spaceId}/archive', { params: { path: { orgId: org.id, spaceId } } }),
      ),
    onSuccess: () => {
      toast.success('Space archived');
      void client.invalidateQueries({ queryKey: orgKey(org, 'spaces') });
      router.push(orgPath(org, '/spaces') as Route);
    },
    onError: (error) => toast.error(errorMessage(error)),
  });
  const remove = useMutation({
    mutationFn: () =>
      unwrap(api.DELETE('/orgs/{orgId}/spaces/{spaceId}', { params: { path: { orgId: org.id, spaceId } } })),
    onSuccess: () => {
      toast.success('Space deleted');
      void client.invalidateQueries({ queryKey: orgKey(org, 'spaces') });
      router.push(orgPath(org, '/spaces') as Route);
    },
    onError: (error) => toast.error(errorMessage(error)),
  });
  const leave = useMutation({
    mutationFn: () =>
      unwrap(
        api.POST('/orgs/{orgId}/spaces/{spaceId}/leave', { params: { path: { orgId: org.id, spaceId } } }),
      ),
    onSuccess: () => {
      toast.success('Left Space');
      void client.invalidateQueries({ queryKey: orgKey(org, 'spaces') });
      router.push(orgPath(org, '/spaces') as Route);
    },
    onError: (error) => toast.error(errorMessage(error)),
  });
  const join = useMutation({
    mutationFn: () =>
      unwrap(
        api.POST('/orgs/{orgId}/spaces/{spaceId}/join', { params: { path: { orgId: org.id, spaceId } } }),
      ),
    onSuccess: () => {
      toast.success('Joined Space');
      void client.invalidateQueries({ queryKey: orgKey(org, 'spaces') });
      void client.invalidateQueries({ queryKey: orgKey(org, 'space', spaceId) });
    },
    onError: (error) => toast.error(errorMessage(error)),
  });
  if (query.isPending) return <Skeleton className="h-45 w-full" />;
  if (query.isError)
    return (
      <EmptyState
        role="alert"
        title="Space unavailable"
        description={errorMessage(query.error)}
        action={
          <Button asChild>
            <Link href={orgPath(org, '/spaces') as Route}>All Spaces</Link>
          </Button>
        }
      />
    );
  const space = query.data.space;
  const manage =
    org.role !== 'guest' &&
    (space.viewer_role === 'space_admin' || org.role === 'owner' || org.role === 'admin');
  return (
    <div className="max-w-280">
      <PageHeader
        eyebrow="Spaces"
        title={space.name}
        description={space.description || 'A shared place for people and their work.'}
        actions={
          <>
            {!space.joined &&
              org.role !== 'guest' &&
              (space.visibility === 'public' || org.role === 'owner' || org.role === 'admin') && (
                <Button onClick={() => join.mutate()} loading={join.isPending}>
                  Join Space
                </Button>
              )}
            {space.joined && (
              <Button onClick={() => leave.mutate()} loading={leave.isPending}>
                Leave Space
              </Button>
            )}
            {manage && (
              <Button onClick={() => setEditOpen(true)}>
                <Settings aria-hidden /> Settings
              </Button>
            )}
          </>
        }
      />
      <Tabs defaultValue="overview">
        <TabsList aria-label="Space views">
          <TabsTrigger value="overview">Overview</TabsTrigger>
          <TabsTrigger value="channels">Channels</TabsTrigger>
          <TabsTrigger value="members">Members</TabsTrigger>
          <TabsTrigger value="files">Files</TabsTrigger>
          <TabsTrigger value="meetings">Meetings</TabsTrigger>
        </TabsList>
        <TabsContent value="overview">
          <div className="rounded-md border border-border bg-card p-6">
            <h2 className="mb-3 text-16 font-heading">Space overview</h2>
            {space.overview ? (
              <p className="whitespace-pre-wrap text-14 leading-relaxed text-fg">{space.overview}</p>
            ) : (
              <p className="text-13 text-fg-muted">No overview note has been added yet.</p>
            )}
            <p className="mt-5 text-12 text-fg-muted">
              {space.kind === 'team' ? 'Team Space' : 'Project Space'} ·{' '}
              {space.visibility === 'public' ? 'Public' : 'Private'}
            </p>
          </div>
        </TabsContent>
        <TabsContent value="channels">
          {space.joined ? (
            <ChannelsSection space={space} onCreate={() => setChannelOpen(true)} />
          ) : (
            <EmptyState
              title="Join this Space to view Channels"
              description="Channel content is available only to Space members."
            />
          )}
        </TabsContent>
        <TabsContent value="members">
          {space.joined || manage ? (
            <MembersSection space={space} manage={manage} />
          ) : (
            <EmptyState title="Members are private" description="Join this Space to see its members." />
          )}
        </TabsContent>
        <TabsContent value="files">
          <EmptyState
            title="Files arrive in Phase 6"
            description="Space files will be available when file sharing is implemented."
          />
        </TabsContent>
        <TabsContent value="meetings">
          <EmptyState
            title="Meetings arrive in Phase 8"
            description="Space meetings will be available after the meeting service is implemented."
          />
        </TabsContent>
      </Tabs>
      {channelOpen && <CreateChannelDialog spaceId={spaceId} onClose={() => setChannelOpen(false)} />}
      {editOpen && (
        <EditSpaceDialog
          space={space}
          etag={query.data.etag}
          onClose={() => setEditOpen(false)}
          onArchive={() => {
            setEditOpen(false);
            setConfirmArchive(true);
          }}
          onDelete={() => {
            setEditOpen(false);
            setConfirmDelete(true);
          }}
        />
      )}
      <ConfirmDialog
        open={confirmArchive}
        onOpenChange={setConfirmArchive}
        title="Archive Space?"
        description="The Space and all of its Channels will be archived. Messages cannot be added."
        confirmLabel="Archive Space"
        onConfirm={() => archive.mutate()}
      />
      <ConfirmDialog
        open={confirmDelete}
        onOpenChange={setConfirmDelete}
        title="Permanently delete Space?"
        description="This permanently deletes the Space, its Channels, memberships and guest grants. This action cannot be undone."
        confirmLabel="Delete Space"
        onConfirm={() => remove.mutate()}
      />
    </div>
  );
}

function ChannelsSection({ space, onCreate }: { space: Space; onCreate: () => void }) {
  const org = useOrg();
  const channels = useChannels(space.id);
  const canCreate = org.role !== 'guest';
  return (
    <div>
      <div className="mb-4 flex items-center justify-between gap-3">
        <h2 className="text-15 font-heading">Channels</h2>
        {canCreate && (
          <Button onClick={onCreate}>
            <Plus aria-hidden /> New Channel
          </Button>
        )}
      </div>
      {channels.isPending ? (
        <Skeleton className="h-32 w-full" />
      ) : channels.isError ? (
        <FormNotice>{errorMessage(channels.error)}</FormNotice>
      ) : (
        <div className="overflow-hidden rounded-md border border-border bg-card">
          {channels.data.map((channel) => (
            <Link
              key={channel.id}
              href={orgPath(org, `/spaces/${space.id}/channels/${channel.id}`) as Route}
              className="flex items-center gap-3 border-b border-border px-5 py-3 last:border-0 hover:bg-hover"
            >
              {channel.visibility === 'private' ? (
                <LockKeyhole className="size-4 text-fg-muted" />
              ) : (
                <Hash className="size-4 text-fg-muted" />
              )}
              <span className="min-w-0 flex-1 text-13 text-fg">{channel.name}</span>
              {channel.is_default && <span className="text-11 text-fg-muted">Default</span>}
            </Link>
          ))}
        </div>
      )}
    </div>
  );
}

function MembersSection({ space, manage }: { space: Space; manage: boolean }) {
  const org = useOrg();
  const client = useQueryClient();
  const [userId, setUserId] = useState('');
  const [role, setRole] = useState<'member' | 'space_admin'>('member');
  const members = useQuery({
    queryKey: orgKey(org, 'space', space.id, 'members'),
    queryFn: async () =>
      (
        await unwrap(
          api.GET('/orgs/{orgId}/spaces/{spaceId}/members', {
            params: { path: { orgId: org.id, spaceId: space.id } },
          }),
        )
      ).data,
  });
  const directory = useQuery({
    queryKey: orgKey(org, 'directory', 'space-grants'),
    queryFn: async () =>
      (
        await unwrap(
          api.GET('/orgs/{orgId}/members', {
            params: { path: { orgId: org.id }, query: { limit: 200 } },
          }),
        )
      ).data,
    enabled: manage,
  });
  const add = useMutation({
    mutationFn: () =>
      unwrap(
        api.PUT('/orgs/{orgId}/spaces/{spaceId}/members/{userId}', {
          params: { path: { orgId: org.id, spaceId: space.id, userId } },
          body: { role },
        }),
      ),
    onSuccess: () => {
      toast.success('Member access updated');
      setUserId('');
      void client.invalidateQueries({ queryKey: orgKey(org, 'space', space.id, 'members') });
    },
    onError: (err) => toast.error(errorMessage(err)),
  });
  const remove = useMutation({
    mutationFn: (id: string) =>
      unwrap(
        api.DELETE('/orgs/{orgId}/spaces/{spaceId}/members/{userId}', {
          params: { path: { orgId: org.id, spaceId: space.id, userId: id } },
        }),
      ),
    onSuccess: () => {
      toast.success('Member removed');
      void client.invalidateQueries({ queryKey: orgKey(org, 'space', space.id, 'members') });
    },
    onError: (err) => toast.error(errorMessage(err)),
  });
  return (
    <div className="space-y-4">
      {manage && directory.data && (
        <form
          onSubmit={(e) => {
            e.preventDefault();
            if (userId) add.mutate();
          }}
          className="flex flex-wrap items-end gap-3 rounded-md border border-border bg-card p-4"
        >
          <Field label="Organisation member">
            <Select
              value={userId}
              onValueChange={setUserId}
              options={[
                { value: '', label: 'Choose a person' },
                ...directory.data
                  .filter((m) => m.status === 'active')
                  .map((m) => ({ value: m.user_id, label: m.display_name })),
              ]}
            />
          </Field>
          <Field label="Space role">
            <Select
              value={role}
              onValueChange={(v) => setRole(v as typeof role)}
              options={[
                { value: 'member', label: 'Member' },
                { value: 'space_admin', label: 'Space Admin' },
              ]}
            />
          </Field>
          <Button variant="primary" type="submit" disabled={!userId} loading={add.isPending}>
            Grant access
          </Button>
        </form>
      )}
      {members.isPending ? (
        <Skeleton className="h-20 w-full" />
      ) : members.isError ? (
        <FormNotice>{errorMessage(members.error)}</FormNotice>
      ) : (
        <div className="overflow-hidden rounded-md border border-border bg-card">
          {members.data.map((member) => (
            <div
              key={member.user_id}
              className="flex items-center gap-4 border-b border-border px-5 py-3 last:border-0"
            >
              <UsersRound className="size-4 text-fg-muted" aria-hidden />
              <span className="min-w-0 flex-1 text-13 text-fg">{member.display_name}</span>
              <span className="text-12 text-fg-muted">
                {member.role === 'space_admin' ? 'Space Admin' : 'Member'}
              </span>
              {manage && (
                <Button size="sm" disabled={remove.isPending} onClick={() => remove.mutate(member.user_id)}>
                  Remove
                </Button>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

function CreateChannelDialog({ spaceId, onClose }: { spaceId: string; onClose: () => void }) {
  const org = useOrg();
  const client = useQueryClient();
  const [name, setName] = useState('');
  const [topic, setTopic] = useState('');
  const [visibility, setVisibility] = useState<'public' | 'private'>('public');
  const create = useMutation({
    mutationFn: () =>
      unwrap(
        api.POST('/orgs/{orgId}/spaces/{spaceId}/channels', {
          params: { path: { orgId: org.id, spaceId } },
          body: {
            name: name.trim().toLowerCase().replace(/\s+/g, '-'),
            topic: topic.trim() || null,
            visibility,
          },
        }),
      ),
    onSuccess: () => {
      toast.success('Channel created');
      void client.invalidateQueries({ queryKey: orgKey(org, 'space', spaceId, 'channels') });
      onClose();
    },
  });
  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent title="New Channel" description="Channels organise discussions inside this Space.">
        <form
          onSubmit={(e: FormEvent) => {
            e.preventDefault();
            create.mutate();
          }}
          className="space-y-4"
        >
          {create.isError && <FormNotice>{errorMessage(create.error)}</FormNotice>}
          <Field label="Channel name">
            <Input value={name} maxLength={80} onChange={(e) => setName(e.target.value)} autoFocus />
          </Field>
          <Field label="Topic">
            <Textarea value={topic} onChange={(e) => setTopic(e.target.value)} maxLength={250} rows={2} />
          </Field>
          <Field label="Visibility">
            <Select
              value={visibility}
              onValueChange={(v) => setVisibility(v as typeof visibility)}
              options={[
                { value: 'public', label: 'Public' },
                { value: 'private', label: 'Private' },
              ]}
            />
          </Field>
          <DialogFooter>
            <Button onClick={onClose}>Cancel</Button>
            <Button type="submit" variant="primary" loading={create.isPending} disabled={!name.trim()}>
              Create Channel
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

function EditSpaceDialog({
  space,
  etag,
  onClose,
  onArchive,
  onDelete,
}: {
  space: Space;
  etag: string;
  onClose: () => void;
  onArchive: () => void;
  onDelete: () => void;
}) {
  const org = useOrg();
  const client = useQueryClient();
  const [name, setName] = useState(space.name);
  const [description, setDescription] = useState(space.description || '');
  const [overview, setOverview] = useState(space.overview || '');
  const [visibility, setVisibility] = useState(space.visibility);
  const [kind, setKind] = useState(space.kind);
  const update = useMutation({
    mutationFn: () =>
      unwrap(
        api.PUT('/orgs/{orgId}/spaces/{spaceId}', {
          params: { path: { orgId: org.id, spaceId: space.id }, header: { 'If-Match': etag } },
          body: { name, description: description || null, overview: overview || null, visibility, kind },
        }),
      ),
    onSuccess: () => {
      toast.success('Space updated');
      void client.invalidateQueries({ queryKey: orgKey(org, 'spaces') });
      void client.invalidateQueries({ queryKey: orgKey(org, 'space', space.id) });
      onClose();
    },
  });
  return (
    <Dialog open onOpenChange={(open) => !open && onClose()}>
      <DialogContent
        title="Space settings"
        description="Update the information and visibility of this Space."
      >
        <form
          onSubmit={(e: FormEvent) => {
            e.preventDefault();
            update.mutate();
          }}
          className="space-y-4"
        >
          {update.isError && <FormNotice>{errorMessage(update.error)}</FormNotice>}
          <Field label="Name">
            <Input value={name} maxLength={80} onChange={(e) => setName(e.target.value)} />
          </Field>
          <Field label="Description">
            <Textarea
              value={description}
              maxLength={500}
              onChange={(e) => setDescription(e.target.value)}
              rows={2}
            />
          </Field>
          <Field label="Overview note">
            <Textarea
              value={overview}
              maxLength={5000}
              onChange={(e) => setOverview(e.target.value)}
              rows={4}
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
                { value: 'public', label: 'Public' },
                { value: 'private', label: 'Private' },
              ]}
            />
          </Field>
          <div className="flex flex-wrap gap-3 border-t border-border pt-4">
            <Button type="button" onClick={onArchive}>
              <Archive aria-hidden /> Archive Space
            </Button>
            {(org.role === 'owner' || org.role === 'admin') && (
              <Button type="button" variant="danger" onClick={onDelete}>
                Delete Space
              </Button>
            )}
          </div>
          <DialogFooter>
            <Button onClick={onClose}>Cancel</Button>
            <Button type="submit" variant="primary" loading={update.isPending} disabled={!name.trim()}>
              Save changes
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
