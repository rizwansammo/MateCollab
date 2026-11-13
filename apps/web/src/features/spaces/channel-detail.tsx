'use client';

import type { Channel } from '@matecollab/contracts';
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
import { Archive, Hash, LockKeyhole, Settings, UsersRound } from 'lucide-react';
import Link from 'next/link';
import type { Route } from 'next';
import { useRouter } from 'next/navigation';
import { useState, type FormEvent } from 'react';
import { FormNotice } from '@/features/auth/auth-frame';
import { orgKey, orgPath, useOrg } from '@/features/org/org-context';
import { useSession } from '@/features/auth/session';
import { ApiError, api, errorMessage, unwrap } from '@/lib/api';
import { useSpace } from './data';

export function ChannelDetails({ spaceId, channelId }: { spaceId: string; channelId: string }) {
  const org = useOrg();
  const { user } = useSession();
  const router = useRouter();
  const client = useQueryClient();
  const space = useSpace(spaceId);
  const [editing, setEditing] = useState(false);
  const [archiveOpen, setArchiveOpen] = useState(false);
  const channel = useQuery({
    queryKey: orgKey(org, 'space', spaceId, 'channel', channelId),
    queryFn: async () => {
      const { data, error, response } = await api.GET('/orgs/{orgId}/spaces/{spaceId}/channels/{channelId}', {
        params: { path: { orgId: org.id, spaceId, channelId } },
      });
      if (!data) throw new ApiError(response.status, error);
      return { channel: data, etag: response.headers.get('ETag') || '' };
    },
  });
  const archive = useMutation({
    mutationFn: () =>
      unwrap(
        api.POST('/orgs/{orgId}/spaces/{spaceId}/channels/{channelId}/archive', {
          params: { path: { orgId: org.id, spaceId, channelId } },
        }),
      ),
    onSuccess: () => {
      toast.success('Channel archived');
      void client.invalidateQueries({ queryKey: orgKey(org, 'space', spaceId, 'channels') });
      router.push(orgPath(org, `/spaces/${spaceId}`) as Route);
    },
    onError: (err) => toast.error(errorMessage(err)),
  });
  if (channel.isPending || space.isPending) return <Skeleton className="h-44 w-full" />;
  if (channel.isError || space.isError)
    return (
      <EmptyState
        role="alert"
        title="Channel unavailable"
        description={errorMessage(channel.error || space.error)}
        action={
          <Button asChild>
            <Link href={orgPath(org, '/spaces') as Route}>Spaces</Link>
          </Button>
        }
      />
    );
  const ch = channel.data.channel;
  const manage =
    org.role !== 'guest' &&
    (org.role === 'owner' ||
      org.role === 'admin' ||
      space.data.space.viewer_role === 'space_admin' ||
      ch.created_by === user.id);
  return (
    <div className="max-w-280">
      <PageHeader
        eyebrow={space.data.space.name}
        title={`#${ch.name}`}
        description={ch.topic || 'No topic has been added.'}
        actions={
          manage && !ch.is_default ? (
            <Button onClick={() => setEditing(true)}>
              <Settings aria-hidden />
              Channel settings
            </Button>
          ) : undefined
        }
      />
      <Tabs defaultValue="conversation">
        <TabsList aria-label="Channel views">
          <TabsTrigger value="conversation">Conversation</TabsTrigger>
          <TabsTrigger value="members">Members</TabsTrigger>
        </TabsList>
        <TabsContent value="conversation">
          <div className="rounded-md border border-border bg-card p-8 text-center">
            {ch.visibility === 'private' ? (
              <LockKeyhole className="mx-auto size-7 text-fg-muted" />
            ) : (
              <Hash className="mx-auto size-7 text-fg-muted" />
            )}
            <h2 className="mt-3 text-16 font-heading">Channel is ready</h2>
            <p className="mt-2 text-13 text-fg-muted">
              Messages and realtime conversations will be available in Phase 4.
            </p>
          </div>
        </TabsContent>
        <TabsContent value="members">
          {ch.visibility === 'public' ? (
            <div className="rounded-md border border-border bg-card p-5 text-13 text-fg-muted">
              Everyone in this Space can access the public Channel.
            </div>
          ) : (
            <PrivateChannelMembers spaceId={spaceId} channel={ch} />
          )}
        </TabsContent>
      </Tabs>
      {editing && (
        <EditChannelDialog
          spaceId={spaceId}
          channel={ch}
          etag={channel.data.etag}
          onClose={() => setEditing(false)}
          onArchive={() => {
            setEditing(false);
            setArchiveOpen(true);
          }}
        />
      )}
      <ConfirmDialog
        open={archiveOpen}
        onOpenChange={setArchiveOpen}
        title="Archive Channel?"
        description="This Channel will no longer be available for new conversations."
        confirmLabel="Archive Channel"
        onConfirm={() => archive.mutate()}
      />
    </div>
  );
}

