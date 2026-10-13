'use client';

import type { Role } from '@matecollab/contracts';
import { Skeleton } from '@matecollab/ui';
import { useQuery } from '@tanstack/react-query';
import { Check, Minus } from 'lucide-react';
import { FormNotice } from '@/features/auth/auth-frame';
import { orgKey, useOrg } from '@/features/org/org-context';
import { roleDescriptions, roleNames } from '@/features/org/roles';
import { api, errorMessage, unwrap } from '@/lib/api';

const settingNotes: Record<string, string> = {
  'security.invite_permission': 'Changes with “Who can invite Members” in Security policy.',
  'security.allow_guest_invites': 'Changes with “Allow guest invitations” in Security policy.',
};

/**
 * Roles & Permissions: the fixed role model with this organisation's effective permissions,
 * exactly as the server's authorisation catalogue reports them (no copy of the rules here).
 */
export function RolesAndPermissions() {
  const org = useOrg();
  const catalogue = useQuery({
    queryKey: orgKey(org, 'roles'),
    queryFn: () => unwrap(api.GET('/orgs/{orgId}/roles', { params: { path: { orgId: org.id } } })),
  });
  if (catalogue.isPending) return <Skeleton className="h-80 w-full" />;
  if (catalogue.isError) return <FormNotice>{errorMessage(catalogue.error)}</FormNotice>;
  const { roles, permissions } = catalogue.data;
  const holds = (role: Role, id: string) =>
    roles.find((r) => r.role === role)?.permissions.includes(id) ?? false;
  return (
    <>
      <ul
        className="mb-6 grid grid-cols-4 gap-3 max-tablet:grid-cols-2 max-phone:grid-cols-1"
        aria-label="Roles"
      >
        {roles.map((r) => (
          <li key={r.role} className="rounded-md border border-border bg-card p-4">
            <p className="text-14 font-heading">{roleNames[r.role]}</p>
            <p className="mt-1 text-12 text-fg-muted">{roleDescriptions[r.role]}</p>
          </li>
        ))}
      </ul>
      <div className="overflow-x-auto rounded-md border border-border bg-card">
        <table className="w-full min-w-150 text-13">
          <caption className="sr-only">Permissions held by each role in this organisation</caption>
          <thead>
            <tr className="border-b border-border text-left text-12 text-fg-muted">
              <th scope="col" className="px-5 py-3 font-strong">
                Permission
              </th>
              {roles.map((r) => (
                <th key={r.role} scope="col" className="w-24 px-3 py-3 text-center font-strong">
                  {roleNames[r.role]}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {permissions.map((p) => (
              <tr key={p.id} className="border-b border-border last:border-b-0">
                <th scope="row" className="px-5 py-3 text-left font-book">
                  <span className="block">{p.description}</span>
                  {p.depends_on ? (
                    <span className="block text-12 text-fg-muted">
                      {settingNotes[p.depends_on] ?? p.depends_on}
                    </span>
                  ) : null}
                </th>
                {roles.map((r) => (
                  <td key={r.role} className="px-3 py-3 text-center">
                    {holds(r.role, p.id) ? (
                      <Check aria-label="Allowed" className="mx-auto size-4 text-success" />
                    ) : (
                      <Minus aria-label="Not allowed" className="mx-auto size-4 text-icon-muted" />
                    )}
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <p className="mt-4 text-12 text-fg-muted">
        Roles are fixed. Change someone’s role under Members; ownership moves only by transfer.
      </p>
    </>
  );
}
