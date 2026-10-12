'use client';

import { ChevronDown } from 'lucide-react';
import { Slot } from 'radix-ui';
import type { ComponentProps, ReactNode } from 'react';
import { NavCount } from '../components/badge';
import { cn } from '../lib/cn';

export interface SidebarProps {
  /** Organisation switcher. */
  header?: ReactNode;
  children: ReactNode;
  /** Connection status and the small lockup. */
  footer?: ReactNode;
  className?: string;
}

/** Context sidebar layout: header, scrolling body, footer. */
export function Sidebar({ header, children, footer, className }: SidebarProps) {
  return (
    <div className={cn('flex h-full flex-col bg-sidebar text-13', className)}>
      {header ? <div className="shrink-0 border-b border-border px-4 pt-5.25 pb-4.5">{header}</div> : null}
      <div className="min-h-0 flex-1 overflow-y-auto px-3 py-4.25">{children}</div>
      {footer ? <div className="shrink-0 border-t border-border px-5 py-3.75">{footer}</div> : null}
    </div>
  );
}

/** Module title at the top of the sidebar body (e.g. "Chats"), with an optional action. */
export function SidebarTitle({ children, action }: { children: ReactNode; action?: ReactNode }) {
  return (
    <div className="flex items-center justify-between gap-2 px-2 pb-3.5">
      <h2 className="text-16 font-heading tracking-[-0.2px]">{children}</h2>
      {action}
    </div>
  );
}

/** Small uppercase group label ("FAVORITE SPACES"). */
export function SidebarSectionLabel({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <h3
      className={cn(
        'mx-2.5 mt-7 mb-2.25 text-11 font-heading tracking-label text-fg-muted uppercase first:mt-0',
        className,
      )}
    >
      {children}
    </h3>
  );
}

export interface SidebarItemProps extends Omit<ComponentProps<'a'>, 'children'> {
  icon?: ReactNode;
  label: string;
  /** Unread / open count; zero hides it. */
  count?: number;
  countLabel?: string;
  active?: boolean;
  /** Render the single child element (e.g. a Next.js <Link>); icon and label go inside it. */
  asChild?: boolean;
  children?: ReactNode;
}

/** 36 px navigation row in the context sidebar. */
export function SidebarItem({
  icon,
  label,
  count,
  countLabel = 'unread',
  active = false,
  asChild = false,
  className,
  children,
  ...props
}: SidebarItemProps) {
  const Comp = asChild ? Slot.Root : 'a';
  return (
    <Comp
      aria-current={active ? 'page' : undefined}
      className={cn(
        'group flex h-9 w-full items-center gap-2.5 rounded-md px-2.5 text-13 font-book text-fg-nav',
        'transition-colors duration-140 hover:bg-muted',
        '[&_svg]:size-4.25 [&_svg]:shrink-0 [&_svg]:text-icon-muted',
        'aria-[current=page]:bg-accent aria-[current=page]:font-semibold aria-[current=page]:text-primary-ink',
        'aria-[current=page]:[&_svg]:text-primary',
        className,
      )}
      {...props}
    >
      {asChild ? <Slot.Slottable>{children}</Slot.Slottable> : null}
      {icon}
      <span className="min-w-0 flex-1 truncate">{label}</span>
      {count ? (
        <>
          <NavCount active={active} aria-hidden>
            {count}
          </NavCount>
          <span className="sr-only">
            , {count} {countLabel}
          </span>
        </>
      ) : null}
    </Comp>
  );
}

/** Organisation initial tile (organisations never use the MateCollab mark). */
export function OrgBadge({ name, className }: { name: string; className?: string }) {
  return (
    <span
      aria-hidden
      className={cn(
        'flex size-8.5 shrink-0 items-center justify-center rounded-md border border-org-line bg-org text-18 font-heavy text-org-fg',
        className,
      )}
    >
      {Array.from(name.trim())[0]?.toLocaleUpperCase() ?? '?'}
    </span>
  );
}

export interface OrgSwitcherButtonProps extends ComponentProps<'button'> {
  name: string;
  subtitle?: string;
}

/** Sidebar header button showing the current organisation; wrap in a DropdownMenuTrigger. */
export function OrgSwitcherButton({ name, subtitle, className, ...props }: OrgSwitcherButtonProps) {
  return (
    <button
      type="button"
      className={cn(
        'flex w-full items-center gap-2.5 rounded-md text-left text-fg outline-offset-2',
        className,
      )}
      {...props}
    >
      <OrgBadge name={name} />
      <span className="min-w-0 flex-1">
        <strong className="block truncate text-13 font-heading">{name}</strong>
        {subtitle ? <small className="mt-px block text-11 text-fg-muted">{subtitle}</small> : null}
      </span>
      <ChevronDown className="size-4 shrink-0 text-fg-muted" aria-hidden />
    </button>
  );
}
