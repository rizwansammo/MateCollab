import type { MembershipStatus, Role } from '@matecollab/contracts';

// Labels and presentation hints only. Every permission is decided by the API (internal/authz);
// the UI hides actions a role can never take, and still handles 403 for everything else.

export const roleNames: Record<Role, string> = {
  owner: 'Owner',
  admin: 'Admin',
  member: 'Member',
  guest: 'Guest',
};

export const roleDescriptions: Record<Role, string> = {
  owner: 'Full control, including billing, deletion and Admins.',
  admin: 'Manages members, invitations and organisation settings.',
  member: 'Takes part in conversations, Spaces and meetings.',
  guest: 'Limited access to the Spaces they are added to.',
};

export function roleLabel(role: Role, { article = false }: { article?: boolean } = {}): string {
  const name = roleNames[role] ?? role;
  if (!article) return name;
  return /^[AEIOU]/.test(name) ? `an ${name}` : `a ${name}`;
}

export const statusNames: Record<MembershipStatus, string> = {
  active: 'Active',
  suspended: 'Suspended',
  deactivated: 'Deactivated',
};

/** Owners and Admins see the administration area. */
export function isAdministrator(role: Role): boolean {
  return role === 'owner' || role === 'admin';
}
