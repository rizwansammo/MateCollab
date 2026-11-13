'use client';

import {
  AppFrame,
  Avatar,
  Breadcrumb,
  BrandLockup,
  BrandMark,
  ContextPanel,
  Dialog,
  DialogContent,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
  EmptyState,
  GlobalBar,
  IconButton,
  Kbd,
  OrgBadge,
  OrgSwitcherButton,
  Rail,
  RailItem,
  SearchTrigger,
  SectionTitle,
  Sidebar,
  SidebarItem,
  SidebarTitle,
  Tooltip,
  cn,
  iconButtonVariants,
  useModifierKeyLabel,
  useShell,
} from '@matecollab/ui';
import type { OrganisationSummary } from '@matecollab/contracts';
import {
  ArrowLeftRight,
  Bell,
  CalendarDays,
  Check,
  CircleHelp,
  Keyboard,
  Layers,
  LifeBuoy,
  LogOut,
  MessageSquare,
  PanelLeftClose,
  PanelLeftOpen,
  Palette,
  Plus,
  Search,
  Settings,
  ShieldCheck,
  UserRound,
  UsersRound,
} from 'lucide-react';
import type { Route } from 'next';
import Link from 'next/link';
import { usePathname, useRouter, useSearchParams } from 'next/navigation';
import { useCallback, useState, type ReactNode } from 'react';
import { useSession, useSignOut } from '@/features/auth/session';
import { orgPath } from '@/features/org/org-context';
import { SpacesSidebarTree } from '@/features/spaces/sidebar-tree';
import { isAdministrator, roleNames } from '@/features/org/roles';
import { useRuntimeConfig } from '@/lib/api';
import { SIDEBAR_WIDTH_COOKIE, writePreferenceCookie } from '@/lib/preferences';
import { ConnectionStatus } from './connection-status';
import {
  isLinkActive,
  moduleFor,
  moduleLabels,
  railModules,
  sidebarSections,
  type ModuleId,
} from './navigation';

export function AppShell({
  org,
  initialSidebarWidth,
  children,
}: {
  org: OrganisationSummary;
  initialSidebarWidth: number;
  children: ReactNode;
}) {
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const router = useRouter();
  // Navigation is defined relative to the organisation; the URL is /{org}/...
  const subpath = pathname.slice(`/${org.slug}`.length) || '/';
  const current = moduleFor(subpath);
  const [sidebarWidth, setSidebarWidth] = useState(initialSidebarWidth);
  const [searchOpen, setSearchOpen] = useState(false);
  const openSearch = useCallback(() => setSearchOpen(true), []);

  // Panels live in the URL (?panel=help) so they survive reloads and can be linked.
  const panel = searchParams.get('panel');
  const setPanel = useCallback(
    (value: string | null) => {
      const params = new URLSearchParams(searchParams.toString());
      if (value) params.set('panel', value);
      else params.delete('panel');
      const query = params.toString();
      router.replace((query ? `${pathname}?${query}` : pathname) as Route, { scroll: false });
    },
    [pathname, router, searchParams],
  );
  const toggleHelp = useCallback(() => setPanel(panel === 'help' ? null : 'help'), [panel, setPanel]);

  const viewParams = new URLSearchParams(searchParams.toString());
  viewParams.delete('panel');

  return (
    <>
      <AppFrame
        locationKey={`${pathname}?${viewParams.toString()}`}
        sidebarLabel={`${sidebarSections[current].title} navigation`}
        sidebarWidth={sidebarWidth}
        onSidebarWidthChange={setSidebarWidth}
        onSidebarWidthCommit={(width) => writePreferenceCookie(SIDEBAR_WIDTH_COOKIE, String(width))}
        rail={
          <PrimaryRail org={org} current={current} helpOpen={panel === 'help'} onToggleHelp={toggleHelp} />
        }
        header={
          <GlobalBar
            start={
              <>
                <NavigationToggle />
                <Breadcrumb items={[{ label: org.name }, { label: moduleLabels[current], current: true }]} />
              </>
            }
            center={<SearchTrigger onActivate={openSearch} />}
            end={
              <>
                <QuickCreateMenu />
                <Tooltip content="Activity">
                  <Link
                    href={orgPath(org, '/activity') as Route}
                    aria-label="Activity"
                    className={cn(iconButtonVariants(), 'max-phone:hidden')}
                  >
                    <Bell aria-hidden />
                  </Link>
                </Tooltip>
              </>
            }
          />
        }
        sidebar={<ContextSidebar org={org} current={current} pathname={subpath} search={viewParams} />}
        panel={panel === 'help' ? <HelpPanel onClose={() => setPanel(null)} /> : null}
      >
        {children}
      </AppFrame>
      <SearchDialog open={searchOpen} onOpenChange={setSearchOpen} />
    </>
  );
}

