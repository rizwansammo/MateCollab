'use client';

import type { Organisation } from '@matecollab/contracts';
import { Button, ConfirmDialog, Field, Input, Select, Skeleton, toast } from '@matecollab/ui';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Crown } from 'lucide-react';
import { useMemo, useState } from 'react';
import { Panel } from '@/components/panel';
import { FormNotice } from '@/features/auth/auth-frame';
import { sessionKey, useSession } from '@/features/auth/session';
import { useStepUp } from '@/features/auth/step-up';
import { orgKey, orgPath, useOrg } from '@/features/org/org-context';
import { ApiError, api, errorMessage, unwrap } from '@/lib/api';
import { AccessDenied } from './access-denied';

const LOCALES = [
  { value: 'en-GB', label: 'English (United Kingdom)' },
  { value: 'en-US', label: 'English (United States)' },
  { value: 'bn-BD', label: 'Bangla (Bangladesh)' },
];

function timeZones(current: string): { value: string; label: string }[] {
  let zones: string[];
  try {
    zones = Intl.supportedValuesOf('timeZone');
  } catch {
    zones = ['UTC'];
  }
  if (!zones.includes(current)) zones = [current, ...zones];
  return zones.map((z) => ({ value: z, label: z.replaceAll('_', ' ') }));
}

/** Admin → General: organisation profile and, for the Owner, ownership transfer. */
export function GeneralSettings() {
  const org = useOrg();
  const profile = useQuery({
    queryKey: orgKey(org, 'profile', 'editable'),
    queryFn: async () => {
      const { data, error, response } = await api.GET('/orgs/{orgId}', {
        params: { path: { orgId: org.id } },
      });
      if (!data) throw new ApiError(response.status, error);
      return { org: data, etag: response.headers.get('ETag') ?? '' };
    },
  });
  if (profile.error instanceof ApiError && profile.error.status === 403) {
    return <AccessDenied home={orgPath(org, '/home')} what="change organisation settings" />;
  }
  if (profile.isPending) return <Skeleton className="h-60 w-full max-w-177.5" />;
  if (profile.isError) return <FormNotice>{errorMessage(profile.error)}</FormNotice>;
  return (
    <div className="max-w-177.5">
      <ProfileForm key={profile.data.etag} current={profile.data.org} etag={profile.data.etag} />
      {org.role === 'owner' ? <OwnershipPanel /> : null}
    </div>
  );
}

function ProfileForm({ current, etag }: { current: Organisation; etag: string }) {
  const org = useOrg();
  const client = useQueryClient();
  const [name, setName] = useState(current.name);
  const [timezone, setTimezone] = useState(current.default_timezone);
  const [locale, setLocale] = useState(current.default_locale);
  const zones = useMemo(() => timeZones(current.default_timezone), [current.default_timezone]);
  const locales = LOCALES.some((l) => l.value === current.default_locale)
    ? LOCALES
    : [{ value: current.default_locale, label: current.default_locale }, ...LOCALES];
  const save = useMutation({
    mutationFn: () =>
      unwrap(
        api.PATCH('/orgs/{orgId}', {
          params: { path: { orgId: org.id }, header: { 'If-Match': etag } },
          body: { name, default_timezone: timezone, default_locale: locale },
        }),
      ),
    onSuccess: () => {
      toast.success('Organisation updated');
      void client.invalidateQueries({ queryKey: orgKey(org, 'profile') });
      void client.invalidateQueries({ queryKey: sessionKey }); // the switcher shows the name
    },
    onError: (err) => {
      if (err instanceof ApiError && err.code === 'version_mismatch') {
        toast.error('Someone else changed these settings. The latest version has been loaded.');
        void client.invalidateQueries({ queryKey: orgKey(org, 'profile') });
      }
    },
  });
  const err = save.error instanceof ApiError ? save.error : null;
  const dirty =
    name !== current.name || timezone !== current.default_timezone || locale !== current.default_locale;
  return (
    <Panel id="profile" title="Organisation profile" description="Shown to everyone in the organisation.">
      <form
        onSubmit={(e) => {
          e.preventDefault();
          save.mutate();
        }}
      >
        {save.error && err?.code !== 'version_mismatch' && !err?.fields.length ? (
          <FormNotice>{errorMessage(save.error)}</FormNotice>
        ) : null}
        <Field label="Name" error={err?.field('name')}>
          <Input value={name} onChange={(e) => setName(e.target.value)} maxLength={100} />
        </Field>
        <Field label="Web address" description="Used in links to this organisation. It can’t be changed.">
          <Input value={`/${current.slug}`} readOnly />
        </Field>
        <Field label="Default time zone" error={err?.field('default_timezone')}>
          <Select options={zones} value={timezone} onValueChange={setTimezone} />
        </Field>
        <Field label="Default language" error={err?.field('default_locale')}>
          <Select options={locales} value={locale} onValueChange={setLocale} />
        </Field>
        <Button type="submit" variant="primary" loading={save.isPending} disabled={!dirty}>
          Save changes
        </Button>
      </form>
    </Panel>
  );
}

