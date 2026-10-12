'use client';

import type { OrganisationSummary } from '@matecollab/contracts';
import { createContext, useContext } from 'react';

const OrgContext = createContext<OrganisationSummary | null>(null);
export const OrgProvider = OrgContext.Provider;

/** The organisation of the current /{org}/… route (resolved from the session's memberships). */
export function useOrg(): OrganisationSummary {
  const org = useContext(OrgContext);
  if (!org) throw new Error('useOrg must be used inside an organisation route');
  return org;
}

/** Builds a path inside the current organisation. */
export function orgPath(org: Pick<OrganisationSummary, 'slug'>, path: string): string {
  return `/${org.slug}${path.startsWith('/') ? path : `/${path}`}`;
}

/** Query keys of tenant data are scoped by organisation (FRONTEND.md §8). */
export function orgKey(org: Pick<OrganisationSummary, 'id'>, ...parts: unknown[]) {
  return ['org', org.id, ...parts] as const;
}
