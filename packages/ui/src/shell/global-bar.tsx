import { ChevronRight } from 'lucide-react';
import { Fragment, type ReactNode } from 'react';
import { cn } from '../lib/cn';

export interface GlobalBarProps {
  /** Navigation toggle and breadcrumb. */
  start: ReactNode;
  /** Search trigger. */
  center?: ReactNode;
  /** Quick create, notifications. */
  end?: ReactNode;
}

/** 56 px bar above the workspace (53 px on phones). */
export function GlobalBar({ start, center, end }: GlobalBarProps) {
  return (
    <header
      className={cn(
        'flex h-14 shrink-0 items-center justify-between gap-4.5 border-b border-border bg-bg px-5.75',
        'max-compact:px-4.25 max-tablet:px-4.75 max-phone:h-13.25 max-phone:gap-2.5 max-phone:px-3',
      )}
    >
      <div className="flex min-w-0 shrink-0 items-center gap-2.75 max-phone:gap-1.75">{start}</div>
      {center}
      <div className="flex shrink-0 items-center gap-3 max-phone:gap-1">{end}</div>
    </header>
  );
}

export interface BreadcrumbProps {
  /** Context first (organisation), current page last. */
  items: { label: string; current?: boolean }[];
}

/**
 * Organisation › module. Compact desktops hide the organisation; phones show only the
 * navigation toggle beside it.
 */
export function Breadcrumb({ items }: BreadcrumbProps) {
  return (
    <nav aria-label="Breadcrumb" className="min-w-0 max-phone:hidden">
      <ol className="flex items-center gap-2.75 text-12 whitespace-nowrap text-fg-muted">
        {items.map((item, index) => (
          <Fragment key={`${index}-${item.label}`}>
            {index > 0 ? (
              <li aria-hidden className="inline-flex">
                <ChevronRight className="size-3.25" />
              </li>
            ) : null}
            <li
              aria-current={item.current ? 'page' : undefined}
              className={cn('truncate', item.current ? 'font-strong text-fg' : 'max-compact:hidden')}
            >
              {item.label}
            </li>
          </Fragment>
        ))}
      </ol>
    </nav>
  );
}