function PrimaryRail({
  org,
  current,
  helpOpen,
  onToggleHelp,
}: {
  org: OrganisationSummary;
  current: ModuleId;
  helpOpen: boolean;
  onToggleHelp: () => void;
}) {
  return (
    <Rail
      brand={
        <Link
          href={orgPath(org, '/home') as Route}
          aria-label="MateCollab home"
          className="inline-flex rounded-md p-1.5 transition-colors duration-140 hover:bg-rail-hover"
        >
          <BrandMark variant="white" size={28} alt="" />
        </Link>
      }
      footer={
        <>
          <IconButton
            label="Help and keyboard shortcuts"
            tone="rail"
            size="lg"
            tooltipSide="right"
            pressed={helpOpen}
            onClick={onToggleHelp}
            className="aria-pressed:bg-rail-hover aria-pressed:text-rail-fg-active"
          >
            <CircleHelp aria-hidden />
          </IconButton>
          <Tooltip content="Settings" side="right">
            <Link
              href={orgPath(org, '/settings/profile') as Route}
              aria-label="Settings"
              aria-current={current === 'settings' ? 'page' : undefined}
              className={cn(
                iconButtonVariants({ tone: 'rail', size: 'lg' }),
                'aria-[current=page]:bg-rail-hover aria-[current=page]:text-rail-fg-active',
              )}
            >
              <Settings aria-hidden />
            </Link>
          </Tooltip>
          <AccountMenu org={org} onShowShortcuts={onToggleHelp} />
        </>
      }
    >
      {railModules.map((item) => (
        <RailItem
          key={item.id}
          asChild
          icon={<item.icon aria-hidden />}
          label={item.label}
          active={current === item.id}
        >
          <Link href={orgPath(org, item.href) as Route} />
        </RailItem>
      ))}
    </Rail>
  );
}

function NavigationToggle() {
  const { toggleNavigation, sidebarCollapsed } = useShell();
  return (
    <IconButton label={sidebarCollapsed ? 'Expand sidebar' : 'Collapse sidebar'} onClick={toggleNavigation}>
      {sidebarCollapsed ? <PanelLeftOpen aria-hidden /> : <PanelLeftClose aria-hidden />}
    </IconButton>
  );
}