function OwnershipPanel() {
  const org = useOrg();
  const { user } = useSession();
  const client = useQueryClient();
  const stepUp = useStepUp();
  const admins = useQuery({
    queryKey: orgKey(org, 'members', { role: 'admin', status: 'active' }),
    queryFn: () =>
      unwrap(
        api.GET('/orgs/{orgId}/members', {
          params: { path: { orgId: org.id }, query: { role: 'admin', status: 'active', limit: 200 } },
        }),
      ),
  });
  const [target, setTarget] = useState('');
  const [confirm, setConfirm] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const candidates = (admins.data?.data ?? []).filter((m) => m.user_id !== user.id);
  const chosen = candidates.find((m) => m.user_id === target);

  const transfer = async () => {
    setBusy(true);
    setError(null);
    try {
      const ok = await stepUp.run(() =>
        unwrap(
          api.POST('/orgs/{orgId}/ownership-transfer', {
            params: { path: { orgId: org.id } },
            body: { user_id: target },
          }),
        ),
      );
      if (ok) {
        toast.success(`${chosen?.display_name ?? 'The Admin'} is now the Owner`);
        await client.invalidateQueries({ queryKey: sessionKey });
        void client.invalidateQueries({ queryKey: ['org', org.id] });
      }
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBusy(false);
    }
  };

  return (
    <Panel
      id="ownership"
      title={
        <span className="flex items-center gap-2">
          <Crown aria-hidden className="size-4 text-warning" /> Transfer ownership
        </span>
      }
      description="Make an Admin the Owner. You stay in the organisation as an Admin."
    >
      {error ? <FormNotice>{error}</FormNotice> : null}
      {admins.isPending ? (
        <Skeleton className="h-10 w-full" />
      ) : candidates.length === 0 ? (
        <p className="text-13 text-fg-muted">
          Only an active Admin can become the Owner. Make someone an Admin under Members first.
        </p>
      ) : (
        <div className="flex items-end gap-3 max-phone:flex-col max-phone:items-stretch">
          <Field label="New Owner" className="mb-0 flex-1">
            <Select
              placeholder="Choose an Admin"
              options={candidates.map((m) => ({
                value: m.user_id,
                label: `${m.display_name} (${m.email ?? ''})`,
              }))}
              value={target}
              onValueChange={setTarget}
            />
          </Field>
          <Button variant="danger" disabled={!target} loading={busy} onClick={() => setConfirm(true)}>
            Transfer ownership
          </Button>
        </div>
      )}
      <ConfirmDialog
        open={confirm}
        onOpenChange={setConfirm}
        title={`Make ${chosen?.display_name ?? 'this Admin'} the Owner?`}
        description="They get full control of the organisation, including Admins and ownership. You become an Admin. Only the new Owner can reverse this."
        confirmLabel="Transfer ownership"
        onConfirm={() => void transfer()}
      />
      {stepUp.dialog}
    </Panel>
  );
}
