'use client';

import type { Member } from '@matecollab/contracts';
import { Button, Field, Input, Select, Skeleton, toast } from '@matecollab/ui';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { Panel } from '@/components/panel';
import { useSession } from '@/features/auth/session';
import { orgKey, useOrg } from '@/features/org/org-context';
import { ApiError, api, errorMessage, unwrap } from '@/lib/api';

/** The member's own title and contact email in the current organisation. */
export function MembershipProfilePanel() {
  const org = useOrg();
  const { user } = useSession();
  const me = useQuery({
    queryKey: orgKey(org, 'members', user.id),
    queryFn: () =>
      unwrap(
        api.GET('/orgs/{orgId}/members/{userId}', { params: { path: { orgId: org.id, userId: user.id } } }),
      ),
  });
  const emails = useQuery({ queryKey: ['me', 'emails'], queryFn: () => unwrap(api.GET('/me/emails')) });
  return (
    <Panel
      id="membership"
      title={`Your profile in ${org.name}`}
      description="Your job title and the email address people in this organisation see."
    >
      {me.isPending || emails.isPending ? (
        <Skeleton className="h-24 w-full" />
      ) : me.isError || emails.isError ? (
        <p className="text-13 text-danger">{errorMessage(me.error ?? emails.error)}</p>
      ) : (
        <MembershipForm
          key={`${me.data.title ?? ''}|${me.data.email ?? ''}`}
          member={me.data}
          addresses={emails.data.data.filter((e) => e.verified_at).map((e) => e.email)}
        />
      )}
    </Panel>
  );
}

function MembershipForm({ member, addresses }: { member: Member; addresses: string[] }) {
  const org = useOrg();
  const client = useQueryClient();
  const [title, setTitle] = useState(member.title ?? '');
  const [contact, setContact] = useState(member.email ?? addresses[0] ?? '');
  const save = useMutation({
    mutationFn: () =>
      unwrap(
        api.PATCH('/orgs/{orgId}/me/membership', {
          params: { path: { orgId: org.id } },
          body: { title: title.trim() || null, ...(contact ? { contact_email: contact } : {}) },
        }),
      ),
    onSuccess: (m) => {
      client.setQueryData(orgKey(org, 'members', m.user_id), m);
      void client.invalidateQueries({ queryKey: orgKey(org, 'members') });
      toast.success('Organisation profile updated');
    },
  });
  const err = save.error instanceof ApiError ? save.error : null;
  const dirty = title.trim() !== (member.title ?? '') || contact !== (member.email ?? '');
  return (
    <form
      className="max-w-120"
      onSubmit={(e) => {
        e.preventDefault();
        save.mutate();
      }}
    >
      {save.error && !err?.fields.length ? (
        <p role="alert" className="mb-3 text-13 text-danger">
          {errorMessage(save.error)}
        </p>
      ) : null}
      <Field label="Job title" error={err?.field('title')} description="Optional. Shown next to your name.">
        <Input value={title} onChange={(e) => setTitle(e.target.value)} maxLength={100} />
      </Field>
      <Field
        label="Contact email"
        error={err?.field('contact_email')}
        description="Only your confirmed addresses can be used. Add more under Email addresses."
      >
        <Select
          options={addresses.map((a) => ({ value: a, label: a }))}
          value={contact}
          onValueChange={setContact}
        />
      </Field>
      <Button type="submit" variant="primary" loading={save.isPending} disabled={!dirty}>
        Save
      </Button>
    </form>
  );
}