function PrivateChannelMembers({ spaceId, channel }: { spaceId: string; channel: Channel }) {
  const org = useOrg();
  const client = useQueryClient();
  const [userId, setUserId] = useState('');
  const path = { orgId: org.id, spaceId, channelId: channel.id };
  const members = useQuery({
    queryKey: orgKey(org, 'space', spaceId, 'channel', channel.id, 'members'),
    queryFn: async () =>
      (
        await unwrap(
          api.GET('/orgs/{orgId}/spaces/{spaceId}/channels/{channelId}/members', { params: { path } }),
        )
      ).data,
  });
  const spaceMembers = useQuery({
    queryKey: orgKey(org, 'space', spaceId, 'members'),
    queryFn: async () =>
      (
        await unwrap(
          api.GET('/orgs/{orgId}/spaces/{spaceId}/members', { params: { path: { orgId: org.id, spaceId } } }),
        )
      ).data,
  });
  const add = useMutation({
    mutationFn: () =>
      unwrap(
        api.POST('/orgs/{orgId}/spaces/{spaceId}/channels/{channelId}/members', {
          params: { path },
          body: { user_id: userId },
        }),
      ),
    onSuccess: () => {
      toast.success('Channel access granted');
      setUserId('');
      void client.invalidateQueries({
        queryKey: orgKey(org, 'space', spaceId, 'channel', channel.id, 'members'),
      });
    },
    onError: (e) => toast.error(errorMessage(e)),
  });
  const remove = useMutation({
    mutationFn: (id: string) =>
      unwrap(
        api.DELETE('/orgs/{orgId}/spaces/{spaceId}/channels/{channelId}/members/{userId}', {
          params: { path: { ...path, userId: id } },
        }),
      ),
    onSuccess: () => {
      toast.success('Channel member removed');
      void client.invalidateQueries({
        queryKey: orgKey(org, 'space', spaceId, 'channel', channel.id, 'members'),
      });
    },
    onError: (e) => toast.error(errorMessage(e)),
  });
  return (
    <div className="space-y-4">
      {org.role !== 'guest' && spaceMembers.data && (
        <form
          onSubmit={(e: FormEvent) => {
            e.preventDefault();
            if (userId) add.mutate();
          }}
          className="flex flex-wrap items-end gap-3 rounded-md border border-border bg-card p-4"
        >
          <Field label="Space member">
            <Select
              value={userId}
              onValueChange={setUserId}
              options={[
                { value: '', label: 'Choose a person' },
                ...spaceMembers.data.map((m) => ({ value: m.user_id, label: m.display_name })),
              ]}
            />
          </Field>
          <Button variant="primary" type="submit" disabled={!userId} loading={add.isPending}>
            Add to Channel
          </Button>
        </form>
      )}
      {members.isPending ? (
        <Skeleton className="h-24 w-full" />
      ) : members.isError ? (
        <FormNotice>{errorMessage(members.error)}</FormNotice>
      ) : (
        <div className="overflow-hidden rounded-md border border-border bg-card">
          {members.data.map((m) => (
            <div
              key={m.user_id}
              className="flex items-center gap-3 border-b border-border px-4 py-3 last:border-0"
            >
              <UsersRound className="size-4 text-fg-muted" aria-hidden />
              <span className="min-w-0 flex-1 text-13 text-fg">{m.display_name}</span>
              {org.role !== 'guest' && (
                <Button size="sm" disabled={remove.isPending} onClick={() => remove.mutate(m.user_id)}>
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

function EditChannelDialog({
  spaceId,
  channel,
  etag,
  onClose,
  onArchive,
}: {
  spaceId: string;
  channel: Channel;
  etag: string;
  onClose: () => void;
  onArchive: () => void;
}) {
  const org = useOrg();
  const client = useQueryClient();
  const [name, setName] = useState(channel.name);
  const [topic, setTopic] = useState(channel.topic || '');
  const [visibility, setVisibility] = useState(channel.visibility);
  const update = useMutation({
    mutationFn: () =>
      unwrap(
        api.PUT('/orgs/{orgId}/spaces/{spaceId}/channels/{channelId}', {
          params: { path: { orgId: org.id, spaceId, channelId: channel.id }, header: { 'If-Match': etag } },
          body: { name, topic: topic || null, visibility },
        }),
      ),
    onSuccess: () => {
      toast.success('Channel updated');
      void client.invalidateQueries({ queryKey: orgKey(org, 'space', spaceId, 'channels') });
      void client.invalidateQueries({ queryKey: orgKey(org, 'space', spaceId, 'channel', channel.id) });
      onClose();
    },
  });
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent title="Channel settings" description="Update your Channel information.">
        <form
          onSubmit={(e: FormEvent) => {
            e.preventDefault();
            update.mutate();
          }}
          className="space-y-4"
        >
          {update.isError && <FormNotice>{errorMessage(update.error)}</FormNotice>}
          <Field label="Channel name">
            <Input value={name} onChange={(e) => setName(e.target.value)} maxLength={80} />
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
          <div className="border-t border-border pt-4">
            <Button onClick={onArchive}>
              <Archive aria-hidden /> Archive Channel
            </Button>
          </div>
          <DialogFooter>
            <Button onClick={onClose}>Cancel</Button>
            <Button variant="primary" type="submit" loading={update.isPending}>
              Save changes
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
