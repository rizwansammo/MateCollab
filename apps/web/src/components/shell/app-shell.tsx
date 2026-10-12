'use client';

import {
  AppFrame,
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
import {
  Bell,
  CalendarDays,
  CircleHelp,
  Keyboard,
  Layers,
  LifeBuoy,
  MessageSquare,
  PanelLeftClose,
  PanelLeftOpen,
  Palette,
  Plus,
  Search,
  Settings,
  UserRound,
} from 'lucide-react';
import type { Route } from 'next';
import Link from 'next/link';
import { usePathname, useRouter, useSearchParams } from 'next/navigation';
import { useCallback, useState, type ReactNode } from 'react';
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

/** Organisation slot until organisations and sign-in exist. */
const ORGANISATION_PLACEHOLDER = 'Your organisation';

export function AppShell({
  initialSidebarWidth,
  children,
}: {
  initialSidebarWidth: number;
  children: ReactNode;
}) {
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const router = useRouter();
  const current = moduleFor(pathname);
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
        rail={<PrimaryRail current={current} helpOpen={panel === 'help'} onToggleHelp={toggleHelp} />}
        header={
          <GlobalBar
            start={
              <>
                <NavigationToggle />
                <Breadcrumb
                  items={[
                    { label: ORGANISATION_PLACEHOLDER },
                    { label: moduleLabels[current], current: true },
                  ]}
                />
              </>
            }
            center={<SearchTrigger onActivate={openSearch} />}
            end={
              <>
                <QuickCreateMenu />
                <Tooltip content="Activity">
                  <Link
                    href="/activity"
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
        sidebar={<ContextSidebar current={current} pathname={pathname} search={viewParams} />}
        panel={panel === 'help' ? <HelpPanel onClose={() => setPanel(null)} /> : null}
      >
        {children}
      </AppFrame>
      <SearchDialog open={searchOpen} onOpenChange={setSearchOpen} />
    </>
  );
}

function PrimaryRail({
  current,
  helpOpen,
  onToggleHelp,
}: {
  current: ModuleId;
  helpOpen: boolean;
  onToggleHelp: () => void;
}) {
  return (
    <Rail
      brand={
        <Link
          href="/home"
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
              href="/settings/appearance"
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
          <AccountMenu onShowShortcuts={onToggleHelp} />
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
          <Link href={item.href as Route} />
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

function AccountMenu({ onShowShortcuts }: { onShowShortcuts: () => void }) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        aria-label="Account"
        className="rounded-full outline-offset-2 transition-opacity duration-140 hover:opacity-90"
      >
        <span className="inline-flex size-8.75 items-center justify-center rounded-full bg-white/10 text-rail-fg">
          <UserRound className="size-5" aria-hidden />
        </span>
      </DropdownMenuTrigger>
      <DropdownMenuContent side="right" align="end">
        <DropdownMenuLabel>Not signed in</DropdownMenuLabel>
        <DropdownMenuItem asChild>
          <Link href="/settings/appearance">
            <Palette aria-hidden /> Appearance
          </Link>
        </DropdownMenuItem>
        <DropdownMenuItem onSelect={onShowShortcuts}>
          <Keyboard aria-hidden /> Keyboard shortcuts
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
        <DropdownMenuLabel>Available after sign-in</DropdownMenuLabel>
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
  current,
  pathname,
  search,
}: {
  current: ModuleId;
  pathname: string;
  search: URLSearchParams;
}) {
  const section = sidebarSections[current];
  return (
    <Sidebar
      header={
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <OrgSwitcherButton name={ORGANISATION_PLACEHOLDER} subtitle="Not signed in" />
          </DropdownMenuTrigger>
          <DropdownMenuContent className="w-60">
            <DropdownMenuLabel>No organisation yet</DropdownMenuLabel>
            <DropdownMenuItem asChild>
              <Link href="/settings/appearance">
                <Settings aria-hidden /> Personal settings
              </Link>
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      }
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
            <Link href={link.href as Route} />
          </SidebarItem>
        ))}
      </nav>
      {section.note ? (
        <p className="mx-2.5 mt-6 text-12 leading-[1.6] text-fg-muted">{section.note}</p>
      ) : null}
    </Sidebar>
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
