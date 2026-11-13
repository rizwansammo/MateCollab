import {
  AtSign,
  Bell,
  Bookmark,
  Building2,
  CalendarClock,
  CalendarDays,
  CircleCheck,
  Clock,
  Files,
  FolderOpen,
  History,
  House,
  KeyRound,
  Layers,
  LayoutGrid,
  LifeBuoy,
  LockKeyhole,
  MessageSquare,
  MessagesSquare,
  Palette,
  Reply,
  ScrollText,
  Share2,
  ShieldCheck,
  UserRound,
  UserRoundPlus,
  UsersRound,
  Video,
  type LucideIcon,
} from 'lucide-react';

// Paths here are relative to the organisation: the shell prefixes them with /{org}.

export type ModuleId = 'home' | 'chats' | 'spaces' | 'meetings' | 'files' | 'activity' | 'settings' | 'admin';

export interface RailModule {
  id: Exclude<ModuleId, 'settings' | 'admin'>;
  label: string;
  href: string;
  icon: LucideIcon;
}

/** The six primary modules, in rail order. */
export const railModules: RailModule[] = [
  { id: 'home', label: 'Home', href: '/home', icon: House },
  { id: 'chats', label: 'Chats', href: '/chats', icon: MessageSquare },
  { id: 'spaces', label: 'Spaces', href: '/spaces', icon: Layers },
  { id: 'meetings', label: 'Meetings', href: '/meetings', icon: Video },
  { id: 'files', label: 'Files', href: '/files', icon: Files },
  { id: 'activity', label: 'Activity', href: '/activity', icon: Bell },
];

export const moduleLabels: Record<ModuleId, string> = {
  home: 'Home',
  chats: 'Chats',
  spaces: 'Spaces',
  meetings: 'Meetings',
  files: 'Files',
  activity: 'Activity',
  settings: 'Settings',
  admin: 'Administration',
};

const homePaths = ['/home', '/actions', '/saved', '/support'];

/** Which module owns an organisation-relative path (Home also owns its sub-pages). */
export function moduleFor(pathname: string): ModuleId {
  const first = `/${pathname.split('/')[1] ?? ''}`;
  if (homePaths.includes(first)) return 'home';
  if (first === '/settings') return 'settings';
  if (first === '/admin') return 'admin';
  const match = railModules.find((m) => m.href === first);
  return match?.id ?? 'home';
}

export interface SidebarLink {
  label: string;
  href: string;
  icon: LucideIcon;
  /** Active when the path matches and, if given, the query parameter has this value. */
  path: string;
  param?: { name: string; value: string; isDefault?: boolean };
}

export interface SidebarSection {
  title: string;
  links: SidebarLink[];
  note?: string;
}

const view = (path: string, name: string, value: string, isDefault = false) => ({
  path,
  param: { name, value, isDefault },
  href: isDefault ? path : `${path}?${name}=${value}`,
});

export const sidebarSections: Record<ModuleId, SidebarSection> = {
  home: {
    title: 'Your workspace',
    links: [
      { label: 'Overview', href: '/home', path: '/home', icon: House },
      { label: 'My action items', href: '/actions', path: '/actions', icon: CircleCheck },
      { label: 'Saved for later', href: '/saved', path: '/saved', icon: Bookmark },
      { label: 'My support requests', href: '/support', path: '/support', icon: LifeBuoy },
    ],
  },
  chats: {
    title: 'Chats',
    links: [{ label: 'All conversations', href: '/chats', path: '/chats', icon: MessagesSquare }],
    note: 'Direct messages and group conversations will appear here.',
  },
  spaces: {
    title: 'Spaces',
    links: [{ label: 'Browse Spaces', href: '/spaces', path: '/spaces', icon: LayoutGrid }],
  },
  meetings: {
    title: 'Meetings',
    links: [
      { label: 'Upcoming', icon: CalendarClock, ...view('/meetings', 'view', 'upcoming', true) },
      { label: 'Past', icon: History, ...view('/meetings', 'view', 'past') },
    ],
  },
  files: {
    title: 'Files',
    links: [
      { label: 'Recent', icon: Clock, ...view('/files', 'view', 'recent', true) },
      { label: 'Shared with me', icon: Share2, ...view('/files', 'view', 'shared') },
      { label: 'Space files', icon: FolderOpen, ...view('/files', 'view', 'spaces') },
    ],
  },
  activity: {
    title: 'Activity',
    links: [
      { label: 'All activity', icon: Bell, ...view('/activity', 'type', 'all', true) },
      { label: 'Mentions', icon: AtSign, ...view('/activity', 'type', 'mentions') },
      { label: 'Replies', icon: Reply, ...view('/activity', 'type', 'replies') },
      { label: 'Meetings', icon: CalendarDays, ...view('/activity', 'type', 'meetings') },
      { label: 'Support', icon: LifeBuoy, ...view('/activity', 'type', 'support') },
    ],
  },
  settings: {
    title: 'Personal settings',
    links: [
      { label: 'Profile', href: '/settings/profile', path: '/settings/profile', icon: UserRound },
      {
        label: 'Sign-in and security',
        href: '/settings/security',
        path: '/settings/security',
        icon: ShieldCheck,
      },
      { label: 'Appearance', href: '/settings/appearance', path: '/settings/appearance', icon: Palette },
      { label: 'Privacy', href: '/settings/privacy', path: '/settings/privacy', icon: LockKeyhole },
    ],
  },
  admin: {
    title: 'Administration',
    links: [
      { label: 'General', href: '/admin/general', path: '/admin/general', icon: Building2 },
      { label: 'Spaces', href: '/admin/spaces', path: '/admin/spaces', icon: Layers },
      { label: 'Messaging policy', href: '/admin/messaging', path: '/admin/messaging', icon: MessageSquare },
      { label: 'Members', href: '/admin/members', path: '/admin/members', icon: UsersRound },
      { label: 'Invitations', href: '/admin/invitations', path: '/admin/invitations', icon: UserRoundPlus },
      { label: 'Roles and permissions', href: '/admin/roles', path: '/admin/roles', icon: KeyRound },
      { label: 'Security policy', href: '/admin/security', path: '/admin/security', icon: ShieldCheck },
      { label: 'Audit log', href: '/admin/audit', path: '/admin/audit', icon: ScrollText },
    ],
  },
};

export function isLinkActive(link: SidebarLink, pathname: string, search: URLSearchParams): boolean {
  if (pathname !== link.path) return false;
  if (!link.param) return true;
  const current = search.get(link.param.name);
  return current === link.param.value || (current === null && Boolean(link.param.isDefault));
}