function AccountMenu({ org, onShowShortcuts }: { org: OrganisationSummary; onShowShortcuts: () => void }) {
  const { user } = useSession();
  const signOut = useSignOut();
  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        aria-label={`Account: ${user.display_name}`}
        className="rounded-full outline-offset-2 transition-opacity duration-140 hover:opacity-90"
      >
        <Avatar name={user.display_name} size={35} decorative />
      </DropdownMenuTrigger>
      <DropdownMenuContent side="right" align="end" className="w-64">
        <div className="px-2.5 py-2">
          <span className="block truncate text-13 font-heading text-fg">{user.display_name}</span>
          <span className="block truncate text-12 text-fg-muted">{user.primary_email}</span>
        </div>
        <DropdownMenuSeparator />
        <DropdownMenuItem asChild>
          <Link href={orgPath(org, '/settings/profile') as Route}>
            <UserRound aria-hidden /> Profile
          </Link>
        </DropdownMenuItem>
        <DropdownMenuItem asChild>
          <Link href={orgPath(org, '/settings/security') as Route}>
            <ShieldCheck aria-hidden /> Sign-in and security
          </Link>
        </DropdownMenuItem>
        <DropdownMenuItem asChild>
          <Link href={orgPath(org, '/settings/appearance') as Route}>
            <Palette aria-hidden /> Appearance
          </Link>
        </DropdownMenuItem>
        <DropdownMenuItem onSelect={onShowShortcuts}>
          <Keyboard aria-hidden /> Keyboard shortcuts
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem onSelect={() => signOut.mutate()} disabled={signOut.isPending}>
          <LogOut aria-hidden /> Sign out
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

function QuickCreateMenu() {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <IconButton label="Quick create">
          <Plus aria-hidden />
        </IconButton>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <DropdownMenuLabel>Coming soon</DropdownMenuLabel>
        <DropdownMenuItem disabled>
          <MessageSquare aria-hidden /> New message
        </DropdownMenuItem>
        <DropdownMenuItem disabled>
          <CalendarDays aria-hidden /> Schedule meeting
        </DropdownMenuItem>
        <DropdownMenuItem disabled>
          <Layers aria-hidden /> Create Space
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem disabled>
          <LifeBuoy aria-hidden /> Support request
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

function ContextSidebar({
  org,
  current,
  pathname,
  search,
}: {
  org: OrganisationSummary;
  current: ModuleId;
  pathname: string;
  search: URLSearchParams;
}) {
  const section = sidebarSections[current];
  return (
    <Sidebar
      header={<OrganisationSwitcher org={org} />}
      footer={
        <>
          <ConnectionStatus />
          <BrandLockup height={20} className="mt-4" />
        </>
      }
    >
      <SidebarTitle>{section.title}</SidebarTitle>
      <nav aria-label={`${section.title} sections`} className="flex flex-col">
        {section.links.map((link) => (
          <SidebarItem
            key={link.href}
            asChild
            icon={<link.icon aria-hidden />}
            label={link.label}
            active={isLinkActive(link, pathname, search)}
          >
            <Link href={orgPath(org, link.href) as Route} />
          </SidebarItem>
        ))}
      </nav>
      {current === 'spaces' ? <SpacesSidebarTree /> : null}
      {section.note ? (
        <p className="mx-2.5 mt-6 text-12 leading-[1.6] text-fg-muted">{section.note}</p>
      ) : null}
    </Sidebar>
  );
}

/** Sidebar header: the current organisation, switching between memberships, administration. */
function OrganisationSwitcher({ org }: { org: OrganisationSummary }) {
  const { organisations } = useSession();
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <OrgSwitcherButton
          name={org.name}
          subtitle={roleNames[org.role]}
          aria-label={`Organisation: ${org.name}`}
        />
      </DropdownMenuTrigger>
      <DropdownMenuContent className="w-64">
        <DropdownMenuLabel>Organisations</DropdownMenuLabel>
        {organisations.map((o) => (
          <DropdownMenuItem key={o.id} asChild>
            <Link href={orgPath(o, '/home') as Route} aria-current={o.id === org.id ? 'true' : undefined}>
              <OrgBadge name={o.name} className="size-5.5 text-11" />
              <span className="min-w-0 flex-1 truncate">{o.name}</span>
              {o.id === org.id ? <Check aria-hidden className="text-primary-ink" /> : null}
            </Link>
          </DropdownMenuItem>
        ))}
        <DropdownMenuItem asChild>
          <Link href="/select-organisation">
            <ArrowLeftRight aria-hidden /> All organisations
          </Link>
        </DropdownMenuItem>
        {isAdministrator(org.role) ? (
          <>
            <DropdownMenuSeparator />
            <DropdownMenuItem asChild>
              <Link href={orgPath(org, '/admin/members') as Route}>
                <UsersRound aria-hidden /> Members and invitations
              </Link>
            </DropdownMenuItem>
            <DropdownMenuItem asChild>
              <Link href={orgPath(org, '/admin/general') as Route}>
                <Settings aria-hidden /> Organisation settings
              </Link>
            </DropdownMenuItem>
          </>
        ) : null}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

function HelpPanel({ onClose }: { onClose: () => void }) {
  const mod = useModifierKeyLabel();
  const runtime = useRuntimeConfig();
  const shortcuts: [string, ReactNode][] = [
    ['Search', <Kbd key="k">{mod} K</Kbd>],
    ['Close a panel or dialog', <Kbd key="esc">Esc</Kbd>],
    [
      'Resize the sidebar (focus its edge)',
      <span key="arrows" className="flex gap-1">
        <Kbd>←</Kbd>
        <Kbd>→</Kbd>
      </span>,
    ],
  ];
  return (
    <ContextPanel title="Help" description="Keyboard shortcuts and build information" onClose={onClose}>
      <div className="px-6 pt-6">
        <SectionTitle title="Keyboard shortcuts" />
        <dl className="flex flex-col">
          {shortcuts.map(([label, keys]) => (
            <div
              key={label}
              className="flex items-center justify-between gap-4 border-b border-border py-3 text-13"
            >
              <dt>{label}</dt>
              <dd>{keys}</dd>
            </div>
          ))}
        </dl>
        <SectionTitle title="About this build" />
        {runtime.status === 'ready' ? (
          <dl className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-2 text-13">
            <dt className="text-fg-muted">Version</dt>
            <dd className="font-mono text-12">{runtime.config.version}</dd>
            <dt className="text-fg-muted">Environment</dt>
            <dd>{runtime.config.environment}</dd>
            <dt className="text-fg-muted">API</dt>
            <dd>{runtime.config.api_version}</dd>
            <dt className="text-fg-muted">Media server</dt>
            <dd>{runtime.config.media.livekit_url ? 'Configured' : 'Not configured'}</dd>
          </dl>
        ) : (
          <p className="text-13 text-fg-muted">
            {runtime.status === 'loading' ? 'Loading…' : 'Build information is unavailable right now.'}
          </p>
        )}
      </div>
    </ContextPanel>
  );
}

function SearchDialog({ open, onOpenChange }: { open: boolean; onOpenChange: (open: boolean) => void }) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent title="Search" description="Find messages, files, people and Spaces.">
        <EmptyState
          icon={<Search />}
          title="Nothing to search yet"
          description="Search becomes available once your organisation has conversations and files."
          headingLevel={3}
          className="py-8"
        />
      </DialogContent>
    </Dialog>
  );
}
