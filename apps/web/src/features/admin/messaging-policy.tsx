'use client';

import type { MessagingSettingsDocument } from '@matecollab/contracts';
import { Button, Select, Skeleton, toast } from '@matecollab/ui';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState, type FormEvent } from 'react';
import { Panel } from '@/components/panel';
import { FormNotice } from '@/features/auth/auth-frame';
import { orgKey, orgPath, useOrg } from '@/features/org/org-context';
import { ApiError, api, errorMessage, unwrap } from '@/lib/api';
import { AccessDenied } from './access-denied';

export function MessagingPolicy() {
  const org = useOrg();
  const query = useQuery({
    queryKey: orgKey(org, 'settings', 'messaging'),
    queryFn: () =>
      unwrap(api.GET('/orgs/{orgId}/settings/messaging', { params: { path: { orgId: org.id } } })),
  });
  if (query.error instanceof ApiError && query.error.status === 403) {
    return <AccessDenied home={orgPath(org, '/home')} what="manage the messaging policy" />;
  }
  if (query.isPending) return <Skeleton className="h-64 w-full max-w-177.5" />;
  if (query.isError) return <FormNotice>{errorMessage(query.error)}</FormNotice>;
  return <PolicyForm key={query.data.version} current={query.data} />;
}
function PolicyForm({ current }: { current: MessagingSettingsDocument }) {
  const org = useOrg();
  const client = useQueryClient();
  const [publicCreation, setPublicCreation] = useState(current.public_space_creation);
  const [privateCreation, setPrivateCreation] = useState(current.private_space_creation);
  const dirty =
    publicCreation !== current.public_space_creation || privateCreation !== current.private_space_creation;
  const save = useMutation({
    mutationFn: () =>
      unwrap(
        api.PUT('/orgs/{orgId}/settings/messaging', {
          params: { path: { orgId: org.id }, header: { 'If-Match': `"${current.version}"` } },
          body: { public_space_creation: publicCreation, private_space_creation: privateCreation },
        }),
      ),
    onSuccess: (data) => {
      toast.success('Messaging policy saved');
      client.setQueryData(orgKey(org, 'settings', 'messaging'), data);
    },
    onError: (err) => {
      toast.error(errorMessage(err));
      if (err instanceof ApiError && err.code === 'version_mismatch') {
        void client.invalidateQueries({ queryKey: orgKey(org, 'settings', 'messaging') });
      }
    },
  });
  function submit(e: FormEvent) {
    e.preventDefault();
    save.mutate();
  }
  return (
    <form className="max-w-177.5" onSubmit={submit}>
      <Panel
        id="space-creation"
        title="Space creation"
        description="The organisation Owner and Admins can always create Spaces. Control whether Members can create them."
      >
        {save.isError && <FormNotice>{errorMessage(save.error)}</FormNotice>}
        <div className="space-y-5">
          <div>
            <p className="mb-1 text-13 font-strong text-fg">Public Spaces</p>
            <p className="mb-2 text-12 text-fg-muted">Who can create public Team or Project Spaces?</p>
            <Select
              value={publicCreation}
              onValueChange={(v) => setPublicCreation(v as typeof publicCreation)}
              options={[
                { value: 'all', label: 'Owners, Admins and Members' },
                { value: 'admins', label: 'Owners and Admins only' },
              ]}
            />
          </div>
          <div>
            <p className="mb-1 text-13 font-strong text-fg">Private Spaces</p>
            <p className="mb-2 text-12 text-fg-muted">Who can create private Team or Project Spaces?</p>
            <Select
              value={privateCreation}
              onValueChange={(v) => setPrivateCreation(v as typeof privateCreation)}
              options={[
                { value: 'all', label: 'Owners, Admins and Members' },
                { value: 'admins', label: 'Owners and Admins only' },
              ]}
            />
          </div>
        </div>
      </Panel>
      <div className="mt-6 flex justify-end gap-3">
        <Button
          type="button"
          onClick={() => {
            setPublicCreation(current.public_space_creation);
            setPrivateCreation(current.private_space_creation);
          }}
          disabled={!dirty}
        >
          Discard
        </Button>
        <Button type="submit" variant="primary" disabled={!dirty} loading={save.isPending}>
          Save changes
        </Button>
      </div>
    </form>
  );
}
