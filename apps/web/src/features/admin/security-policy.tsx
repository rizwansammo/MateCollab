'use client';

import type { SecuritySettings, SecuritySettingsDocument } from '@matecollab/contracts';
import {
  Button,
  ChoiceLabel,
  RadioGroup,
  RadioGroupItem,
  Select,
  Skeleton,
  Switch,
  toast,
} from '@matecollab/ui';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState, type ReactNode } from 'react';
import { Panel } from '@/components/panel';
import { FormNotice } from '@/features/auth/auth-frame';
import { orgKey, orgPath, useOrg } from '@/features/org/org-context';
import { ApiError, api, errorMessage, unwrap } from '@/lib/api';
import { formatDateTime } from '@/lib/time';
import { AccessDenied } from './access-denied';

const maxAges = [
  { value: '12h', label: '12 hours' },
  { value: '24h', label: '24 hours' },
  { value: '7d', label: '7 days' },
  { value: '30d', label: '30 days (maximum)' },
];

export function SecurityPolicy() {
  const org = useOrg();
  const settings = useQuery({
    queryKey: orgKey(org, 'settings', 'security'),
    queryFn: () =>
      unwrap(api.GET('/orgs/{orgId}/settings/security', { params: { path: { orgId: org.id } } })),
  });
  if (settings.error instanceof ApiError && settings.error.status === 403) {
    return <AccessDenied home={orgPath(org, '/home')} what="change the security policy" />;
  }
  if (settings.isPending) return <Skeleton className="h-60 w-full max-w-177.5" />;
  if (settings.isError) return <FormNotice>{errorMessage(settings.error)}</FormNotice>;
  // Keyed by version: a saved or reloaded version starts a fresh draft.
  return (
    <PolicyForm key={settings.data.version} doc={settings.data} reload={() => void settings.refetch()} />
  );
}

function PolicyForm({ doc, reload }: { doc: SecuritySettingsDocument; reload: () => void }) {
  const org = useOrg();
  const client = useQueryClient();
  const { version, updated_at: updatedAt, ...saved } = doc;
  const [draft, setDraft] = useState<SecuritySettings>(saved);
  const save = useMutation({
    mutationFn: (values: SecuritySettings) =>
      unwrap(
        api.PUT('/orgs/{orgId}/settings/security', {
          params: { path: { orgId: org.id }, header: { 'If-Match': `"${version}"` } },
          body: values,
        }),
      ),
    onSuccess: (next) => {
      client.setQueryData(orgKey(org, 'settings', 'security'), next);
      void client.invalidateQueries({ queryKey: orgKey(org, 'profile') });
      toast.success('Security policy saved');
    },
    onError: (err) => {
      if (err instanceof ApiError && err.code === 'version_mismatch') {
        toast.error('Someone else changed these settings. The latest version has been loaded.');
        reload();
      }
    },
  });

  const set = <K extends keyof SecuritySettings>(k: K, v: SecuritySettings[K]) =>
    setDraft({ ...draft, [k]: v });
  const dirty = JSON.stringify(saved) !== JSON.stringify(draft);
  const saveError =
    save.error instanceof ApiError && save.error.code === 'version_mismatch'
      ? null
      : save.error
        ? errorMessage(save.error)
        : null;

  return (
    <form
      className="max-w-177.5"
      onSubmit={(e) => {
        e.preventDefault();
        save.mutate(draft);
      }}
    >
      {saveError ? <FormNotice>{saveError}</FormNotice> : null}
      <Panel id="sign-in" title="Sign-in" description="Applies to everyone when they use this organisation.">
        <Setting
          label="Require two-step verification"
          description="Members without it can only reach their security settings until they turn it on."
          control={
            <Switch
              aria-label="Require two-step verification"
              checked={draft.require_2fa}
              onCheckedChange={(v) => set('require_2fa', v)}
            />
          }
        />
        <Setting
          label="Sign in again after"
          description="How long a session can be used with this organisation before signing in again."
          control={
            <Select
              aria-label="Sign in again after"
              options={maxAges}
              value={draft.session_max_age}
              onValueChange={(v) => set('session_max_age', v as SecuritySettings['session_max_age'])}
              className="w-48"
            />
          }
        />
      </Panel>
      <Panel id="invitations" title="Invitations" description="Who can bring new people in.">
        <fieldset className="border-b border-border pb-4">
          <legend className="mb-3 text-13 font-strong">Who can invite Members</legend>
          <RadioGroup
            value={draft.invite_permission}
            onValueChange={(v) => set('invite_permission', v as SecuritySettings['invite_permission'])}
          >
            <ChoiceLabel control={<RadioGroupItem value="admins" />} label="Owners and Admins" />
            <ChoiceLabel control={<RadioGroupItem value="members" />} label="All Members" />
          </RadioGroup>
        </fieldset>
        <Setting
          label="Allow guest invitations"
          description="Owners and Admins can invite Guests with limited access."
          control={
            <Switch
              aria-label="Allow guest invitations"
              checked={draft.allow_guest_invites}
              onCheckedChange={(v) => set('allow_guest_invites', v)}
            />
          }
        />
      </Panel>
      <div className="mt-6 flex items-center justify-between gap-4 max-phone:flex-col max-phone:items-stretch">
        <p className="text-12 text-fg-muted">
          {version > 1 ? `Last changed ${formatDateTime(updatedAt)}` : 'Default settings'}
        </p>
        <div className="flex gap-2.5">
          <Button disabled={!dirty || save.isPending} onClick={() => setDraft(saved)}>
            Discard
          </Button>
          <Button type="submit" variant="primary" disabled={!dirty} loading={save.isPending}>
            Save changes
          </Button>
        </div>
      </div>
    </form>
  );
}

function Setting({
  label,
  description,
  control,
}: {
  label: ReactNode;
  description: ReactNode;
  control: ReactNode;
}) {
  return (
    <div className="flex items-center justify-between gap-6 border-b border-border py-4 first:pt-0 last:border-b-0 last:pb-0 max-phone:flex-col max-phone:items-start">
      <div className="min-w-0">
        <p className="text-13 font-strong">{label}</p>
        <p className="mt-0.5 text-12 text-fg-muted">{description}</p>
      </div>
      {control}
    </div>
  );
}
