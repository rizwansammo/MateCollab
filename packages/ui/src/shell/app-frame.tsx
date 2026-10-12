'use client';

import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react';
import { Drawer } from '../components/drawer';
import { ResizeHandle } from '../components/resize-handle';
import { cn } from '../lib/cn';

export const SIDEBAR_DEFAULT_WIDTH = 274;
export const SIDEBAR_MIN_WIDTH = 240;
export const SIDEBAR_MAX_WIDTH = 320;

/** Viewports at or below this width show the context sidebar as a drawer. */
const DRAWER_QUERY = '(max-width: 1100px)';

interface ShellContextValue {
  /** Desktop: collapse/expand the sidebar column. Tablet and phone: open/close the drawer. */
  toggleNavigation: () => void;
  closeDrawer: () => void;
  sidebarCollapsed: boolean;
  drawerOpen: boolean;
}

const ShellContext = createContext<ShellContextValue | null>(null);

export function useShell(): ShellContextValue {
  const value = useContext(ShellContext);
  if (!value) throw new Error('useShell must be used inside <AppFrame>');
  return value;
}

export interface AppFrameProps {
  /** <Rail> — left column on desktop and tablet, bottom navigation on phones. */
  rail: ReactNode;
  /** <GlobalBar> above the workspace. */
  header: ReactNode;
  /** Context sidebar content (<Sidebar>); rendered in the column or in the drawer. */
  sidebar?: ReactNode;
  /** Accessible name of the sidebar region and drawer, e.g. "Chats navigation". */
  sidebarLabel: string;
  /** <ContextPanel> when a right panel is open. */
  panel?: ReactNode;
  sidebarWidth: number;
  onSidebarWidthChange: (width: number) => void;
  /** Persist the width here (called when a drag or key press settles). */
  onSidebarWidthCommit?: (width: number) => void;
  /** Changes whenever the route changes; closes the navigation drawer. */
  locationKey?: string;
  children: ReactNode;
}

/**
 * The MateCollab application frame (FRONTEND.md §4): 70 px rail, resizable 274 px context sidebar,
 * 56 px global bar, workspace, and a 380 px right panel. ≤ 1100 px the sidebar becomes a drawer;
 * ≤ 760 px the rail becomes a 62 px bottom navigation bar.
 */
export function AppFrame({
  rail,
  header,
  sidebar,
  sidebarLabel,
  panel,
  sidebarWidth,
  onSidebarWidthChange,
  onSidebarWidthCommit,
  locationKey,
  children,
}: AppFrameProps) {
  const [collapsed, setCollapsed] = useState(false);
  const [drawerOpen, setDrawerOpen] = useState(false);

  const toggleNavigation = useCallback(() => {
    if (window.matchMedia(DRAWER_QUERY).matches) setDrawerOpen((open) => !open);
    else setCollapsed((value) => !value);
  }, []);
  const closeDrawer = useCallback(() => setDrawerOpen(false), []);

  // Close the drawer on navigation (state adjusted during render, not in an effect) and when
  // the viewport grows past the drawer breakpoint.
  const [lastLocation, setLastLocation] = useState(locationKey);
  if (locationKey !== lastLocation) {
    setLastLocation(locationKey);
    setDrawerOpen(false);
  }
  useEffect(() => {
    const query = window.matchMedia(DRAWER_QUERY);
    const onChange = () => {
      if (!query.matches) setDrawerOpen(false);
    };
    query.addEventListener('change', onChange);
    return () => query.removeEventListener('change', onChange);
  }, []);

  const context = useMemo(
    () => ({ toggleNavigation, closeDrawer, sidebarCollapsed: collapsed, drawerOpen }),
    [toggleNavigation, closeDrawer, collapsed, drawerOpen],
  );
  const hasPanel = Boolean(panel);

  return (
    <ShellContext value={context}>
      <div
        className="flex h-dvh overflow-hidden bg-bg text-fg max-phone:pb-15.5"
        data-panel={hasPanel || undefined}
      >
        <a
          href="#main-content"
          className="sr-only z-90 rounded-md bg-bg px-3 py-2 text-13 font-strong text-primary-ink shadow-popover focus:not-sr-only focus:fixed focus:top-2 focus:left-2"
        >
          Skip to content
        </a>
        {rail}
        <div className="flex min-w-0 flex-1 flex-col overflow-hidden">
          {header}
          <div className="flex min-h-0 flex-1 overflow-hidden">
            {sidebar && !collapsed ? (
              <aside
                aria-label={sidebarLabel}
                className="relative shrink-0 border-r border-border bg-sidebar max-tablet:hidden"
                style={{ width: sidebarWidth }}
              >
                {sidebar}
                <ResizeHandle
                  label="Resize sidebar"
                  value={sidebarWidth}
                  min={SIDEBAR_MIN_WIDTH}
                  max={SIDEBAR_MAX_WIDTH}
                  onChange={onSidebarWidthChange}
                  onCommit={onSidebarWidthCommit}
                />
              </aside>
            ) : null}
            {/* Focusable so keyboard users can scroll it (and the skip link lands here). */}
            <main
              id="main-content"
              tabIndex={0}
              className={cn(
                'min-w-0 flex-1 overflow-auto bg-bg focus-visible:outline-2 focus-visible:-outline-offset-2',
                hasPanel && 'dock:mr-95',
              )}
            >
              {children}
            </main>
          </div>
        </div>
        {panel}
        {sidebar ? (
          <Drawer open={drawerOpen} onOpenChange={setDrawerOpen} title={sidebarLabel}>
            {sidebar}
          </Drawer>
        ) : null}
      </div>
    </ShellContext>
  );
}
